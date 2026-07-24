package pesto

import (
	"net/http"
	"testing"
)

// T is a Pesto test handle. It embeds *testing.T, so every standard testing
// method (Parallel, Run, Cleanup, Errorf, Fatalf, Helper, ...) is available
// directly, and it adds a fluent HTTP [Client].
//
// Because *testing.T is embedded rather than hidden, a test keeps full access to
// the standard library: t.Parallel() can be injected by the transform regardless
// of assertion style, and any assertion library that accepts a testing.TB works.
type T struct {
	*testing.T

	// Client is the HTTP client bound to this test. Requests it builds fail the
	// test on transport errors via the embedded *testing.T.
	Client *Client
}

// config holds the resolved settings for a [T], populated by [Option] values.
type config struct {
	baseURL    string
	httpClient *http.Client
	headers    http.Header
	auth       Auth
}

// Option configures a [T] created by [New].
type Option func(*config)

// WithBaseURL sets the base URL that request paths are resolved against.
func WithBaseURL(url string) Option {
	return func(c *config) { c.baseURL = url }
}

// WithHTTPClient overrides the underlying *http.Client (for custom timeouts,
// transports, or TLS configuration).
func WithHTTPClient(client *http.Client) Option {
	return func(c *config) { c.httpClient = client }
}

// WithHeader adds a default header sent on every request from this test's client.
// It may be called multiple times to add several headers.
func WithHeader(key, value string) Option {
	return func(c *config) { c.headers.Add(key, value) }
}

// WithAuth sets the authentication applied to every request (see [BearerAuth],
// [BasicAuth]).
func WithAuth(auth Auth) Option {
	return func(c *config) { c.auth = auth }
}

// New creates a [T] wrapping t and configured by opts.
func New(t *testing.T, opts ...Option) *T {
	t.Helper()

	cfg := &config{
		httpClient: http.DefaultClient,
		headers:    http.Header{},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	return &T{
		T: t,
		Client: &Client{
			t:          t,
			baseURL:    cfg.baseURL,
			httpClient: cfg.httpClient,
			headers:    cfg.headers,
			auth:       cfg.auth,
		},
	}
}
