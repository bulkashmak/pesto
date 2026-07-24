package pesto_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bulkashmak/pesto"
)

// writeProject lays down a minimal Pesto project (pesto.yaml + one environment)
// in a temp dir and returns its root.
func writeProject(t *testing.T, pestoYAML, envName, envYAML string) string {
	t.Helper()
	root := t.TempDir()
	if pestoYAML != "" {
		if err := os.WriteFile(filepath.Join(root, "pesto.yaml"), []byte(pestoYAML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	envDir := filepath.Join(root, "environments")
	if err := os.MkdirAll(envDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envDir, envName+".yaml"), []byte(envYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// TestLoadEnvironmentDefault resolves the environment via default_env and expands
// a ${VAR} token from the process environment.
func TestLoadEnvironmentDefault(t *testing.T) {
	t.Setenv("API_TOKEN", "secret-123")
	root := writeProject(t,
		"default_env: dev\n",
		"dev", `
base_url: https://api.example.com
auth:
  type: bearer
  token: ${API_TOKEN}
headers:
  X-Team: qa
`)

	env, err := pesto.LoadEnvironment(root, "") // empty -> default_env
	if err != nil {
		t.Fatal(err)
	}
	if env.Name != "dev" {
		t.Errorf("name = %q, want dev", env.Name)
	}
	if env.BaseURL != "https://api.example.com" {
		t.Errorf("base_url = %q", env.BaseURL)
	}
	if env.Auth.Token != "secret-123" {
		t.Errorf("token = %q, want expanded secret-123", env.Auth.Token)
	}
	if env.Headers["X-Team"] != "qa" {
		t.Errorf("header X-Team = %q", env.Headers["X-Team"])
	}
}

// TestBearerAuthAppliedFromEnvironment loads bearer auth from a file and confirms
// it reaches the wire.
func TestBearerAuthAppliedFromEnvironment(t *testing.T) {
	t.Setenv("API_TOKEN", "tok-42")

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	root := writeProject(t, "default_env: dev\n", "dev", `
base_url: `+srv.URL+`
auth:
  type: bearer
  token: ${API_TOKEN}
`)
	env, err := pesto.LoadEnvironment(root, "")
	if err != nil {
		t.Fatal(err)
	}

	pt := pesto.New(t, pesto.WithEnvironment(env))
	pt.Client.GET("/ping").Send().Expect().Status(200)

	if gotAuth != "Bearer tok-42" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok-42")
	}
}

// TestBasicAuthApplied confirms basic credentials reach the wire via WithAuth.
func TestBasicAuthApplied(t *testing.T) {
	var user, pass string
	var ok bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok = r.BasicAuth()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	pt := pesto.New(t,
		pesto.WithBaseURL(srv.URL),
		pesto.WithAuth(pesto.BasicAuth("alice", "hunter2")),
	)
	pt.Client.GET("/ping").Send().Expect().Status(200)

	if !ok || user != "alice" || pass != "hunter2" {
		t.Errorf("basic auth = (%q, %q, ok=%v), want (alice, hunter2, true)", user, pass, ok)
	}
}

// TestPerRequestAuthOverridesDefault confirms an explicit Authorization header
// beats the client's configured auth.
func TestPerRequestAuthOverridesDefault(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	pt := pesto.New(t,
		pesto.WithBaseURL(srv.URL),
		pesto.WithAuth(pesto.BearerAuth("default-token")),
	)
	pt.Client.GET("/ping").
		Header("Authorization", "Bearer override").
		Send().Expect().Status(200)

	if gotAuth != "Bearer override" {
		t.Errorf("Authorization = %q, want per-request override to win", gotAuth)
	}
}

// TestLoadEnvironmentMissingDefault surfaces a clear error when nothing selects an
// environment.
func TestLoadEnvironmentMissingDefault(t *testing.T) {
	root := writeProject(t, "", "dev", "base_url: https://x\n") // no pesto.yaml
	if _, err := pesto.LoadEnvironment(root, ""); err == nil {
		t.Fatal("expected error when no environment is named and no default_env")
	}
}
