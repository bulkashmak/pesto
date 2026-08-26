package generate_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto/internal/generate"
	"github.com/bulkashmak/pesto/internal/runner"
	"github.com/bulkashmak/pesto/internal/scaffold"
)

// TestGeneratedFilesCompile scaffolds a project, generates a suite, an E2E test,
// and a client stub, and drives the project through pesto run with -run '^$' so
// everything is transformed and compiled without hitting the network. This proves
// the generate templates produce code that builds end to end.
func TestGeneratedFilesCompile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	if err := scaffold.Create(scaffold.Options{
		Dir: dir, Module: "example.com/acme", PestoVersion: "v0.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := generate.E2E(dir, "transfer/happy_path"); err != nil {
		t.Fatal(err)
	}
	if _, err := generate.Client(dir, "users"); err != nil {
		t.Fatal(err)
	}

	// Point the pesto require at this local checkout so the project resolves
	// offline (a released CLI would inject a downloadable version instead).
	repoRoot := repoRootFromTest(t)
	appendToFile(t, filepath.Join(dir, "go.mod"),
		"\nreplace github.com/bulkashmak/pesto => "+repoRoot+"\n")
	tidy := exec.Command("go", "mod", "tidy")
	tidy.Dir = dir
	if tidyOut, err := tidy.CombinedOutput(); err != nil {
		t.Fatalf("go mod tidy: %v\n%s", err, tidyOut)
	}

	var out, errOut bytes.Buffer
	code, err := runner.Run(runner.Options{
		Root:   dir,
		Args:   []string{"-run", "^$", "-v"},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0 (generated files should compile)\n%s", code, combined)
	}
	if !strings.Contains(combined, "example.com/acme/tests/e2e/transfer") {
		t.Errorf("expected the generated package to build:\n%s", combined)
	}
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller path")
	}
	// this file: <repo>/internal/generate/e2e_test.go
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func appendToFile(t *testing.T, path, text string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		t.Fatal(err)
	}
}
