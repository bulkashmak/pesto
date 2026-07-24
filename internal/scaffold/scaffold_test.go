package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWritesExpectedLayout(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	if err := Create(Options{Dir: dir, Module: "example.com/acme", PestoVersion: "v0.0.0"}); err != nil {
		t.Fatal(err)
	}

	for _, rel := range []string{
		"go.mod", "pesto.yaml", ".gitignore", "README.md",
		"environments/dev.yaml", "environments/test.yaml",
		"tests/e2e/example/example_test.go",
		"clients/.gitkeep", "fixtures/.gitkeep", "helpers/.gitkeep", "reports/.gitkeep",
	} {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
			t.Errorf("missing scaffolded file %s: %v", rel, err)
		}
	}

	goMod := readFile(t, filepath.Join(dir, "go.mod"))
	if !strings.Contains(goMod, "module example.com/acme") {
		t.Errorf("go.mod missing module line:\n%s", goMod)
	}
	if !strings.Contains(goMod, "require github.com/bulkashmak/pesto v0.0.0") {
		t.Errorf("go.mod missing pesto require:\n%s", goMod)
	}

	pestoYAML := readFile(t, filepath.Join(dir, "pesto.yaml"))
	if !strings.Contains(pestoYAML, "default_env: dev") {
		t.Errorf("pesto.yaml missing default_env:\n%s", pestoYAML)
	}

	example := readFile(t, filepath.Join(dir, "tests/e2e/example/example_test.go"))
	if !strings.Contains(example, "func TestGetTodo(t *pesto.T)") {
		t.Errorf("example test missing *pesto.T signature:\n%s", example)
	}
}

func TestCreateRefusesToOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	opts := Options{Dir: dir, Module: "example.com/acme", PestoVersion: "v0.0.0"}
	if err := Create(opts); err != nil {
		t.Fatal(err)
	}
	if err := Create(opts); err == nil {
		t.Fatal("expected second Create to refuse overwriting existing files")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
