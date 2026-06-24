# Design note: CLI (WIP)

Status: **open**. This captures the intended CLI surface and the questions that
are still undecided. The CLI is Pesto's primary interface for QA engineers, so it
carries the "Django-like codegen" and "Vue-like structure" goals.

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

Creates the full project layout (see [../architecture.md](../architecture.md)):
`pesto.yaml`, `environments/`, `clients/`, `tests/`, `fixtures/`, `helpers/`,
`reports/`, `go.mod`.

Open questions:

- Does it run `go mod init` / `go get` for the user, or just write files?
- Interactive prompts (base URL, first environment) vs. flags vs. pure defaults?
- Should it create a runnable example test so `pesto run` works immediately?

### `pesto generate ...`

The Django-style scaffolding. Takes a path/name and writes a file from a template
that follows project conventions.

The generate command is **level-aware** (see
[design-test-levels.md](design-test-levels.md)): `contract` scaffolds a
schema-driven endpoint bundle under `tests/contract/<api-path>/` (mirroring the URL
or RPC method); `e2e` scaffolds a Go test under `tests/e2e/<domain>/`.

Open questions:

- **Path → folder mapping for contract.** `users/post` → `tests/contract/users/post/`;
  how are `{path params}`, hyphens, and casing normalized deterministically?
- **Seed contract from OpenAPI?** `pesto generate contract users/post --from openapi.yaml`
  to derive the schema from an existing spec, vs. an empty schema to fill in.
- `generate e2e transfer/happy_path` → file path and test name mapping. What's the rule?
- Can `generate client` derive a typed client from an OpenAPI spec, or is it an
  empty stub to fill in (Phase 3 vs. later)?
- How configurable are templates? Project-local overrides in `pesto.yaml`?

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
