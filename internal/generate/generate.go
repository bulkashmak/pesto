// Package generate scaffolds individual files inside an existing Pesto project
// (`pesto generate`): E2E tests, suite folders, typed client stubs, and contract
// bundles. It is
// CLI-only and never imported by the runtime library. The naming rules are
// specified in docs/design/active/design-cli.md.
package generate

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/bulkashmak/pesto/internal/transform"
)

// segmentRE is what every normalized name segment must match.
var segmentRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ErrInvalidName matches (via errors.Is) every error caused by a malformed name
// argument, so callers can report it as a usage mistake.
var ErrInvalidName = errors.New("invalid name")

// nameError is a malformed-name error that keeps its own message while still
// matching ErrInvalidName.
type nameError struct{ msg string }

func (e *nameError) Error() string { return e.msg }

func (e *nameError) Is(target error) bool { return target == ErrInvalidName }

func invalidName(format string, args ...any) error {
	return &nameError{msg: fmt.Sprintf(format, args...)}
}

// E2E scaffolds tests/e2e/<domain>/<name>_test.go for an argument of the form
// "<domain>/<name>". A domain that does not exist yet is created first, as if by
// [Suite]. It returns the paths of the files it created.
func E2E(root, arg string) ([]string, error) {
	segs := strings.Split(arg, "/")
	if len(segs) != 2 {
		return nil, invalidName("generate e2e needs exactly <domain>/<name>, got %q", arg)
	}
	domain, err := normalize(segs[0])
	if err != nil {
		return nil, err
	}
	name, err := normalize(segs[1])
	if err != nil {
		return nil, err
	}
	if err := requireProject(root); err != nil {
		return nil, err
	}

	var created []string
	domainDir := filepath.Join(root, "tests", "e2e", domain)
	if _, err := os.Stat(domainDir); os.IsNotExist(err) {
		suite, err := Suite(root, domain)
		if err != nil {
			return nil, err
		}
		created = append(created, suite...)
	}

	dest := filepath.Join(domainDir, name+"_test.go")
	if err := writeFile(dest, e2eTestTmpl, map[string]string{
		"Package":     domain,
		"Scenario":    name,
		"TestName":    "Test" + exportName(name),
		"RuntimePath": transform.RuntimeImportPath,
	}); err != nil {
		return nil, err
	}
	return append(created, dest), nil
}

// Suite scaffolds tests/e2e/<domain>/ with a suite_test.go that holds the package
// comment and the suite's shared helpers. It returns the paths it created.
func Suite(root, arg string) ([]string, error) {
	domain, err := normalize(arg)
	if err != nil {
		return nil, err
	}
	if err := requireProject(root); err != nil {
		return nil, err
	}

	dest := filepath.Join(root, "tests", "e2e", domain, "suite_test.go")
	if err := writeFile(dest, suiteTmpl, map[string]string{"Package": domain}); err != nil {
		return nil, err
	}
	return []string{dest}, nil
}

// Client scaffolds clients/<name>/<name>.go: an empty typed client stub wrapping
// *pesto.Client for the author to fill in. It returns the paths it created.
func Client(root, arg string) ([]string, error) {
	name, err := normalize(arg)
	if err != nil {
		return nil, err
	}
	if err := requireProject(root); err != nil {
		return nil, err
	}

	dest := filepath.Join(root, "clients", name, name+".go")
	if err := writeFile(dest, clientTmpl, map[string]string{
		"Package":     name,
		"RuntimePath": transform.RuntimeImportPath,
	}); err != nil {
		return nil, err
	}
	return []string{dest}, nil
}

// contractMethods are the HTTP methods a contract can target (the verbs the
// runtime client supports).
var contractMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE"}

