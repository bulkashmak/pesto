package generate

import (
	"errors"
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

func TestContractDir(t *testing.T) {
	cases := []struct {
		path, dir, params string
	}{
		{"/users", "users", ""},
		{"/users/{id}", "users/by_id", "  id: users.default.id"},
		{"/users/{id}/cards", "users/by_id/cards", "  id: users.default.id"},
		{"/credit-cards/{cardId}", "credit_cards/by_cardid", "  cardId: credit_cards.default.cardid"},
		{"/{tenant}/orders/{id}", "by_tenant/orders/by_id", "  tenant: resources.default.tenant\n  id: orders.default.id"},
		{"/", "", ""},
	}
	for _, c := range cases {
		dir, params, err := contractDir(c.path)
		if err != nil {
			t.Errorf("contractDir(%q): %v", c.path, err)
			continue
		}
		if dir != filepath.FromSlash(c.dir) || params != c.params {
			t.Errorf("contractDir(%q) = %q, %q; want %q, %q", c.path, dir, params, c.dir, c.params)
		}
	}
	for _, bad := range []string{"users", "/users/", "/a//b", "/a/{x}y", "/a?q=1", "/{9}", "/café"} {
		if _, _, err := contractDir(bad); !errors.Is(err, ErrInvalidName) {
			t.Errorf("contractDir(%q) error = %v, want ErrInvalidName", bad, err)
		}
	}
}

func TestContract(t *testing.T) {
	root := projectDir(t)
	created, err := Contract(root, "post", "/users/{id}/cards")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "tests", "contract", "users", "by_id", "cards", "post")
	want := []string{"contract.yaml", "contract_test.go", "request.schema.json", "response.schema.json"}
	if len(created) != len(want) {
		t.Fatalf("created %v, want %v", created, want)
	}
	for i, name := range want {
		if created[i] != filepath.Join(dir, name) {
			t.Errorf("created[%d] = %s, want %s", i, created[i], filepath.Join(dir, name))
		}
	}
	yaml := readFile(t, filepath.Join(dir, "contract.yaml"))
	for _, s := range []string{"method: POST", "path: /users/{id}/cards", "  id: users.default.id", "body: request.schema.json", "status: 201", "on_violation:"} {
		if !strings.Contains(yaml, s) {
			t.Errorf("contract.yaml lacks %q:\n%s", s, yaml)
		}
	}
	if src := readFile(t, filepath.Join(dir, "contract_test.go")); !strings.Contains(src, "package post") || !strings.Contains(src, `contract.Run(t, "contract.yaml")`) {
		t.Errorf("contract_test.go:\n%s", src)
	}

	// A second run must refuse without touching anything.
	if _, err := Contract(root, "POST", "/users/{id}/cards"); err == nil || !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("second Contract error = %v", err)
	}
}

func TestContractBodiesByMethod(t *testing.T) {
	cases := []struct {
		method          string
		request, status string
		response        bool
	}{
		{"GET", "", "status: 200", true},
		{"PUT", "request.schema.json", "status: 200", true},
		{"PATCH", "request.schema.json", "status: 200", true},
		{"DELETE", "", "status: 204", false},
	}
	for _, c := range cases {
		root := projectDir(t)
		if _, err := Contract(root, c.method, "/users/{id}"); err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(root, "tests", "contract", "users", "by_id", strings.ToLower(c.method))
		yaml := readFile(t, filepath.Join(dir, "contract.yaml"))
		if !strings.Contains(yaml, c.status) {
			t.Errorf("%s: contract.yaml lacks %q:\n%s", c.method, c.status, yaml)
		}
		if got := fileExists(filepath.Join(dir, "request.schema.json")); got != (c.request != "") {
			t.Errorf("%s: request schema exists = %v", c.method, got)
		}
		if got := fileExists(filepath.Join(dir, "response.schema.json")); got != c.response {
			t.Errorf("%s: response schema exists = %v", c.method, got)
		}
	}
	if _, err := Contract(projectDir(t), "FETCH", "/x"); !errors.Is(err, ErrInvalidName) {
		t.Errorf("bad method error = %v, want ErrInvalidName", err)
	}
}
