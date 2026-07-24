package pesto_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bulkashmak/pesto"
)

// newServer returns a small fake API used by the tests below.
func newServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    1,
			"name":  in["name"],
			"roles": []string{"admin", "user"},
		})
	})

	mux.HandleFunc("GET /users/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "name": "Bob"})
	})

	return httptest.NewServer(mux)
}

// TestCreateUser exercises the fluent request + Expect assertion chain.
func TestCreateUser(t *testing.T) {
	srv := newServer()
	t.Cleanup(srv.Close)

	pt := pesto.New(t, pesto.WithBaseURL(srv.URL))

	resp := pt.Client.POST("/users").
		JSON(map[string]any{"name": "Bob"}).
		Send()

	resp.Expect().
		Status(201).
		Header("Content-Type").Contains("application/json").
		JSON("id").Exists().
		JSON("name").Equals("Bob").
		JSON("roles").Length(2).
		JSON("roles.0").Equals("admin")
}

// TestDecodeEscapeHatch proves the "not a walled garden" principle: decode into a
// struct and assert with the standard library, no Pesto assertions required.
func TestDecodeEscapeHatch(t *testing.T) {
	srv := newServer()
	t.Cleanup(srv.Close)

	pt := pesto.New(t, pesto.WithBaseURL(srv.URL))

	resp := pt.Client.GET("/users/1").Send()

	var user struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	resp.Decode(&user)

	if user.Name != "Bob" {
		t.Errorf("name = %q, want Bob", user.Name)
	}
}

// TestParallelPassthrough proves *pesto.T embeds *testing.T, so t.Parallel() works
// through the wrapper — the mechanism the transform relies on.
func TestParallelPassthrough(t *testing.T) {
	srv := newServer()
	t.Cleanup(srv.Close)

	pt := pesto.New(t, pesto.WithBaseURL(srv.URL))
	pt.Parallel()

	pt.Client.GET("/users/1").Send().
		Expect().
		Status(200).
		JSON("name").Equals("Bob")
}
