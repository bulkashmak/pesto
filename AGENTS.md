# AGENTS.md

Baseline context for AI coding agents working in this repository.

## What is this repo

Pesto: a structured, fast black-box backend test-automation framework for QA
engineers, built in Go. A Pesto test is an ordinary Go test authored with a clean
`func TestX(t *pesto.T)` signature; the `pesto` CLI transforms those into standard
`*testing.T` tests and drives `go test`. Pesto adds structure, a typed HTTP client,
fluent assertions, environments, and reporting on top of Go's `testing` engine - it
does not replace the test runner.

## Status

Under active construction. **Implemented (Phases 1-3):** the runtime library
(`pesto.T`, `Client`, `Expect`/`Decode`, config, auth) and the CLI (`pesto init`,
`pesto run` with the transform + JUnit reports, `pesto generate e2e|suite|client`,
`pesto version`). **Next:** Phase 4 contract-level tests, then fixtures. The source of truth for
progress is [docs/design/roadmap.md](docs/design/roadmap.md).

## Module & toolchain

- Module path: `github.com/bulkashmak/pesto`
- Minimum Go: 1.22 (relies on per-iteration loop variables for parallel subtests)
- Dependencies: only `gopkg.in/yaml.v3` (config parsing). Keep the dependency
  footprint minimal - especially for the runtime library.

## Architecture: two halves, one module

The single most important structural rule:

- **The runtime library is the module root** (`pesto.go`, `client.go`,
  `response.go`, `expect.go`, `auth.go`, `config.go`, `run.go`). It is a plain
  importable library (`import "github.com/bulkashmak/pesto"`), **not** installable,
  and imports nothing from `cmd/` or `internal/`. A test project that imports it
  must not pull in CLI code. Keep this root small and public.
- **The CLI is `cmd/pesto`** (`package main`), installable via
  `go install github.com/bulkashmak/pesto/cmd/pesto@latest`. `main.go` only wires:
  it builds the `cli.State`, registers every command in the registry, and maps
  the returned error to an exit code.
- **CLI implementation lives under `internal/`** (compiler-private). The command
  layer is `internal/cli` (`State`), `internal/cli/commands` (the registry) and
  `internal/cli/handlers` (one `handler_<name>.go` per command). Handlers stay
  thin and call the engines: `transform`, `runner`, `report`, `scaffold`,
  `generate`. All new CLI-only logic goes here, never in the root.

Adding a command: write `internal/cli/handlers/handler_<name>.go` with
`func Handle<Name>(s *cli.State, cmd commands.Command) error`, then register it
in `cmd/pesto/main.go` with its usage and description (help is generated from
the registry). Write output to `s.Stdout`/`s.Stderr`, never `os.Stdout`, and get
the working directory from `s.Dir()`, never `os.Getwd`; return
`commands.Usagef(...)` for bad invocations (exit 2) and
`&commands.ExitError{Code: n}` to propagate a specific exit code.

Dependency arrow points one way only: `cmd/pesto → root + internal/*`; the root
depends on neither. Preserve this when adding code.

### Package map

| Path | Role |
| --- | --- |
| `.` (`package pesto`) | runtime library: `T`, `New`, `Run`, `Client`, `Response`, `Expect`, `Decode`, `Config`/`Environment`, `Auth` |
| `cmd/pesto` | CLI entry point: wires `State` + registry, maps errors to exit codes |
| `internal/cli` | `State` shared by every command handler (version, stdout/stderr) |
| `internal/cli/commands` | command registry (`Register`/`Run`), `UsageError`, `ExitError` |
| `internal/cli/handlers` | one handler per command: `init`, `generate`, `run`, `version`, `help` |
| `internal/project` | project root discovery (walk up to `pesto.yaml`, else `go.mod`) for `run`/`generate` |
| `internal/transform` | AST rewrite: `func TestX(t *pesto.T)` → body rename + generated `*testing.T` wrappers |
| `internal/runner` | builds the `go test -overlay` map and drives `go test` (incl. `--report`) |
| `internal/report` | `go test -json` → JUnit XML |
| `internal/scaffold` | `pesto init` project generator (templated layout) |
| `internal/generate` | `pesto generate e2e|suite|client` file scaffolding (templates + name rules) |

## The transform (why `pesto run` is required)

Go's runner only discovers `func TestXxx(t *testing.T)`, so a `*pesto.T` test is not
runnable by bare `go test`. `pesto run` rewrites tests **in memory** and passes them
via `go test -overlay` - it never edits files on disk:

