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

	data dataSet
}

// config holds the resolved settings for a [T], populated by [Option] values.
type config struct {
	baseURL    string
	httpClient *http.Client
	headers    http.Header
	auth       Auth
	data       dataSet
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

// WithData adds static test data a test reads with [T.Data], without data
// files - mainly for unit tests. Each key is a data file name; its value maps
// record names to records, and a record maps field names to values (or nested
// maps of fields):
//
//	pesto.WithData(map[string]any{
//		"users": map[string]any{"default": map[string]any{"id": 42}},
//	})
//
// A file given here replaces a file of the same name from the environment (see
// [WithEnvironment]); later options win.
func WithData(files map[string]any) Option {
	return func(c *config) {
		for k, v := range files {
			c.data.files[k] = v
		}
	}
}

// New creates a [T] wrapping t and configured by opts.
func New(t *testing.T, opts ...Option) *T {
	t.Helper()

	cfg := &config{
		httpClient: http.DefaultClient,
		headers:    http.Header{},
		data:       dataSet{files: map[string]any{}},
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
		data: cfg.data,
	}
}

// Run runs f as a subtest named name, like [testing.T.Run], but hands it a [T]
// whose client keeps this test's configuration and reports failures on the
// subtest. The standard form stays available as t.T.Run.
func (t *T) Run(name string, f func(*T)) bool {
	t.Helper()
	return t.T.Run(name, func(st *testing.T) {
		f(&T{
			T: st,
			Client: &Client{
				t:          st,
				baseURL:    t.Client.baseURL,
				httpClient: t.Client.httpClient,
				headers:    t.Client.headers.Clone(),
				auth:       t.Client.auth,
			},
			data: t.data,
		})
	})
}

// Data returns the static test data value at ref, a dotted path
// "<file>.<record>.<field>": "users.default.id" reads field id of record
// default in environments/<env>/data/users.yaml of the selected environment
// (or data set with [WithData]). Numbers and booleans are returned in their
// YAML form, e.g. "42".
//
// Static data is predefined and permanent in the target - the test only reads
// it. A missing reference fails the test immediately, naming what is missing
// and where.
func (t *T) Data(ref string) string {
	t.Helper()
	v, err := t.data.lookup(ref)
	if err != nil {
		t.Fatalf("pesto: data %s: %v", ref, err)
	}
	return v
}

// LookupData is like [T.Data] but reports a missing reference as ok == false
// instead of failing the test.
func (t *T) LookupData(ref string) (value string, ok bool) {
	v, err := t.data.lookup(ref)
	return v, err == nil
}
