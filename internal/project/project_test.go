package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// tempDir returns a symlink-free temp dir so paths compare equal on macOS.
func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestFindRootWalksUpToPestoYAML(t *testing.T) {
	root := tempDir(t)
	touch(t, filepath.Join(root, "go.mod"))
	touch(t, filepath.Join(root, "pesto.yaml"))
	sub := filepath.Join(root, "tests", "e2e", "example")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	for _, start := range []string{root, sub} {
		got, err := FindRoot(start)
		if err != nil {
			t.Fatal(err)
		}
		if got != root {
			t.Errorf("FindRoot(%s) = %s, want %s", start, got, root)
		}
	}
}

func TestFindRootPrefersPestoYAMLOverNearerGoMod(t *testing.T) {
	root := tempDir(t)
	touch(t, filepath.Join(root, "pesto.yaml"))
	nested := filepath.Join(root, "tools")
	touch(t, filepath.Join(nested, "go.mod"))

	got, err := FindRoot(nested)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("got %s, want %s", got, root)
	}
}

func TestFindRootFallsBackToGoMod(t *testing.T) {
	root := tempDir(t)
	touch(t, filepath.Join(root, "go.mod"))
	sub := filepath.Join(root, "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRoot(sub)
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Errorf("got %s, want %s", got, root)
	}
}

func TestFindRootNotFound(t *testing.T) {
	_, err := FindRoot(tempDir(t))
	if err == nil || !strings.Contains(err.Error(), "no Pesto project found") {
		t.Fatalf("want not-found error, got %v", err)
	}
}

func TestCheck(t *testing.T) {
	project := tempDir(t)
	touch(t, filepath.Join(project, "pesto.yaml"))
	modOnly := tempDir(t)
	touch(t, filepath.Join(modOnly, "go.mod"))
	empty := tempDir(t)
	file := filepath.Join(empty, "file.txt")
	touch(t, file)

	cases := []struct {
		root, wantErr string
	}{
		{root: project},
		{root: modOnly},
		{root: filepath.Join(empty, "missing"), wantErr: "does not exist"},
		{root: file, wantErr: "is not a directory"},
		{root: empty, wantErr: "is not a Pesto project"},
	}
	for _, c := range cases {
		err := Check(c.root)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("Check(%s) = %v, want nil", c.root, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("Check(%s) = %v, want error containing %q", c.root, err, c.wantErr)
		}
	}
}
