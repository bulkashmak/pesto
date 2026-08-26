# Design note: CLI

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

This captures the intended CLI surface and the questions that
are still undecided. The CLI is Pesto's primary interface for QA engineers, so it
carries the "Django-like codegen" and "Vue-like structure" goals.

> **Implemented (Phase 2):** `pesto init`, `pesto run` (with `--env`/`--report`/
> `--out`/`--keep` and `go test` passthrough), and `pesto version`. See
> [reference/cli.md](../../reference/cli.md) for the shipped surface. `pesto generate`
> is still to come (Phase 3+).

## Intended commands

```
pesto init <name>                       # scaffold a new test project
pesto generate contract <api-path>      # e.g. pesto generate contract users/post
pesto generate e2e <domain>/<name>      # e.g. pesto generate e2e transfer/happy_path
pesto generate suite <domain>           # e2e feature/domain folder + shared setup
pesto generate client <name>
pesto run [flags]                       # run tests
pesto version
```

### `pesto init`

Creates the full project layout (see [../architecture.md](../../architecture.md)):
`pesto.yaml`, `environments/{dev,test}.yaml`, `clients/`, `tests/e2e/example/`,
`fixtures/`, `helpers/`, `reports/`, `go.mod`, `.gitignore`, `README.md`.

Decided (implemented in Phase 2):

- **Writes files only; does not run `go mod init` / `go get`.** It emits a `go.mod`
  requiring the runtime, and the printed next-steps tell the user to run
  `go get github.com/bulkashmak/pesto@latest`. Rationale: keep init offline and
  side-effect-free; module resolution is the user's `go get`. It refuses to
  overwrite existing files.
- **Pure defaults + a `--module` flag** (default module path = the project dir
  name). No interactive prompts - `init` is one non-interactive step.
- **Yes, it creates a runnable example test.** `tests/e2e/example/example_test.go`
  is a real `*pesto.T` test (`GET /todos/1`) against a public sandbox API
  (`jsonplaceholder.typicode.com`) set as the default `base_url`, so
  `pesto init && pesto run` passes immediately **when online**. It teaches the real
  config-driven pattern rather than a self-contained stub.

Still open:

- **go.mod pesto version.** Today init pins `v0.0.0` and leans on the user's
  `go get`; once the CLI has release tags it should inject its own version so the
  require resolves without a manual `go get`.
- **Offline first run.** The example needs network on first `pesto run`. Acceptable
  for an API-testing tool, but an offline/self-contained example mode could be an
  option later.

### `pesto generate ...`

The Django-style scaffolding. Takes a path/name and writes a file from a template
that follows project conventions.

The generate command is **level-aware** (see
[design-test-levels.md](design-test-levels.md)): `contract` scaffolds a
schema-driven endpoint bundle under `tests/contract/<api-path>/` (mirroring the URL
or RPC method); `e2e` scaffolds a Go test under `tests/e2e/<domain>/`.

Decided (drafted for Phase 3):

- **One shared normalization rule** for every user-supplied name segment:
  lowercase, hyphens and spaces become underscores, and the result must match
  `[a-z][a-z0-9_]*` - otherwise generate errors out with the offending segment.
  Deterministic: the author can predict the file path from the command.
- **`generate e2e <domain>/<name>`** → `tests/e2e/<domain>/<name>_test.go`,
  `package <domain>`, containing `func Test<CamelCase(name)>(t *pesto.T)`.
  Example: `transfer/happy_path` → `tests/e2e/transfer/happy_path_test.go` with
  `func TestHappyPath`. Exactly two segments in v1; deeper nesting is an error
  until a real need shows up.
- **`generate suite <domain>`** → creates `tests/e2e/<domain>/` with a
  `suite_test.go` holding the package comment and a home for shared helpers.
  `generate e2e` into a domain that doesn't exist yet implies `generate suite`
  first, so the common path is one command.
- **`generate client <name>`** → an empty typed stub `clients/<name>/<name>.go`
  wrapping `*pesto.Client`. Deriving a client from an OpenAPI spec stays in
  later/maybe (see [roadmap.md](../roadmap.md)) - Phase 3 is templates only.
- **Templates are compiled into the binary and not configurable in v1.**
  Project-local template overrides (e.g. via `pesto.yaml`) are deferred until real
  usage demands them.
- Like `init`, generate **refuses to overwrite existing files**.

Still open (deferred to Phase 4, with the contract engine):

- Contract path → folder mapping beyond the shared normalization rule: path params
  map as `GET /users/{id}` → `users/get_by_id/`; confirm the multi-param form and
  the RPC casing rule when the engine lands (see
  [design-test-levels.md](design-test-levels.md)).
- **Seed contract from OpenAPI** (`--from openapi.yaml`): envelope + schemas from
  an OpenAPI 3.1 operation vs. an empty schema to fill in.

### `pesto run`

Resolves the environment, invokes the Go test toolchain, collects reports.

Proposed flags:

- `--env <name>` - select environment (default from `pesto.yaml`)
- `--filter <pattern>` - run a subset (maps to `go test -run`?)
- `--parallel <n>` - concurrency
- `--report <format>` - junit | json | html (repeatable?)
- `--tags <tags>` - select by tag/label

Open questions:

- Is filtering by Go test name (`-run`) enough, or do we need a Pesto-level
  tag system independent of function names?
- How are reports written - always to `reports/`, or to a `--out` path?
- Exit-code semantics for CI (nonzero on any failure - anything subtler needed?).

## Cross-cutting questions

- **Single binary vs. `go run`.** Distribute `pesto` as a prebuilt binary, install
  via `go install`, or both?
- **Versioning.** How does the CLI version relate to the runtime library version
  a project imports? Must they match?
- **Relationship to raw `go test`.** Authored tests use a `*pesto.T` signature, so
  `pesto run` is **required** - it runs a codegen/transform step first (see
  [design-test-context.md](design-test-context.md)). Bare `go test` will not run a
  Pesto project. Open question: should the generated `*testing.T` wrappers be left
  on disk (so a curious user *can* inspect/run them) or produced in a temp dir?
