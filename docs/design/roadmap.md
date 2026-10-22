# Roadmap (WIP)

> This is a working document. Nothing here is committed; it captures the intended
> order of building Pesto and what "done" means for each phase.

## Phase 0 - Documentation & design (ongoing alongside development)

- [x] README with the pitch and vision
- [x] `docs/` overview, architecture, concepts
- [ ] `docs/design/active/` design notes for the open questions (in progress)
- [x] Decide module path and minimum Go version
      (`github.com/bulkashmak/pesto`, Go 1.24+)

## Phase 1 - Runtime core (the library)

The smallest thing that lets someone write a Pesto test by hand, no CLI yet.

- [x] `pesto.T` embedding `*testing.T`, constructed via `pesto.New(t, opts…)`
- [x] `Client` - fluent HTTP builder over `net/http` (GET/POST/PUT/PATCH/DELETE,
      JSON body, headers)
- [x] `Expect` - status, header, and JSON-path assertions (+ `Decode` escape hatch)
- [x] Environment config loading (`pesto.yaml` + `environments/*.yaml`,
      with `${VAR}` expansion)
- [x] Auth: bearer + basic to start

**Done when:** a hand-written standard `func TestX(t *testing.T)` that calls
`pesto.New(t)` can hit a real API and assert on it via `go test`. (The clean
`*pesto.T` authoring signature comes online with the transform in Phase 2.)

## Phase 2 - CLI: structure & the transform

- [x] `pesto init <name>` - scaffold the full project layout (runnable example)
- [x] **Transform step** - rewrite `func TestX(t *pesto.T)` into standard
      `*testing.T` wrappers via `go test -overlay` + AST rewrite
      (see [design-test-context.md](shipped/design-test-context.md))
- [x] `pesto run --env <e>` - resolve config, transform, drive `go test`, exit codes
- [x] JUnit XML report output (the CI-standard XML schema, not the JUnit tool)
      via `pesto run --report junit [--out <path>]`

**Done when:** `pesto init` + a `*pesto.T` test + `pesto run` works end to end on a
fresh project.

## Phase 3 - CLI: code generation (E2E level)

- [x] `pesto generate e2e <domain>/<name>`
- [x] `pesto generate suite <domain>`
- [x] `pesto generate client <name>`
- [x] Templates follow project conventions consistently
      (built into the binary, gofmt-clean, shared name normalization)

**Done when:** a QA engineer can scaffold and run a new E2E test without writing
boilerplate.

## Release v0.1.0

The first tagged release: Phases 1-3 installable with `go install ...@v0.1.0`.

- [x] CI (gofmt, vet, race tests, build) on the minimum and latest Go
- [x] `pesto version` reports the build's real version (`internal/version`)
- [x] `pesto init` pins the runtime to the CLI's version
- [x] `pesto run` warns on a CLI/runtime version mismatch
- [x] `CHANGELOG.md`
- [x] Tag `v0.1.0` and verify a clean install: `go install`, `init`, `run`
      with no local `replace` (verified from proxy.golang.org on 2026-10-20)

## Phase 4 - Contract level (schema-driven)

The first non-E2E level. See [design-test-levels.md](active/design-test-levels.md).

- [x] Minimal setup primitive: env-supplied auth + static test data for
      referenced resources (`t.Data`, `path_params`) - full fixtures stay Phase 5 (see
      [design-fixtures.md](active/design-fixtures.md))
- [x] Static test data: `environments/<env>/data/*.yaml`, `t.Data`, data
      references in `path_params` - replaces `variables` / `t.Var` (see
      [design-fixtures.md](active/design-fixtures.md#decided-drafted-static-vs-dynamic-test-data))
- [x] Request body fields from static data (`request.data` in `contract.yaml`) -
      see [design-test-levels.md](active/design-test-levels.md#body-values-from-static-data-requestdata-decided-drafted)
- [x] JSON Schema (2020-12) bodies + thin Pesto HTTP envelope (`contract.yaml`)
- [ ] OpenAPI 3.1 seeding (`generate contract --from openapi.yaml`)
- [x] Contract engine (`contract.Run`): build happy-path request from the schema's
      `const`/`examples`/`default`/`enum` (no constraint-based value generation
      in v1), validate response
- [x] Negative cases: derive one per field violation from the schema, assert
      `on_violation` status (`pattern`/`format`/array items not derived yet)
- [x] `pesto generate contract <METHOD> <path>` (REST path → folder mapping)
- [x] Contract tests discovered and run by `pesto run` alongside E2E

**Done when:** a schema under `tests/contract/users/post/` produces passing
happy-path + violation checks via `pesto run`.

## Release v0.2.0

REST contract tests and static test data, installable with
`go install ...@v0.2.0`. OpenAPI seeding is not in it.

- [x] `CHANGELOG.md` entry
- [ ] Tag `v0.2.0` and verify a clean install: `go install`, `init`, `run`,
      `generate contract` with no local `replace`

## Phase 5 - Fixtures & data

- [ ] Fixture lifecycle (setup/teardown, isolation) - see
      [design-fixtures.md](active/design-fixtures.md)
- [ ] Data builders / factories
- [ ] Parallel-by-default: transform-time `t.Parallel()` injection, gated on
      isolation being solid (see
      [design-parallelism.md](active/design-parallelism.md))

## Phase 6 - Reporting & DX polish

- [ ] HTML report
- [ ] Test filtering and tagging (including filtering by level)
- [ ] Parallelism controls and guidance
- [ ] CI recipes (GitHub Actions, GitLab)

## Later / maybe

- Additional test levels (integration, smoke)
- RPC contract tests (dropped from Phase 4 until a concrete RPC target exists)
- gRPC and GraphQL clients (see [design-protocols.md](active/design-protocols.md)) -
  enables RPC contract tests
- Typed client generation from OpenAPI specs
- A higher-level / lower-barrier authoring layer (see
  [design-authoring.md](active/design-authoring.md))
- Retry/polling helpers for eventually-consistent systems