1. Each authored `_test.go` with Pesto tests is overlaid with a rewritten copy where
   `func TestX(t *pesto.T)` is renamed (byte-precise) to `pestoBody_TestX`, with a
   `//line` directive so diagnostics point at the authored source.
2. A generated `pesto_generated_test.go` is injected into the package (a virtual
   overlay file) with `func TestX(t *testing.T) { pesto.Run(t, pestoBody_TestX) }`.

`pesto.Run` builds the `*pesto.T`, resolving the selected environment once per binary
from the `PESTO_ENV` / `PESTO_ROOT` process variables that `pesto run` sets. Full
rationale: [docs/design/active/design-test-context.md](docs/design/active/design-test-context.md).

## Build / test / run

```bash
go build ./...          # build everything
go vet ./...            # keep this clean
go test ./...           # unit + integration tests (all packages)

# Exercise the CLI against the bundled fixture:
go run ./cmd/pesto run --root internal/runner/testdata/basic -v
go run ./cmd/pesto init /tmp/demo --module example.com/demo
```

Always run `go test ./...` and `go vet ./...` before considering a change done.

## Testing conventions

- The runtime library is tested from `package pesto_test` via httptest servers
  (see `pesto_test.go`, `config_test.go`).
- Transform/report logic has fast, offline unit tests (golden-style string checks).
- End-to-end CLI tests use throwaway modules under `testdata/` with a
  `replace github.com/bulkashmak/pesto => ../../...` directive so they resolve to
  this checkout and run offline (see `internal/runner/testdata/basic`,
  `internal/scaffold/e2e_test.go`). `testdata/` is ignored by the parent build.
- Keep tests deterministic and offline. The scaffolded example targets a public API
  (jsonplaceholder), but automated tests must not depend on the network - the init
  e2e compiles the scaffold with `-run '^$'` instead of hitting it.

## Code conventions

- Follow the surrounding style; godoc every exported identifier (name-first).
- Assertions are **soft by default** (`Expect()` uses `t.Errorf`, collecting all
  failures); transport/decode errors are **fatal** (`t.Fatalf`).
- `pesto.T` embeds `*testing.T` on purpose - never hide it; it is what lets the
  transform inject `t.Parallel()` and lets any assertion library work.
- Prefer stdlib; add a dependency only with clear justification.

## Documentation

Docs live in three layers, each with a different audience and lifecycle. Keep them
separate - never mix working notes into published docs.

**1. In-source (godoc)** - the API reference, for anyone reading the code
- `doc.go` per package with a `// Package ...` comment
- Doc comment on every exported identifier, starting with its name
- Prefer runnable `Example` functions - they're compiled and tested, so they
  can't drift. Published docs link to this layer, they don't duplicate it

**2. End-user docs - `docs/`** - for people using Pesto. Published and stable;
describes only what's true right now. Content lands here only once implemented
- `docs/overview.md` - the problem and philosophy
- `docs/architecture.md` - how Pesto is built
- `docs/concepts.md` - the building blocks
- `docs/guides/` - task-oriented how-tos (getting-started)
- `docs/reference/` - dry lookups: CLI commands, config keys
- Mark not-yet-built building blocks **(planned)** rather than implying they exist

**3. Working/design docs - `docs/design/`** - for the team and maintainer.
Iterative, user-story/task style. Split by lifecycle state:

- `docs/design/active/` - being iterated on or implemented
- `docs/design/parked/` - useful but postponed; a backlog to revisit, not a
  graveyard
- `docs/design/shipped/` - implemented and promoted; kept as a decision record

Every design note carries a status header:

```markdown
- **Status:** active | parked | shipped
- **Owner:** <name>
- **Promoted to:** docs/concepts.md#assertions   (filled once shipped)
```

**The pipeline (3 → 2)**: a note starts in `docs/design/active/`, gets iterated,
then implemented. On ship, extract the durable user-facing parts into `docs/`
(that's promotion - not a file move), move the note to `docs/design/shipped/`, and
mark it `Status: shipped`. The messy *why* (debate, rejected alternatives) stays in
the design note; the settled *what/how* lives in the end-user doc. A design note
and its end-user doc are two different documents, not one document over time.

When you implement or change a feature, update the relevant end-user docs and the
roadmap in the same change, and add an "Implemented" note to the design note.
