package pesto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// dataDirName is the folder inside an environment folder that holds its data:
// environments/<env>/data/<file>.yaml.
const dataDirName = "data"

// dataSet is the static test data a [T] reads with [T.Data]: one tree per data
// file, keyed by the file's base name. A file maps record names to records; a
// record maps field names to scalar values or nested maps of fields.
type dataSet struct {
	dir   string         // where the files came from, for messages; "" when set in code
	files map[string]any // file name -> map[string]any of records
}

// lookup resolves a reference "<file>.<record>.<field>[.<nested>...]" to its
// value, formatted as a string. The error says which part is missing and where.
func (d dataSet) lookup(ref string) (string, error) {
	parts := strings.Split(ref, ".")
	if len(parts) < 3 || slices.Contains(parts, "") {
		return "", fmt.Errorf("invalid reference %q: want <file>.<record>.<field>, e.g. users.default.id", ref)
	}

	file, ok := d.files[parts[0]]
	if !ok {
		if d.dir == "" {
			return "", fmt.Errorf("no data file %q", parts[0])
		}
		return "", fmt.Errorf("no file %s", filepath.Join(d.dir, parts[0]+".yaml"))
	}

	where := d.fileName(parts[0])
	node := file
	for i, key := range parts[1:] {
		m, ok := asMap(node)
		if !ok {
			return "", fmt.Errorf("%s in %s is a value, not a map of fields", strings.Join(parts[1:i+1], "."), where)
		}
		next, ok := m[key]
		if !ok {
			if i == 0 {
				return "", fmt.Errorf("no record %q in %s", key, where)
			}
			return "", fmt.Errorf("no field %q in %s", strings.Join(parts[1:i+2], "."), where)
		}
		node = next
	}

	if _, ok := asMap(node); ok {
		return "", fmt.Errorf("%s in %s is a map of fields, not a value", strings.Join(parts[1:], "."), where)
	}
	switch v := node.(type) {
	case nil:
		return "", fmt.Errorf("%s in %s has no value", strings.Join(parts[1:], "."), where)
	case string:
		return v, nil
	default:
		return fmt.Sprint(v), nil
	}
}

// fileName names data file name for messages.
func (d dataSet) fileName(name string) string {
	if d.dir == "" {
		return "data file " + name
	}
	return filepath.Join(d.dir, name+".yaml")
}

// asMap returns v as a map of fields, accepting the map types a test is likely
// to pass to [WithData].
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[string]string:
		out := make(map[string]any, len(m))
		for k, s := range m {
			out[k] = s
		}
		return out, true
	case map[string]map[string]any:
		out := make(map[string]any, len(m))
		for k, r := range m {
			out[k] = r
		}
		return out, true
	}
	return nil, false
}

// loadData reads every <name>.yaml in dir. A missing dir is no data, not an
// error. display is the dir as shown in messages (relative to the project).
func loadData(dir, display string) (dataSet, error) {
	d := dataSet{dir: display, files: map[string]any{}}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return d, nil
	}
	if err != nil {
		return d, fmt.Errorf("read %s: %w", display, err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		shown := filepath.Join(display, name)
		if strings.Contains(strings.TrimSuffix(name, ".yaml"), ".") {
			return d, fmt.Errorf("%s: a data file name cannot contain a dot (it separates the parts of a reference)", shown)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return d, fmt.Errorf("read %s: %w", shown, err)
		}
		records, err := parseDataFile(b)
		if err != nil {
			return d, fmt.Errorf("%s: %w", shown, err)
		}
		d.files[strings.TrimSuffix(name, ".yaml")] = records
	}
	return d, nil
}

// parseDataFile parses one data file: a map of named records, each a map of
// fields. Scalar values keep their YAML text and get ${VAR} expansion.
func parseDataFile(b []byte) (map[string]any, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, err
	}
	records := map[string]any{}
	if doc.Kind == 0 { // empty file
		return records, nil
	}
	root := deref(doc.Content[0])
	if root.Kind != yaml.MappingNode {
		return nil, errors.New("want a map of named records at the top level")
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		name, rec := root.Content[i].Value, deref(root.Content[i+1])
		if strings.Contains(name, ".") {
			return nil, fmt.Errorf("record %q (line %d): a record name cannot contain a dot", name, root.Content[i].Line)
		}
		if rec.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("record %q (line %d) must be a map of fields", name, rec.Line)
		}
		fields, err := dataFields(rec, name)
		if err != nil {
			return nil, err
		}
		records[name] = fields
	}
	return records, nil
}

// dataFields converts a mapping node of fields; path names it for messages.
func dataFields(n *yaml.Node, path string) (map[string]any, error) {
	fields := make(map[string]any, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		if strings.Contains(key, ".") {
			return nil, fmt.Errorf("%s.%s (line %d): a field name cannot contain a dot", path, key, n.Content[i].Line)
		}
		v := deref(n.Content[i+1])
		switch v.Kind {
		case yaml.ScalarNode:
			if v.Tag == "!!null" {
				fields[key] = nil
			} else {
				fields[key] = os.ExpandEnv(v.Value)
			}
		case yaml.MappingNode:
			nested, err := dataFields(v, path+"."+key)
			if err != nil {
				return nil, err
			}
			fields[key] = nested
		default:
			return nil, fmt.Errorf("%s.%s (line %d): lists are not supported; use a map of fields", path, key, v.Line)
		}
	}
	return fields, nil
}

// deref follows YAML aliases to the node they point at.
func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}
