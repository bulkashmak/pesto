package pesto

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// Expectation is a fluent, soft assertion chain over a [Response]. Each check
// reports a failure with t.Errorf but does not stop the test, so a single chain
// surfaces every mismatch at once. Every method returns the Expectation so checks
// can be chained.
type Expectation struct {
	t    *testing.T
	resp *Response

	decoded    any  // lazily parsed JSON body
	decodeOK   bool // whether decoded is valid
	decodeDone bool
}

// Status asserts the response status code equals code.
func (e *Expectation) Status(code int) *Expectation {
	e.t.Helper()
	if e.resp.status != code {
		e.t.Errorf("pesto: status = %d, want %d", e.resp.status, code)
	}
	return e
}

// Header begins an assertion on the named response header.
func (e *Expectation) Header(name string) *HeaderAssertion {
	return &HeaderAssertion{e: e, name: name}
}

// JSON begins an assertion on the value at a dotted path in the JSON body.
// Path segments index into objects by key and into arrays by number, e.g.
// "data.items.0.id". An empty path refers to the whole body.
func (e *Expectation) JSON(path string) *JSONAssertion {
	return &JSONAssertion{e: e, path: path}
}

// body returns the parsed JSON body, parsing it once on first use.
func (e *Expectation) body() (any, bool) {
	if !e.decodeDone {
		e.decodeDone = true
		e.decodeOK = json.Unmarshal(e.resp.body, &e.decoded) == nil
	}
	return e.decoded, e.decodeOK
}

// HeaderAssertion asserts on a single response header. Its methods return the
// parent [Expectation] so the chain continues.
type HeaderAssertion struct {
	e    *Expectation
	name string
}

// Exists asserts the header is present.
func (h *HeaderAssertion) Exists() *Expectation {
	h.e.t.Helper()
	if _, ok := h.e.resp.header[http.CanonicalHeaderKey(h.name)]; !ok {
		h.e.t.Errorf("pesto: header %q = <missing>, want present", h.name)
	}
	return h.e
}

// Equals asserts the header's value equals want.
func (h *HeaderAssertion) Equals(want string) *Expectation {
	h.e.t.Helper()
	if got := h.e.resp.header.Get(h.name); got != want {
		h.e.t.Errorf("pesto: header %q = %q, want %q", h.name, got, want)
	}
	return h.e
}

// Contains asserts the header's value contains sub.
func (h *HeaderAssertion) Contains(sub string) *Expectation {
	h.e.t.Helper()
	if got := h.e.resp.header.Get(h.name); !strings.Contains(got, sub) {
		h.e.t.Errorf("pesto: header %q = %q, want to contain %q", h.name, got, sub)
	}
	return h.e
}

// JSONAssertion asserts on a value at a JSON path. Its methods return the parent
// [Expectation] so the chain continues.
type JSONAssertion struct {
	e    *Expectation
	path string
}

// Exists asserts a value is present at the path.
func (j *JSONAssertion) Exists() *Expectation {
	j.e.t.Helper()
	if _, ok := j.lookup(); !ok {
		j.e.t.Errorf("pesto: JSON %q = <missing>, want present", j.path)
	}
	return j.e
}

// Equals asserts the value at the path equals want. Comparison is JSON-aware:
// want is normalized through JSON encoding so numeric types line up (e.g. an int
// literal matches a JSON number).
func (j *JSONAssertion) Equals(want any) *Expectation {
	j.e.t.Helper()
	got, ok := j.lookup()
	if !ok {
		j.e.t.Errorf("pesto: JSON %q = <missing>, want %v", j.path, want)
		return j.e
	}
	if !reflect.DeepEqual(got, normalize(want)) {
		j.e.t.Errorf("pesto: JSON %q = %v, want %v", j.path, got, want)
	}
	return j.e
}

// Length asserts the value at the path is an array, object, or string of length n.
func (j *JSONAssertion) Length(n int) *Expectation {
	j.e.t.Helper()
	got, ok := j.lookup()
	if !ok {
		j.e.t.Errorf("pesto: JSON %q = <missing>, want length %d", j.path, n)
		return j.e
	}
	got, gotOK := lengthOf(got)
	if !gotOK {
		j.e.t.Errorf("pesto: JSON %q is not measurable for length", j.path)
		return j.e
	}
	if got != n {
		j.e.t.Errorf("pesto: JSON %q length = %d, want %d", j.path, got, n)
	}
	return j.e
}

func (j *JSONAssertion) lookup() (any, bool) {
	root, ok := j.e.body()
	if !ok {
		return nil, false
	}
	return lookupPath(root, j.path)
}

// lookupPath walks a dotted path into a decoded JSON value.
func lookupPath(root any, path string) (any, bool) {
	if path == "" {
		return root, true
	}
	cur := root
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// lengthOf returns the length of an array, object, or string.
func lengthOf(v any) (int, bool) {
	switch node := v.(type) {
	case []any:
		return len(node), true
	case map[string]any:
		return len(node), true
	case string:
		return len(node), true
	default:
		return 0, false
	}
}

// normalize round-trips v through JSON so its types match a decoded body (numbers
// become float64, etc.), making DeepEqual comparisons reliable.
func normalize(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
