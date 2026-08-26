// Package generate scaffolds individual files inside an existing Pesto project
// (`pesto generate`): E2E tests, suite folders, and typed client stubs. It is
// CLI-only and never imported by the runtime library. The naming rules are
// specified in docs/design/active/design-cli.md.
package generate

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/bulkashmak/pesto/internal/transform"
)

// segmentRE is what every normalized name segment must match.
var segmentRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// E2E scaffolds tests/e2e/<domain>/<name>_test.go for an argument of the form
// "<domain>/<name>". A domain that does not exist yet is created first, as if by
// [Suite]. It returns the paths of the files it created.
func E2E(root, arg string) ([]string, error) {
	segs := strings.Split(arg, "/")
	if len(segs) != 2 {
		return nil, fmt.Errorf("generate e2e needs exactly <domain>/<name>, got %q", arg)
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

// normalize maps a user-supplied name segment to its canonical form: lowercase,
// with hyphens and spaces as underscores. Anything that does not then match
// [a-z][a-z0-9_]* is an error, so generated identifiers are always valid Go.
func normalize(seg string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(seg))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if !segmentRE.MatchString(s) {
		return "", fmt.Errorf("invalid name %q: %q must match [a-z][a-z0-9_]*", seg, s)
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
