package pesto

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
)

// Client is a fluent HTTP builder bound to a test. Each verb method starts a
// [Request]; call [Request.Send] to execute it and get a [Response].
type Client struct {
	t          *testing.T
	baseURL    string
	httpClient *http.Client
	headers    http.Header
	auth       Auth
}

// GET starts a GET request for path (resolved against the client's base URL).
func (c *Client) GET(path string) *Request { return c.newRequest(http.MethodGet, path) }

// POST starts a POST request for path.
func (c *Client) POST(path string) *Request { return c.newRequest(http.MethodPost, path) }

// PUT starts a PUT request for path.
func (c *Client) PUT(path string) *Request { return c.newRequest(http.MethodPut, path) }

// PATCH starts a PATCH request for path.
func (c *Client) PATCH(path string) *Request { return c.newRequest(http.MethodPatch, path) }

// DELETE starts a DELETE request for path.
func (c *Client) DELETE(path string) *Request { return c.newRequest(http.MethodDelete, path) }

func (c *Client) newRequest(method, path string) *Request {
	r := &Request{
		t:      c.t,
		client: c,
		method: method,
		path:   path,
		header: http.Header{},
	}
	for key, values := range c.headers {
		for _, v := range values {
			r.header.Add(key, v)
		}
	}
	return r
}

// Request is an in-progress HTTP request. Builder methods return the request so
// calls can be chained; [Request.Send] executes it.
type Request struct {
	t      *testing.T
	client *Client
	method string
	path   string
	header http.Header
	body   []byte
	err    error
}

// JSON sets the request body to the JSON encoding of v and the Content-Type
// header to application/json.
func (r *Request) JSON(v any) *Request {
	if r.err != nil {
		return r
	}
	b, err := json.Marshal(v)
	if err != nil {
		r.err = err
		return r
	}
	r.body = b
	r.header.Set("Content-Type", "application/json")
	return r
}

// Header sets a header on the request, overriding any client default for key.
func (r *Request) Header(key, value string) *Request {
	r.header.Set(key, value)
	return r
}

// Send executes the request and returns the [Response]. A transport, encoding, or
// body-read error fails the test immediately via t.Fatalf.
func (r *Request) Send() *Response {
	r.t.Helper()

	if r.err != nil {
		r.t.Fatalf("pesto: build %s %s: %v", r.method, r.path, r.err)
	}

	fullURL := r.path
	if r.client.baseURL != "" {
		joined, err := url.JoinPath(r.client.baseURL, r.path)
		if err != nil {
			r.t.Fatalf("pesto: resolve URL for %s %s: %v", r.method, r.path, err)
		}
		fullURL = joined
	}

	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}

	req, err := http.NewRequestWithContext(context.Background(), r.method, fullURL, body)
	if err != nil {
		r.t.Fatalf("pesto: create request %s %s: %v", r.method, fullURL, err)
	}
	req.Header = r.header

	// Auth applies only when the request has no explicit Authorization header, so a
	// per-request Header call always overrides the client's default auth.
	if r.client.auth != nil && req.Header.Get("Authorization") == "" {
		r.client.auth.apply(req)
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		r.t.Fatalf("pesto: send %s %s: %v", r.method, fullURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		r.t.Fatalf("pesto: read response body from %s %s: %v", r.method, fullURL, err)
	}

	return &Response{
		t:      r.t,
		status: resp.StatusCode,
		header: resp.Header,
		body:   respBody,
	}
}
