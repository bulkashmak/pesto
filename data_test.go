package pesto_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto"
)

// writeFiles writes files (relative path -> content) under a temp project root
// and returns the root.
func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// loadT loads environment name from root and returns a T configured by it.
func loadT(t *testing.T, root, name string) *pesto.T {
	t.Helper()
	env, err := pesto.LoadEnvironment(root, name)
	if err != nil {
		t.Fatal(err)
	}
	return pesto.New(t, pesto.WithEnvironment(env))
}

// TestEnvironmentFolderData loads an environment folder and reads its data:
// scalars keep their YAML text, nested fields resolve, and ${VAR} expands.
func TestEnvironmentFolderData(t *testing.T) {
	t.Setenv("DEV_PASSWORD", "s3cret")
	root := writeFiles(t, map[string]string{
		"environments/dev/env.yaml": "base_url: https://dev.example.com\n",
		"environments/dev/data/users.yaml": `
default:
  id: 42
  email: qa@example.com
  password: ${DEV_PASSWORD}
  active: true
  address:
    city: Kazan
`,
		"environments/dev/data/notes.txt": "not data",
	})
	pt := loadT(t, root, "dev")
	for ref, want := range map[string]string{
		"users.default.id":           "42",
		"users.default.email":        "qa@example.com",
		"users.default.password":     "s3cret",
		"users.default.active":       "true",
		"users.default.address.city": "Kazan",
	} {
		if got := pt.Data(ref); got != want {
			t.Errorf("Data(%s) = %q, want %q", ref, got, want)
		}
	}
}

// TestEnvironmentFlatFileHasNoData keeps the v0.1.0 flat form working, as an
// environment with no data.
func TestEnvironmentFlatFileHasNoData(t *testing.T) {
	root := writeFiles(t, map[string]string{
		"environments/dev.yaml": "base_url: https://dev.example.com\nvariables:\n  old: ignored\n",
	})
	env, err := pesto.LoadEnvironment(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if env.BaseURL != "https://dev.example.com" {
		t.Errorf("base_url = %q", env.BaseURL)
	}
	if _, ok := pesto.New(t, pesto.WithEnvironment(env)).LookupData("users.default.id"); ok {
		t.Error("flat environment reported data")
	}
}

// TestLoadEnvironmentErrors covers the ways an environment or its data can be
// malformed.
func TestLoadEnvironmentErrors(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "defined twice",
			files: map[string]string{
				"environments/dev.yaml":     "base_url: https://x\n",
				"environments/dev/env.yaml": "base_url: https://x\n",
			},
			want: `environment "dev" is defined twice`,
		},
		{
			name:  "folder without env.yaml",
			files: map[string]string{"environments/dev/data/users.yaml": "default:\n  id: 1\n"},
			want:  "has no env.yaml",
		},
		{
			name:  "not found",
			files: map[string]string{"environments/test.yaml": "base_url: https://x\n"},
			want:  `environment "dev" not found`,
		},
		{
			name: "record is not a map",
			files: map[string]string{
				"environments/dev/env.yaml":        "base_url: https://x\n",
				"environments/dev/data/users.yaml": "default: 42\n",
			},
			want: `users.yaml: record "default" (line 1) must be a map of fields`,
		},
		{
			name: "list value",
			files: map[string]string{
				"environments/dev/env.yaml":        "base_url: https://x\n",
				"environments/dev/data/users.yaml": "default:\n  roles: [admin]\n",
			},
			want: "default.roles (line 2): lists are not supported",
		},
		{
			name: "dotted file name",
			files: map[string]string{
				"environments/dev/env.yaml":           "base_url: https://x\n",
				"environments/dev/data/users.v2.yaml": "default:\n  id: 1\n",
			},
			want: "a data file name cannot contain a dot",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pesto.LoadEnvironment(writeFiles(t, tc.files), "dev")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestLookupDataMisses checks the references LookupData reports as missing.
func TestLookupDataMisses(t *testing.T) {
	pt := pesto.New(t, pesto.WithData(map[string]any{
		"users": map[string]any{"default": map[string]any{
			"id":      42,
			"address": map[string]any{"city": "Kazan"},
			"nothing": nil,
		}},
	}))
	if v, ok := pt.LookupData("users.default.id"); !ok || v != "42" {
		t.Errorf("LookupData(users.default.id) = %q, %v; want 42, true", v, ok)
	}
	for _, ref := range []string{
		"users",                   // too short
		"users.default",           // a record, not a value
		"users..id",               // empty part
		"cards.default.id",        // no file
		"users.blocked.id",        // no record
		"users.default.email",     // no field
		"users.default.address",   // a map, not a value
		"users.default.id.nested", // through a value
		"users.default.nothing",   // null
	} {
		if v, ok := pt.LookupData(ref); ok {
			t.Errorf("LookupData(%s) = %q, true; want missing", ref, v)
		}
	}
}

// TestDataMissingIsFatal checks, in a child process, that Data fails the test
// with a message naming the environment's file and the missing record.
func TestDataMissingIsFatal(t *testing.T) {
	if root := os.Getenv("PESTO_DATA_ROOT"); root != "" {
		pt := loadT(t, root, "staging")
		pt.Data("users.default.id")
		t.Log("REACHED AFTER DATA")
		return
	}

	root := writeFiles(t, map[string]string{
		"environments/staging/env.yaml":        "base_url: https://x\n",
		"environments/staging/data/users.yaml": "blocked:\n  id: 43\n",
	})
	cmd := exec.Command(os.Args[0], "-test.run=^TestDataMissingIsFatal$", "-test.v")
	cmd.Env = append(os.Environ(), "PESTO_DATA_ROOT="+root)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("test passed with missing data:\n%s", out)
	}
	got := string(out)
	want := `pesto: data users.default.id: no record "default" in ` + filepath.Join("environments", "staging", "data", "users.yaml")
	if !strings.Contains(got, want) {
		t.Errorf("output lacks %q:\n%s", want, got)
	}
	if strings.Contains(got, "REACHED AFTER DATA") {
		t.Errorf("test continued after missing data:\n%s", got)
	}
}
