package runner

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// cardsServer fakes POST /users/42/cards, where account 7 is the only account.
// A strict server answers 422 to every body that breaks the fixture's request
// schema; a sloppy one accepts a body without "limit".
func cardsServer(t *testing.T, strict bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/users/42/cards" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		if json.NewDecoder(r.Body).Decode(&body) != nil || !validCard(body, strict) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id": 1}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("PESTO_CONTRACT_URL", srv.URL)
}

func validCard(b map[string]any, strict bool) bool {
	for k := range b {
		if k != "account_id" && k != "number" && k != "limit" {
			return false
		}
	}
	if id, ok := b["account_id"].(float64); !ok || id != 7 {
		return false
	}
	if n, ok := b["number"].(string); !ok || len(n) != 16 {
		return false
	}
	limit, present := b["limit"]
	if !present {
		return !strict
	}
	f, ok := limit.(float64)
	return ok && f == float64(int64(f)) && f >= 0
}

func runContractFixture(t *testing.T) (int, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   "testdata/contract",
		Env:    "test",
		Args:   []string{"-v", "-count=1", "./..."},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	return code, combined
}

// TestRunContract drives a contract bundle through the whole pipeline: the
// transform, environment data for the path param and a body field, and the
// contract engine.
func TestRunContract(t *testing.T) {
	cardsServer(t, true)
	code, out := runContractFixture(t)
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, out)
	}
	for _, want := range []string{
		"--- PASS: TestContract/happy_path",
		"--- PASS: TestContract/violation/required/limit",
		"--- PASS: TestContract/violation/additional_properties/body",
		"--- PASS: TestContract/violation/type/account_id",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

// TestRunContractCatchesViolation checks that a server accepting a missing
// required field fails exactly that case, with a non-zero exit.
func TestRunContractCatchesViolation(t *testing.T) {
	cardsServer(t, false)
	code, out := runContractFixture(t)
	if code == 0 {
		t.Fatalf("exit = 0, want a failure\n%s", out)
	}
	if !strings.Contains(out, "--- FAIL: TestContract/violation/required/limit") {
		t.Errorf("expected required/limit to fail:\n%s", out)
	}
	if n := strings.Count(out, "--- FAIL: TestContract/"); n != 1 {
		t.Errorf("%d failing cases, want 1:\n%s", n, out)
	}
}
