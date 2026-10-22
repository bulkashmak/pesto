# Changelog

All notable changes to Pesto. The CLI (`cmd/pesto`) and the runtime library
(`github.com/bulkashmak/pesto`) are released together under one version.

## v0.2.0 - unreleased

REST contract tests: describe an endpoint with JSON Schema and let Pesto derive
the valid request and one negative case per schema violation. Static test data:
records that exist in each environment, read by tests and contracts.

### Runtime library

- New `contract` package: `contract.Run(t, "contract.yaml")` runs a contract's
  `happy_path` and `violation/<kind>/<field>` cases. Adds the
  `santhosh-tekuri/jsonschema/v6` dependency, compiled only by projects that
  import `contract`.
- `T.Run(name, func(*pesto.T))`: subtests that keep the client configuration.
  **Changed:** this shadows `testing.T.Run`; a subtest written as
  `t.Run(name, func(t *testing.T) {...})` must become `t.T.Run(...)`.
- Static test data: an environment can be a folder,
  `environments/<env>/env.yaml` plus `environments/<env>/data/*.yaml` of named
  records. `T.Data("users.default.id")` reads a value (fatal when missing),
  `T.LookupData` is the comma-ok form, and `WithData` sets data in code. The
  flat `environments/<env>.yaml` still works, as an environment with no data.
- **Removed:** the `variables` environment key (it was parsed in v0.1.0 but
  nothing could read it; an old file still loads). Contract `path_params` now
  name data references.
- Contract `request.data`: fills request body fields from static data
  (`account_id: accounts.default.id`), converted to each field's schema type.
  Violations are derived from the filled body.
- `Client.Do(method, path)` starts a request with any method.

### CLI

- `pesto generate contract <METHOD> <path>`: scaffolds `contract.yaml`, request
  and response schemas, and `contract_test.go` under
  `tests/contract/<path>/<method>/` (`{param}` → `by_<param>`).
  Each `{param}` gets a data reference in `path_params` (`{id}` after `users`
  → `users.default.id`).
- `pesto init` scaffolds environments as folders (`environments/dev/env.yaml`,
  `environments/dev/data/todos.yaml`), and the starter test reads its todo id
  with `t.Data`.

## v0.1.0 - 2026-10-20

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
