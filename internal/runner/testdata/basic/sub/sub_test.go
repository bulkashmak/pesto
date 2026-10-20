package sub

import (
	"net/http"
	"net/http/httptest"

	"github.com/bulkashmak/pesto"
)

// TestSub lives in a nested package so tests can run `pesto run` from a
// subdirectory of the project and check only this package runs.
func TestSub(t *pesto.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	t.Client.GET(srv.URL).Send().Expect().Status(204)
}
