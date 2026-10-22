package contract

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// example builds a valid value for the schema node n at JSON path p from the
// schema's own values, in priority order: const, examples[0], default, enum[0].
// An object without one is built from its properties: required ones must
// produce a value, optional ones are included only when they can. Nothing is
// ever invented, so a required field with no value source is an error naming it.
//
// Fields named in slots (dotted paths) are filled from static data later; they
// get a [dataSlot] placeholder instead, so they need no value of their own.
func (s *schema) example(n any, p path, slots map[string]bool) (any, error) {
	if slots[p.String()] && len(p) > 0 {
		return dataSlot{}, nil
	}
	node := s.node(n)
	if node == nil {
		return nil, fmt.Errorf("%s: no example value: add \"examples\" (or const, default, enum) to its schema", p)
	}
	if v, ok := node["const"]; ok {
		return clone(v), nil
	}
	if ex, ok := node["examples"].([]any); ok && len(ex) > 0 {
		return clone(ex[0]), nil
	}
	if v, ok := node["default"]; ok {
		return clone(v), nil
	}
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		return clone(enum[0]), nil
	}

	switch {
	case isObject(node):
		props, _ := node["properties"].(map[string]any)
		required := requiredSet(node)
		obj := map[string]any{}
		for _, name := range sortedKeys(props) {
			v, err := s.example(props[name], p.child(name), slots)
			if err != nil {
				if required[name] {
					return nil, err
				}
				continue
			}
			obj[name] = v
		}
		for _, name := range sortedKeys(required) {
			if _, ok := props[name]; !ok {
				return nil, fmt.Errorf("%s: required but not described under properties", p.child(name))
			}
		}
		return obj, nil
	case hasType(node, "array"):
		item, err := s.example(node["items"], p.child("0"), slots)
		if err != nil {
			return nil, fmt.Errorf("%s: no example value: add \"examples\" to the array or its items", p)
		}
		return []any{item}, nil
	}
	return nil, fmt.Errorf("%s: no example value: add \"examples\" (or const, default, enum) to its schema", p)
}

// dataSlot marks a body field that request.data fills from static data.
type dataSlot struct{}

// field returns the schema node describing the body field at p, following
// nested objects' properties and local $refs. It is nil for a field whose
// schema says nothing about it (a boolean schema).
func (s *schema) field(p path) (map[string]any, error) {
	cur := s.root
	for i, key := range p {
		node := s.node(cur)
		if node == nil || !isObject(node) {
			return nil, fmt.Errorf("%s is not an object in %s", path(p[:i]), s.file)
		}
		props, _ := node["properties"].(map[string]any)
		next, ok := props[key]
		if !ok {
			return nil, fmt.Errorf("%s declares no property %s", s.file, path(p[:i+1]))
		}
		cur = next
	}
	node := s.node(cur)
	if node != nil && (isObject(node) || hasType(node, "array")) {
		return nil, fmt.Errorf("%s is not a single value in %s: name one of its fields", p, s.file)
	}
	return node, nil
}

// convert turns a static data value (always text) into the JSON type the
// field's schema declares: integer, number, boolean, or string - also when the
// schema declares no type or lists string among several.
func convert(value string, node map[string]any) (any, error) {
	ts := types(node)
	if len(ts) == 0 || slices.Contains(ts, "string") {
		return value, nil
	}
	for _, t := range ts {
		switch t {
		case "integer":
			if n, err := strconv.ParseInt(value, 10, 64); err == nil {
				return n, nil
			}
		case "number":
			if f, err := strconv.ParseFloat(value, 64); err == nil {
				return f, nil
			}
		case "boolean":
			if b, err := strconv.ParseBool(value); err == nil {
				return b, nil
			}
		}
	}
	return nil, fmt.Errorf("%q is not a valid %s", value, strings.Join(ts, " or "))
}

// setPath sets the field at p in body to v, creating parent objects that are
// missing so the field has somewhere to live.
func setPath(body any, p path, v any) error {
	obj, ok := body.(map[string]any)
	if !ok {
		return fmt.Errorf("the request body is not an object")
	}
	for i, key := range p[:len(p)-1] {
		next, ok := obj[key].(map[string]any)
		if !ok {
			if _, present := obj[key]; present {
				return fmt.Errorf("%s is not an object in the built body", path(p[:i+1]))
			}
			next = map[string]any{}
			obj[key] = next
		}
		obj = next
	}
	obj[p[len(p)-1]] = v
	return nil
}

// requiredSet returns the node's "required" keyword as a set.
func requiredSet(n map[string]any) map[string]bool {
	set := map[string]bool{}
	if req, ok := n["required"].([]any); ok {
		for _, r := range req {
			if name, ok := r.(string); ok {
				set[name] = true
			}
		}
	}
	return set
}

// sortedKeys returns m's keys in order, so derived cases are deterministic.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// clone deep-copies a JSON value so mutating a case never touches another.
func clone(v any) any {
	switch v := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, e := range v {
			out[k] = clone(e)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = clone(e)
		}
		return out
	}
	return v
}

// path is a location in a JSON body, as a list of object keys.
type path []string

func (p path) child(key string) path {
	return append(p[:len(p):len(p)], key)
}

// String renders the path in the dotted form [pesto.Expectation.JSON] uses;
// the body itself is "body".
func (p path) String() string {
	if len(p) == 0 {
		return "body"
	}
	s := p[0]
	for _, k := range p[1:] {
		s += "." + k
	}
	return s
}
