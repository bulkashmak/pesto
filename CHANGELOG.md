# Changelog

All notable changes to Pesto. The CLI (`cmd/pesto`) and the runtime library
(`github.com/bulkashmak/pesto`) are released together under one version.

## v0.1.0 - unreleased

First release: write, scaffold, and run black-box API tests.

### Runtime library

- `pesto.T`: the test context, embedding `*testing.T`, with a configured `Client`.
- `Client`: fluent HTTP builder (GET/POST/PUT/PATCH/DELETE, JSON bodies, headers).
- `Expect`: soft-by-default status, header, and JSON-path assertions;
  `Decode` as an escape hatch to plain structs.
- Configuration: `pesto.yaml` + `environments/*.yaml` with `${VAR}` expansion.
- Auth: bearer and basic.

### CLI

- `pesto init <name>`: scaffolds a complete project with a runnable example;
  the generated `go.mod` pins the runtime to the CLI's version.
- `pesto run`: transforms `func TestX(t *pesto.T)` tests in memory (no files are
  modified) and runs them with `go test`; `--env`, `--report junit`, `--out`,
  and any `go test` flags. Works from any folder inside the project; warns when
  the CLI and the project's runtime versions differ.
- `pesto generate e2e|suite|client`: scaffolds tests, suites, and client stubs.
- `pesto version`, `pesto help`.

### Requirements

- Go 1.24 or newer.
