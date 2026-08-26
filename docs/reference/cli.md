# CLI reference

The `pesto` command. This documents what is implemented today; commands marked
_planned_ are on the [roadmap](../design/roadmap.md).

```
pesto init <name> [--module <path>]
pesto generate <e2e|suite|client> <name> [--root <dir>]
pesto run [flags] [go test args...]
pesto version
pesto generate contract ...        # planned (Phase 4)
```

## `pesto init <name>`

Scaffolds a new project directory named `<name>` with the full Pesto layout
(`pesto.yaml`, `environments/`, `tests/e2e/example/`, `clients/`, `fixtures/`,
`helpers/`, `reports/`, `go.mod`, `.gitignore`, `README.md`). It refuses to
overwrite existing files.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--module <path>` | the project dir name | Go module path written to `go.mod` |

After init, resolve the runtime dependency with
`go get github.com/bulkashmak/pesto@latest`.

## `pesto run [flags] [go test args...]`

Transforms authored `func TestX(t *pesto.T)` tests into standard Go tests (via
`go test -overlay`, without touching your files) and runs them. Any argument pesto
does not recognize is passed through to `go test`, so `-run`, `-v`, `-count`, `-p`,
package patterns, etc. all work.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--env <name>` | `default_env` from `pesto.yaml` | environment to run against |
| `--root <dir>` | `.` | project root (module directory) |
| `--report <format>` | none | write a report; `junit` is supported |
| `--out <path>` | `<root>/reports/junit.xml` | report output path |
| `--keep` | off | keep the generated overlay temp dir for inspection |

Examples:

```bash
pesto run                                  # default environment, all tests
pesto run --env staging -run TestCreate -v # a subset, verbose
pesto run --report junit                   # + reports/junit.xml
pesto run --report junit --out out/ci.xml  # custom report path
```

Exit code is `go test`'s: `0` on success, non-zero on any test failure - suitable
for CI.

Notes:
- Requesting a report runs `go test -json` internally but still reproduces normal
  console output.
- Bare `go test` will not run a Pesto project; `pesto run` is the entry point.

## `pesto version`

Prints the CLI version.

## `pesto generate <kind> <name>`

Scaffolds one file (from a built-in template) inside an existing project. Every
name segment is normalized - lowercased, hyphens and spaces become underscores -
and must then match `[a-z][a-z0-9_]*`; anything else is an error. Existing files
are never overwritten.

| Command | Creates |
| --- | --- |
| `generate e2e <domain>/<name>` | `tests/e2e/<domain>/<name>_test.go` with `func Test<Name>(t *pesto.T)` |
| `generate suite <domain>` | `tests/e2e/<domain>/suite_test.go` for the domain's shared helpers |
| `generate client <name>` | `clients/<name>/<name>.go`, a typed stub wrapping `*pesto.Client` |

`generate e2e` into a domain that does not exist yet creates the suite first, so
one command is enough for a new domain.

| Flag | Default | Meaning |
| --- | --- | --- |
| `--root <dir>` | `.` | project root (must contain `go.mod`) |

Examples:

```bash
pesto generate e2e transfer/happy-path   # tests/e2e/transfer/happy_path_test.go
pesto generate suite billing             # tests/e2e/billing/suite_test.go
pesto generate client users              # clients/users/users.go
```

`generate contract` (schema-driven contract tests) is planned for Phase 4 - see
the [roadmap](../design/roadmap.md).
