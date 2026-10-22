// Package scaffold creates the on-disk layout for a new Pesto project
// (`pesto init`). It is CLI-only and never imported by the runtime library.
package scaffold

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"

	"github.com/bulkashmak/pesto/internal/transform"
)

// RuntimeImportPath returns the module path of the Pesto runtime library, for use
// in CLI messages.
func RuntimeImportPath() string { return transform.RuntimeImportPath }

// Options controls project creation.
type Options struct {
	// Dir is the project directory to create (also the project root).
	Dir string
	// Module is the Go module path written to go.mod.
	Module string
	// PestoVersion is the version required for the runtime library in go.mod.
	PestoVersion string
}

// file is one templated file in the scaffold.
type file struct {
	path string // relative to the project dir
	tmpl string
}

// files is the complete project layout produced by `pesto init`. Directories that
// should exist but start empty are seeded with a .gitkeep.
var files = []file{
	{"go.mod", goModTmpl},
	{"pesto.yaml", pestoYAMLTmpl},
	{".gitignore", gitignoreTmpl},
	{"README.md", readmeTmpl},
	{"environments/dev/env.yaml", devEnvTmpl},
	{"environments/dev/data/todos.yaml", devTodosTmpl},
	{"environments/test/env.yaml", testEnvTmpl},
	{"environments/test/data/todos.yaml", testTodosTmpl},
	{"tests/e2e/example/example_test.go", exampleTestTmpl},
	{"clients/.gitkeep", ""},
	{"fixtures/.gitkeep", ""},
	{"helpers/.gitkeep", ""},
	{"reports/.gitkeep", ""},
}

// Create writes the project layout. It refuses to overwrite an existing file so a
// mistaken init cannot clobber work.
func Create(opts Options) error {
	if opts.Module == "" {
		return fmt.Errorf("module path is required")
	}
	data := map[string]string{
		"Module":       opts.Module,
		"PestoVersion": opts.PestoVersion,
		"RuntimePath":  transform.RuntimeImportPath,
	}

	for _, f := range files {
		dest := filepath.Join(opts.Dir, filepath.FromSlash(f.path))
		if _, err := os.Stat(dest); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s", dest)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		content, err := render(f.tmpl, data)
		if err != nil {
			return fmt.Errorf("render %s: %w", f.path, err)
		}
		if err := os.WriteFile(dest, content, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func render(tmpl string, data map[string]string) ([]byte, error) {
	t, err := template.New("f").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

const goModTmpl = `module {{.Module}}

go 1.24

require {{.RuntimePath}} {{.PestoVersion}}
`

const pestoYAMLTmpl = `# Pesto project configuration.

# The environment used by ` + "`pesto run`" + ` when --env is not given.
default_env: dev

# Each environment is a folder: environments/<name>/env.yaml for where and how
# to connect, environments/<name>/data/ for the static test data that exists there.
`

const gitignoreTmpl = `# Generated test reports
/reports/

# Go test/build artifacts
*.test
*.out
`

const readmeTmpl = `# {{.Module}}

A [Pesto](https://github.com/bulkashmak/pesto) black-box API test project.

## Layout

    environments/   per environment (dev, test, ...): env.yaml (base URL + auth)
                    and data/ (static test data that exists there)
    tests/e2e/      hand-written end-to-end tests, by domain
    clients/        typed API clients
    fixtures/       reusable test data + setup/teardown
    helpers/        shared helpers
    reports/        generated reports (gitignored)

## Running

    pesto run                 # default environment (see pesto.yaml)
    pesto run --env test      # a specific environment
    pesto run -run TestGetTodo -v

Tests are authored with the ` + "`*pesto.T`" + ` signature and transformed by
` + "`pesto run`" + `; plain ` + "`go test`" + ` does not run a Pesto project.
`

const devEnvTmpl = `# Development environment.
# base_url is where t.Client sends requests; test paths are relative to it.
base_url: https://jsonplaceholder.typicode.com

# Authentication (uncomment and configure as needed):
# auth:
#   type: bearer
#   token: ${API_TOKEN}   # ${VAR} is expanded from the OS environment at run time
`

const testEnvTmpl = `# Test / staging environment. Point base_url at your own API.
base_url: https://jsonplaceholder.typicode.com
`

// devTodosTmpl and testTodosTmpl are static test data: records that exist
// permanently in each environment, read in tests with t.Data.
const devTodosTmpl = `# Static test data: todos that exist permanently in the dev environment.
# Tests read it with t.Data("todos.default.id"): file todos.yaml, record
# default, field id. Values support ${VAR} expansion, like env.yaml.
default:
  id: 1
`

const testTodosTmpl = `# Static test data: todos that exist permanently in the test environment.
# Each environment lists its own records - ids usually differ between them.
default:
  id: 1
`

const exampleTestTmpl = `package example

import (
	"{{.RuntimePath}}"
)

// TestGetTodo is a starter Pesto test. It runs against the base_url configured in
// environments/<env>/env.yaml (dev by default; see pesto.yaml) and reads a todo
// that exists there from environments/<env>/data/todos.yaml. Replace it with
// tests against your own API.
//
// Run it with:  pesto run
func TestGetTodo(t *pesto.T) {
	id := t.Data("todos.default.id")

	resp := t.Client.GET("/todos/" + id).Send()

	resp.Expect().
		Status(200).
		JSON("id").Exists().
		JSON("title").Exists()
}
`