// Contract scaffolds a contract bundle for one REST operation, e.g. method
// "POST" and path "/users/{id}/cards", in tests/contract/users/by_id/cards/post/:
// contract.yaml, the request (POST, PUT, PATCH) and response (all but DELETE)
// schemas, and a contract_test.go that runs them. It writes nothing if any of
// those files already exists, and returns the paths it created.
func Contract(root, method, apiPath string) ([]string, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	if !slices.Contains(contractMethods, method) {
		return nil, invalidName("invalid method %q: must be one of %s", method, strings.Join(contractMethods, ", "))
	}
	rel, params, err := contractDir(apiPath)
	if err != nil {
		return nil, err
	}
	if err := requireProject(root); err != nil {
		return nil, err
	}

	pkg := strings.ToLower(method)
	dir := filepath.Join(root, "tests", "contract", rel, pkg)
	data := map[string]string{
		"Package":      pkg,
		"Method":       method,
		"Path":         apiPath,
		"PathParams":   params,
		"RequestBody":  "",
		"ResponseBody": "response.schema.json",
		"Status":       "200",
		"RuntimePath":  transform.RuntimeImportPath,
	}
	files := []struct{ name, tmpl string }{
		{"contract.yaml", contractYAMLTmpl},
		{"contract_test.go", contractTestTmpl},
	}
	switch method {
	case "POST", "PUT", "PATCH":
		data["RequestBody"] = "request.schema.json"
		files = append(files, struct{ name, tmpl string }{"request.schema.json", requestSchemaTmpl})
	}
	switch method {
	case "POST":
		data["Status"] = "201"
	case "DELETE":
		data["Status"] = "204"
		data["ResponseBody"] = ""
	}
	if data["ResponseBody"] != "" {
		files = append(files, struct{ name, tmpl string }{"response.schema.json", responseSchemaTmpl})
	}

	// Check every destination first, so a clash leaves no half-written bundle.
	for _, f := range files {
		if dest := filepath.Join(dir, f.name); fileExists(dest) {
			return nil, fmt.Errorf("refusing to overwrite existing file: %s", dest)
		}
	}
	var created []string
	for _, f := range files {
		dest := filepath.Join(dir, f.name)
		if err := writeFile(dest, f.tmpl, data); err != nil {
			return created, err
		}
		created = append(created, dest)
	}
	return created, nil
}

// contractDir maps a REST path to its folder under tests/contract: each literal
// segment is normalized, and each {param} becomes by_<param>, so
// "/users/{id}/cards" -> "users/by_id/cards". It also returns the path_params
// block for contract.yaml, pointing each parameter at a data reference named
// after the segment before it ({id} after users -> users.default.id).
func contractDir(apiPath string) (string, string, error) {
	if !strings.HasPrefix(apiPath, "/") {
		return "", "", invalidName("invalid path %q: must start with /, e.g. /users/{id}", apiPath)
	}
	trimmed := strings.TrimPrefix(apiPath, "/")
	if trimmed == "" {
		return "", "", nil
	}

	var dirs []string
	var params strings.Builder
	prev := ""
	for _, seg := range strings.Split(trimmed, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			raw := seg[1 : len(seg)-1]
			name, err := normalize(raw)
			if err != nil {
				return "", "", invalidName("invalid path %q: parameter %s: %v", apiPath, seg, err)
			}
			file := prev
			if file == "" {
				file = "resources"
			}
			fmt.Fprintf(&params, "  %s: %s.default.%s\n", raw, file, name)
			dirs = append(dirs, "by_"+name)
			continue
		}
		if seg == "" || strings.ContainsAny(seg, "{}?#") {
			return "", "", invalidName("invalid path %q: segment %q must be a name or a {param}", apiPath, seg)
		}
		name, err := normalize(seg)
		if err != nil {
			return "", "", invalidName("invalid path %q: %v", apiPath, err)
		}
		dirs = append(dirs, name)
		prev = name
	}
	return filepath.Join(dirs...), strings.TrimSuffix(params.String(), "\n"), nil
}

// fileExists reports whether path exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// normalize maps a user-supplied name segment to its canonical form: lowercase,
// with hyphens and spaces as underscores. Anything that does not then match
// [a-z][a-z0-9_]* is an error, so generated identifiers are always valid Go.
func normalize(seg string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(seg))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if !segmentRE.MatchString(s) {
		return "", invalidName("invalid name %q: %q must match [a-z][a-z0-9_]*", seg, s)
	}
	return s, nil
}

