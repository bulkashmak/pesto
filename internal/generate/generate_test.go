package generate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projectDir creates a minimal directory generate accepts as a project root.
func projectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/p\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	return string(b)
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "transfer", want: "transfer"},
		{in: "Happy-Path", want: "happy_path"},
		{in: "login 2fa", want: "login_2fa"},
		{in: "9lives", wantErr: true},
		{in: "", wantErr: true},
		{in: "café", wantErr: true},
		{in: "_x", wantErr: true},
	}
	for _, c := range cases {
		got, err := normalize(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("normalize(%q) = %q, want error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("normalize(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestExportName(t *testing.T) {
	for in, want := range map[string]string{
		"happy_path": "HappyPath",
		"transfer":   "Transfer",
		"login_2fa":  "Login2fa",
	} {
		if got := exportName(in); got != want {
			t.Errorf("exportName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestE2ECreatesSuiteAndTest(t *testing.T) {
	root := projectDir(t)
	created, err := E2E(root, "transfer/happy-path")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("created = %v, want suite + test", created)
	}

	suite := readFile(t, filepath.Join(root, "tests", "e2e", "transfer", "suite_test.go"))
	if !strings.Contains(suite, "package transfer") {
		t.Errorf("suite file:\n%s", suite)
	}

	test := readFile(t, filepath.Join(root, "tests", "e2e", "transfer", "happy_path_test.go"))
	for _, want := range []string{"package transfer", "func TestHappyPath(t *pesto.T)"} {
		if !strings.Contains(test, want) {
			t.Errorf("test file missing %q:\n%s", want, test)
		}
	}

	// A second test in the same domain must not re-create the suite.
	created, err = E2E(root, "transfer/limits")
	if err != nil {
		t.Fatal(err)
	}
	if len(created) != 1 {
		t.Errorf("second e2e created = %v, want only the test file", created)
	}
}

func TestE2ERejectsBadArgs(t *testing.T) {
	root := projectDir(t)
	for _, arg := range []string{"transfer", "a/b/c", "transfer/9lives", "Bad!/name"} {
		if _, err := E2E(root, arg); err == nil {
			t.Errorf("E2E(%q) succeeded, want error", arg)
		}
	}
}

func TestGenerateRefusesOverwrite(t *testing.T) {
	root := projectDir(t)
	if _, err := E2E(root, "transfer/happy_path"); err != nil {
		t.Fatal(err)
	}
	if _, err := E2E(root, "transfer/happy_path"); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("overwrite err = %v, want refusal", err)
	}
	if _, err := Suite(root, "transfer"); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("suite overwrite err = %v, want refusal", err)
	}
}

func TestClientStub(t *testing.T) {
	root := projectDir(t)
	if _, err := Client(root, "users"); err != nil {
		t.Fatal(err)
	}
	stub := readFile(t, filepath.Join(root, "clients", "users", "users.go"))
	for _, want := range []string{"package users", "*pesto.Client", "func New(c *pesto.Client) *Client"} {
		if !strings.Contains(stub, want) {
			t.Errorf("client stub missing %q:\n%s", want, stub)
		}
	}
}

func TestRequiresProjectRoot(t *testing.T) {
	if _, err := E2E(t.TempDir(), "transfer/happy_path"); err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Errorf("err = %v, want a no-go.mod project error", err)
	}
}
