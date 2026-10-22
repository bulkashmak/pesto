package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schema is a loaded JSON Schema file: the raw document, walked to build values
// and derive violations, and its compiled form, used to validate.
type schema struct {
	file     string
	root     any
	compiled *jsonschema.Schema
}

// loadSchema reads and compiles the JSON Schema at path. The default draft is
// 2020-12 and format keywords are asserted.
func loadSchema(path string) (*schema, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	loc := (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.AssertFormat()
	if err := c.AddResource(loc, doc); err != nil {
		return nil, fmt.Errorf("load %s: %w", path, err)
	}
	compiled, err := c.Compile(loc)
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", path, err)
	}
	return &schema{file: path, root: root, compiled: compiled}, nil
}

// validate checks v (plain Go JSON values) against the schema.
func (s *schema) validate(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.validateJSON(data)
}

// validateJSON checks a raw JSON document against the schema.
func (s *schema) validateJSON(data []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("body is not JSON: %w", err)
	}
	return s.compiled.Validate(inst)
}

// node returns n as a schema object with any local $ref ("#/...") followed. A
// boolean schema or anything unrecognized comes back nil.
func (s *schema) node(n any) map[string]any {
	for range 32 { // bound the chain so a $ref cycle cannot loop forever
		m, ok := n.(map[string]any)
		if !ok {
			return nil
		}
		ref, ok := m["$ref"].(string)
		if !ok || !strings.HasPrefix(ref, "#") {
			return m
		}
		n = s.pointer(strings.TrimPrefix(ref, "#"))
	}
	return nil
}

// pointer resolves a JSON pointer within the schema document.
func (s *schema) pointer(ptr string) any {
	cur := s.root
	if ptr == "" {
		return cur
	}
	for _, tok := range strings.Split(strings.TrimPrefix(ptr, "/"), "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[tok]
	}
	return cur
}

// types returns the node's "type" keyword as a list.
func types(n map[string]any) []string {
	switch t := n["type"].(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, v := range t {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// hasType reports whether the node declares type t ("number" also admits
// "integer" nodes, since an integer is a number).
func hasType(n map[string]any, t string) bool {
	for _, have := range types(n) {
		if have == t || (t == "integer" && have == "number") {
			return true
		}
	}
	return false
}

// isObject reports whether the node describes an object.
func isObject(n map[string]any) bool {
	if hasType(n, "object") {
		return true
	}
	_, ok := n["properties"]
	return ok && len(types(n)) == 0
}
