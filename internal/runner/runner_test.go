package runner

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunTransformsAndPasses is the end-to-end proof of Phase 2: an authored
// func TestPing(t *pesto.T) in testdata/basic is transformed and run by the Go
// toolchain, and passes.
func TestRunTransformsAndPasses(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   "testdata/basic",
		Args:   []string{"-run", "TestPing", "-v", "./..."},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestPing") || !strings.Contains(combined, "PASS") {
		t.Errorf("expected TestPing to run and PASS:\n%s", combined)
	}
}

// TestRunWritesJUnitReport runs the fixture with --report junit and checks the
// XML report is written and reflects the passing test.
func TestRunWritesJUnitReport(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:       "testdata/basic",
		Args:       []string{"-run", "TestPing"},
		Report:     "junit",
		ReportPath: reportPath,
		Stdout:     &out,
		Stderr:     &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	xml := string(data)
	for _, want := range []string{`<testsuites`, `<testcase name="TestPing"`} {
		if !strings.Contains(xml, want) {
			t.Errorf("report missing %q\n---\n%s", want, xml)
		}
	}
	// The normal console output is still reproduced alongside the report.
	if !strings.Contains(combined, "TestPing") {
		t.Errorf("console output not reproduced:\n%s", combined)
	}
}

// TestRunSymlinkedRoot ensures a root reached through a symlink (as macOS temp
// dirs are) is resolved before building the overlay, so its keys match the paths
// go test sees.
func TestRunSymlinkedRoot(t *testing.T) {
	abs, err := filepath.Abs("testdata/basic")
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(abs, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}

	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   link,
		Args:   []string{"-run", "TestPing", "-v", "./..."},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestPing") || !strings.Contains(combined, "PASS") {
		t.Errorf("expected TestPing to run and PASS:\n%s", combined)
	}
}

// TestRunNoTests reports a clear error when a project has no Pesto tests.
func TestRunNoTests(t *testing.T) {
	_, err := Run(Options{Root: t.TempDir(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error when no Pesto tests are present")
	}
}

// TestRunFromSubdirectory runs go test from a directory inside the root: the
// default ./... pattern then covers only that subtree.
func TestRunFromSubdirectory(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   "testdata/basic",
		Dir:    "testdata/basic/sub",
		Args:   []string{"-v"},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestSub") {
		t.Errorf("expected TestSub to run:\n%s", combined)
	}
	if strings.Contains(combined, "TestPing") {
		t.Errorf("TestPing is outside the run directory and should not run:\n%s", combined)
	}
}

// TestRunDirOutsideRoot rejects a run directory that is not inside the root.
func TestRunDirOutsideRoot(t *testing.T) {
	_, err := Run(Options{Root: "testdata/basic", Dir: t.TempDir(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "outside the project root") {
		t.Fatalf("want outside-root error, got %v", err)
	}
}

// TestRunSymlinkedWorkingDirectory reproduces running from a shell that cd'd in
// through a symlink: $PWD holds the symlinked path, which go would otherwise use
// as its working directory and miss every (resolved) overlay key.
func TestRunSymlinkedWorkingDirectory(t *testing.T) {
	abs, err := filepath.Abs("testdata/basic")
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(abs, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	t.Setenv("PWD", link)

	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   link,
		Dir:    link,
		Args:   []string{"-run", "TestPing", "-v", "."},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}
}

// TestRunSubdirectoryWithoutTests fails with a clear error, rather than go's
// "no packages to test", when run from a folder with no tests below it.
func TestRunSubdirectoryWithoutTests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/p\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(root, "reports")
	if err := os.Mkdir(empty, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Run(Options{Root: root, Dir: empty, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "no tests in reports or its subfolders") {
		t.Fatalf("want no-tests error, got %v", err)
	}
}