// exportName turns a normalized segment into an exported Go name:
// "happy_path" -> "HappyPath".
func exportName(s string) string {
	var b strings.Builder
	for _, part := range strings.Split(s, "_") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]))
		b.WriteString(part[1:])
	}
	return b.String()
}

// requireProject checks that root looks like a project generate can add to.
func requireProject(root string) error {
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return fmt.Errorf("%s is not a project root (no go.mod); run `pesto init` first or pass --root", root)
	}
	return nil
}

// writeFile renders tmpl with data into dest, refusing to overwrite.
func writeFile(dest, tmpl string, data map[string]string) error {
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("refusing to overwrite existing file: %s", dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	t, err := template.New("f").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("render %s: %w", dest, err)
	}
	return os.WriteFile(dest, buf.Bytes(), 0o644)
}

const e2eTestTmpl = `package {{.Package}}

import (
	"{{.RuntimePath}}"
)

// {{.TestName}} covers the {{.Scenario}} scenario of the {{.Package}} domain.
//
// Run it with:  pesto run -run {{.TestName}}
func {{.TestName}}(t *pesto.T) {
	// TODO: point the request at the endpoint under test and assert on the
	// response.
	resp := t.Client.GET("/").Send()

	resp.Expect().Status(200)
}
`

const suiteTmpl = `// Package {{.Package}} holds the {{.Package}} E2E suite. Shared helpers and
// setup used by its tests live in this file, so individual test files stay
// focused on their scenario.
package {{.Package}}
`

const clientTmpl = `// Package {{.Package}} provides a typed client for the {{.Package}} API, built
// on the Pesto HTTP client.
package {{.Package}}

import (
	"{{.RuntimePath}}"
)

// Client adds typed helpers for the {{.Package}} API on top of [pesto.Client].
type Client struct {
	*pesto.Client
}

// New wraps a test's client, e.g. New(t.Client).
func New(c *pesto.Client) *Client { return &Client{Client: c} }

// TODO: add one method per operation, for example:
//
//	func (c *Client) GetByID(id string) *pesto.Response {
//		return c.GET("/{{.Package}}/" + id).Send()
//	}
`

const contractYAMLTmpl = `# Contract for {{.Method}} {{.Path}}, run by contract_test.go.
{{- if .RequestBody}}
# Pesto builds a valid request from the request schema's examples, then one
# request per schema violation. See docs/reference/contract.md.
{{- else}}
# Pesto sends the request and checks the response. See docs/reference/contract.md.
{{- end}}
method: {{.Method}}
path: {{.Path}}
{{- if .PathParams}}
# Each path parameter is filled from the environment's static data:
# users.default.id reads field id of record default in
# environments/<env>/data/users.yaml.
path_params:
{{.PathParams}}
{{- end}}
{{- if .RequestBody}}
request:
  body: {{.RequestBody}}
  # Body fields that must hold data existing in the environment can read it
  # the same way, converted to the field's schema type:
  # data:
  #   account_id: accounts.default.id
{{- end}}
response:
  success:
    status: {{.Status}}
{{- if .ResponseBody}}
    body: {{.ResponseBody}}
{{- end}}
{{- if .RequestBody}}
  # The status the API returns for a request that breaks the request schema.
  on_violation:
    status: 422
{{- end}}
`

const contractTestTmpl = `// Package {{.Package}} holds the contract test for {{.Method}} {{.Path}}.
package {{.Package}}

import (
	"{{.RuntimePath}}"
	"{{.RuntimePath}}/contract"
)

// TestContract checks {{.Method}} {{.Path}} against contract.yaml and its
// schemas: a happy path, plus one case per violation of the request schema.
func TestContract(t *pesto.T) {
	contract.Run(t, "contract.yaml")
}
`

const requestSchemaTmpl = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name"],
  "additionalProperties": false,
  "properties": {
    "name": { "type": "string", "minLength": 1, "examples": ["Bob"] }
  }
}
`

const responseSchemaTmpl = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object"
}
`
