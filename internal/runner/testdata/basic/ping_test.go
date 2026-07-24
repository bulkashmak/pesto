package basic

import (
	"net/http"
	"net/http/httptest"

	"github.com/bulkashmak/pesto"
)

// TestPing is authored with the clean *pesto.T signature. It is not a runnable Go
// test on its own; `pesto run` transforms it into a standard test. It starts a
// local server so the run needs no external environment.
func TestPing(t *pesto.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(srv.Close)

	t.Client.GET(srv.URL).Send().
		Expect().
		Status(200).
		JSON("status").Equals("ok")
}
