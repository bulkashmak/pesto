package pesto

import (
	"encoding/json"
	"net/http"
	"testing"
)

// Response is a completed HTTP response captured by [Request.Send]. Its body is
// read fully into memory, so it can be inspected repeatedly by both [Expect] and
// [Response.Decode].
type Response struct {
	t      *testing.T
	status int
	header http.Header
	body   []byte
}

// Status returns the HTTP status code.
func (r *Response) Status() int { return r.status }

// Header returns the response headers.
func (r *Response) Header() http.Header { return r.header }

// Body returns the raw response body.
func (r *Response) Body() []byte { return r.body }

// Expect begins a fluent assertion chain over the response.
func (r *Response) Expect() *Expectation {
	return &Expectation{t: r.t, resp: r}
}

// Decode unmarshals the JSON response body into v. A decode error fails the test
// immediately. This is the first-class escape hatch: decode into a struct, then
// assert with any library you prefer.
func (r *Response) Decode(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		r.t.Fatalf("pesto: decode response body: %v\nbody: %s", err, r.body)
	}
}
