package contract

import (
	"math"
	"strings"
)

// violation is one negative case: a request body that breaks the schema in a
// single, named way.
type violation struct {
	name string // e.g. "required/name", "type/address.zip"
	body any
}

// unexpectedField is the key added to probe additionalProperties: false.
const unexpectedField = "pesto_unexpected_field"

// violations derives the negative cases from the schema, starting from the
// valid happy-path body: each mutates one field. A candidate the schema still
// accepts (e.g. a wrong type the schema also allows) is dropped, so every case
// returned is a real violation.
func (s *schema) violations(happy any) []violation {
	var out []violation
	seen := map[string]bool{}
	add := func(name string, p path, mutate func(parent map[string]any, key string) bool) {
		if seen[name] {
			return
		}
		body := clone(happy)
		if len(p) == 0 {
			// The body itself: mutate replaces it via a holder.
			holder := map[string]any{"": body}
			if !mutate(holder, "") {
				return
			}
			body = holder[""]
		} else {
			parent, ok := lookup(body, p[:len(p)-1]).(map[string]any)
			if !ok || !mutate(parent, p[len(p)-1]) {
				return
			}
		}
		if s.validate(body) == nil {
			return
		}
		seen[name] = true
		out = append(out, violation{name: name, body: body})
	}
	s.walk(s.root, happy, nil, add)
	return out
}

// walk derives violations for the node n whose valid value is v, then recurses
// into the object properties present in v.
func (s *schema) walk(n any, v any, p path, add func(string, path, func(map[string]any, string) bool)) {
	node := s.node(n)
	if node == nil {
		return
	}
	set := func(value any) func(map[string]any, string) bool {
		return func(parent map[string]any, key string) bool {
			parent[key] = value
			return true
		}
	}

	if wrong, ok := wrongType(node); ok {
		add("type/"+p.String(), p, set(wrong))
	}
	if enum, ok := node["enum"].([]any); ok && len(enum) > 0 {
		add("enum/"+p.String(), p, set(outsideEnum(enum)))
	}
	if f, ok := number(node["minimum"]); ok {
		add("minimum/"+p.String(), p, set(numeric(node, f-1)))
	}
	if f, ok := number(node["exclusiveMinimum"]); ok {
		add("minimum/"+p.String(), p, set(numeric(node, f)))
	}
	if f, ok := number(node["maximum"]); ok {
		add("maximum/"+p.String(), p, set(numeric(node, f+1)))
	}
	if f, ok := number(node["exclusiveMaximum"]); ok {
		add("maximum/"+p.String(), p, set(numeric(node, f)))
	}
	if f, ok := number(node["minLength"]); ok && f > 0 {
		add("min_length/"+p.String(), p, set(strings.Repeat("a", int(f)-1)))
	}
	if f, ok := number(node["maxLength"]); ok {
		add("max_length/"+p.String(), p, set(strings.Repeat("a", int(f)+1)))
	}

	obj, ok := v.(map[string]any)
	if !ok || !isObject(node) {
		return
	}
	if ap, ok := node["additionalProperties"].(bool); ok && !ap {
		add("additional_properties/"+p.String(), p, func(parent map[string]any, key string) bool {
			target, ok := parent[key].(map[string]any)
			if !ok {
				return false
			}
			target[unexpectedField] = "pesto"
			return true
		})
	}
	required := requiredSet(node)
	props, _ := node["properties"].(map[string]any)
	for _, name := range sortedKeys(obj) {
		child := p.child(name)
		if required[name] {
			add("required/"+child.String(), child, func(parent map[string]any, key string) bool {
				delete(parent, key)
				return true
			})
		}
		if sub, ok := props[name]; ok {
			s.walk(sub, obj[name], child, add)
		}
	}
}

// wrongType returns a value of a JSON type the node does not allow.
func wrongType(node map[string]any) (any, bool) {
	if len(types(node)) == 0 {
		return nil, false
	}
	candidates := []struct {
		typ   string
		value any
	}{
		{"string", "pesto-wrong-type"},
		{"number", 12345},
		{"boolean", true},
		{"object", map[string]any{}},
		{"array", []any{}},
	}
	for _, c := range candidates {
		if !hasType(node, c.typ) && !(c.typ == "number" && hasType(node, "integer")) {
			return c.value, true
		}
	}
	return nil, false
}

// outsideEnum returns a value that is not a member of enum and, where it can,
// has the members' type, so the case breaks only the enum: one above the
// largest member of a numeric enum, otherwise a string no member equals.
func outsideEnum(enum []any) any {
	largest := math.Inf(-1)
	for _, e := range enum {
		f, ok := e.(float64)
		if !ok {
			return "pesto-not-in-enum"
		}
		largest = max(largest, f)
	}
	return math.Floor(largest) + 1
}

// numeric renders f as an integer when the node is an integer type and f is
// whole, so the case breaks only the bound, not the type.
func numeric(node map[string]any, f float64) any {
	if hasType(node, "integer") && f == math.Trunc(f) {
		return int64(f)
	}
	return f
}

// number reads a JSON number keyword.
func number(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

// lookup returns the value at p inside v.
func lookup(v any, p path) any {
	for _, k := range p {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}
