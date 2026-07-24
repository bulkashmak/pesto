// Package runner drives a Pesto test run: it transforms authored tests into
// standard Go tests via an overlay and invokes `go test`, without modifying any
// file in the project.
package runner

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bulkashmak/pesto/internal/report"
	"github.com/bulkashmak/pesto/internal/transform"
)

// Options configures a run.
type Options struct {
	Root       string    // project root (module directory)
	Env        string    // environment name; empty means none selected
	Args       []string  // extra arguments passed through to `go test`
	Stdout     io.Writer // where go test stdout goes
	Stderr     io.Writer // where go test stderr goes
	Keep       bool      // keep the temp overlay dir instead of deleting it
	Report     string    // report format: "" (none) or "junit"
	ReportPath string    // report output path (default: <root>/reports/junit.xml)
}

// Run transforms the project's Pesto tests and executes `go test`. It returns the
// `go test` exit code. A non-zero exit code (failing tests) is not an error;
// err is non-nil only for problems running the pipeline itself.
func Run(opts Options) (exitCode int, err error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return 1, err
	}

	overlay, tmpDir, err := buildOverlay(root)
	if err != nil {
		return 1, err
	}
	if opts.Keep {
		fmt.Fprintf(opts.Stderr, "pesto: overlay kept at %s\n", tmpDir)
	} else {
		defer os.RemoveAll(tmpDir)
	}

	overlayPath := filepath.Join(tmpDir, "overlay.json")
	if err := writeOverlay(overlayPath, overlay); err != nil {
		return 1, err
	}

	if opts.Report != "" && opts.Report != "junit" {
		return 1, fmt.Errorf("unknown report format %q (supported: junit)", opts.Report)
	}

	args := []string{"test"}
	if opts.Report != "" {
		args = append(args, "-json")
	}
	args = append(args, "-overlay", overlayPath)
	args = append(args, opts.Args...)
	if !hasPackageArg(opts.Args) {
		args = append(args, "./...")
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Stderr = opts.Stderr
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "PESTO_ROOT="+root)
	if opts.Env != "" {
		cmd.Env = append(cmd.Env, "PESTO_ENV="+opts.Env)
	}

	if opts.Report == "" {
		cmd.Stdout = opts.Stdout
		return waitCode(cmd.Run())
	}
	return runWithReport(cmd, root, opts)
}

// runWithReport streams `go test -json`, reproduces normal console output, and
// writes a JUnit XML report.
func runWithReport(cmd *exec.Cmd, root string, opts Options) (int, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, err
	}
	if err := cmd.Start(); err != nil {
		return 1, err
	}

	rep, parseErr := report.FromGoTestJSON(stdout, opts.Stdout)
	exitCode, waitErr := waitCode(cmd.Wait())
	if waitErr != nil {
		return exitCode, waitErr
	}
	if parseErr != nil {
		return exitCode, fmt.Errorf("parse test output: %w", parseErr)
	}

	path := opts.ReportPath
	if path == "" {
		path = filepath.Join(root, "reports", "junit.xml")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return exitCode, err
	}
	xmlBytes, err := rep.XML()
	if err != nil {
		return exitCode, err
	}
	if err := os.WriteFile(path, xmlBytes, 0o644); err != nil {
		return exitCode, err
	}
	fmt.Fprintf(opts.Stdout, "\npesto: wrote JUnit report to %s\n", path)
	return exitCode, nil
}

// waitCode converts a cmd error into a `go test` exit code. A non-zero exit
// (failing tests) is reported as a code, not an error.
func waitCode(err error) (int, error) {
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, fmt.Errorf("run go test: %w", err)
}

// overlayEntry maps a compiler-visible path to the temp file that replaces it.
type overlayEntry struct {
	visiblePath string // path go test sees
	content     []byte
}

// buildOverlay walks root, transforms every package containing Pesto tests, writes
// the rewritten files + generated wrappers to a temp dir, and returns the overlay
// mapping (visible path -> temp file).
func buildOverlay(root string) (map[string]string, string, error) {
	// Group discovered Pesto tests by their directory.
	type pkgInfo struct {
		name    string
		tests   []transform.TestFunc
		entries []overlayEntry
	}
	pkgs := map[string]*pkgInfo{}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(root, path, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") || d.Name() == transform.GeneratedFileName {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		res, err := transform.RewriteFile(path, src)
		if err != nil {
			return err
		}
		if len(res.Tests) == 0 {
			return nil
		}

		dir := filepath.Dir(path)
		pi := pkgs[dir]
		if pi == nil {
			pi = &pkgInfo{name: res.Package}
			pkgs[dir] = pi
		}
		pi.tests = append(pi.tests, res.Tests...)
		pi.entries = append(pi.entries, overlayEntry{visiblePath: path, content: res.Rewritten})
		return nil
	})
	if walkErr != nil {
		return nil, "", walkErr
	}
	if len(pkgs) == 0 {
		return nil, "", errors.New("no Pesto tests found (looked for func TestX(t *pesto.T))")
	}

	tmpDir, err := os.MkdirTemp("", "pesto-overlay-")
	if err != nil {
		return nil, "", err
	}

	replace := map[string]string{}
	n := 0
	writeTemp := func(content []byte) (string, error) {
		n++
		p := filepath.Join(tmpDir, fmt.Sprintf("f%d.go", n))
		return p, os.WriteFile(p, content, 0o644)
	}

	for dir, pi := range pkgs {
		for _, e := range pi.entries {
			tmp, err := writeTemp(e.content)
			if err != nil {
				return nil, tmpDir, err
			}
			replace[e.visiblePath] = tmp
		}

		wrappers, err := transform.WrappersFile(pi.name, pi.tests)
		if err != nil {
			return nil, tmpDir, err
		}
		wrapperTmp, err := writeTemp(wrappers)
		if err != nil {
			return nil, tmpDir, err
		}
		wrapperVisible := filepath.Join(dir, transform.GeneratedFileName)
		if _, statErr := os.Stat(wrapperVisible); statErr == nil {
			return nil, tmpDir, fmt.Errorf("cannot generate wrappers: %s already exists", wrapperVisible)
		}
		replace[wrapperVisible] = wrapperTmp
	}

	return replace, tmpDir, nil
}

// writeOverlay writes the go -overlay JSON file.
func writeOverlay(path string, replace map[string]string) error {
	data, err := json.MarshalIndent(struct {
		Replace map[string]string
	}{Replace: replace}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// hasPackageArg reports whether the passthrough args already name a package
// pattern (so we should not append ./...). Test-flag values like "TestPing" are
// not package patterns; those start with ./, ../, ".", or contain "...".
func hasPackageArg(args []string) bool {
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if a == "." || strings.HasPrefix(a, "./") || strings.HasPrefix(a, "../") || strings.Contains(a, "...") {
			return true
		}
	}
	return false
}

// skipDir reports whether a directory should be skipped during discovery.
func skipDir(root, path, name string) bool {
	if path == root {
		return false
	}
	switch name {
	case "vendor", "node_modules", "testdata":
		return true
	}
	return strings.HasPrefix(name, ".") && name != "."
}
