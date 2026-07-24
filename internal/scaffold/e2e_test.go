package scaffold_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto/internal/runner"
	"github.com/bulkashmak/pesto/internal/scaffold"
)

// TestInitThenRunCompiles scaffolds a project and drives it through pesto run with
// -run '^$' so the example is transformed and compiled but no test body executes.
// This proves `pesto init` produces a project that builds end-to-end, without any
// network call to the example's public API.
func TestInitThenRunCompiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	if err := scaffold.Create(scaffold.Options{
		Dir: dir, Module: "example.com/acme", PestoVersion: "v0.0.0",
	}); err != nil {
		t.Fatal(err)
	}

	// Point the pesto require at this local checkout so the scaffold resolves
	// offline (a released CLI would inject a downloadable version instead).
	repoRoot := repoRootFromTest(t)
	appendToFile(t, filepath.Join(dir, "go.mod"),
		"\nreplace github.com/bulkashmak/pesto => "+repoRoot+"\n")

	// Resolve the dependency graph (offline, from the module cache) the way a user
	// would with `go get github.com/bulkashmak/pesto@latest` after init.
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
		t.Fatalf("exit = %d, want 0 (scaffold should compile)\n%s", code, combined)
	}
	if !strings.Contains(combined, "example.com/acme/tests/e2e/example") {
		t.Errorf("expected the example package to build:\n%s", combined)
	}
}

func repoRootFromTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine caller path")
	}
	// this file: <repo>/internal/scaffold/e2e_test.go
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
