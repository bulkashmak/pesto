# CLI reference

The `pesto` command. This documents what is implemented today; commands marked
_planned_ are on the [roadmap](../design/roadmap.md).

```
pesto init <name> [--module <path>]
pesto run [flags] [go test args...]
pesto version
pesto generate ...        # planned (Phase 3+)
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

## `pesto generate ...` (planned)

Django-style scaffolding of tests, clients, and suites (`generate e2e`,
`generate contract`, `generate client`, `generate suite`). Designed in
[design-cli.md](../design/active/design-cli.md); not yet implemented.
