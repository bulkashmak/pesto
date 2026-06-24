# Architecture

This document describes how Pesto is put together. For the *why*, see
[overview.md](overview.md); for the day-to-day building blocks, see
[concepts.md](concepts.md).

## Design principle: drive Go's test engine, don't replace it

Pesto does **not** implement its own test runner from scratch - Go's `testing`
package still executes the tests, so parallelism, sub-tests, and failure semantics
are Go's well-understood ones. What Pesto changes is the *authoring surface* and
the *entry point*.

A Pesto test is authored with a clean signature that takes a `*pesto.T`:

```go
func TestCreateUser(t *pesto.T) { ... }
```

Go's own toolchain will not discover that signature - `go test` only runs
`func TestXxx(t *testing.T)`. So **`pesto run` is required**: before compiling, it
performs a **codegen/transform step** that turns each authored `*pesto.T` test into
a standard `func TestXxx(t *testing.T)` wrapper which constructs the `*pesto.T`
(wired to the selected environment) and calls the authored body. Pesto then drives
the normal Go toolchain over the generated package.

The trade-off, chosen deliberately (see
[`wip/design-test-context.md`](wip/design-test-context.md)): tests read cleanly and
carry everything in one `*pesto.T` handle, at the cost of `pesto run` being the
mandatory entry point - you cannot run a Pesto project with bare `go test`.

The `pesto` CLI **scaffolds** project structure and test files (Vue/Django-style)
and **owns the run**: transform, environment selection, filtering, and reporting.

## The two halves of Pesto

```
┌──────────────────────────────────────────────────────────────┐
│  pesto CLI (a Go binary)                                     │
│                                                              │
│   init       scaffold a new project (Vue-style structure)    │
│   generate   scaffold tests / clients / suites (Django-style)│
│   run        transform tests, select env, drive go test,     │
│              collect reports                                 │
└───────────────────────────┬──────────────────────────────────┘
                            │ generates / invokes
                            ▼
┌──────────────────────────────────────────────────────────────┐
│  pesto runtime library (imported by the test project)        │
│                                                              │
│   pesto.T      test context (wraps *testing.T)               │
│   Client       fluent HTTP client                            │
│   Expect       fluent assertions on responses                │
│   Fixtures     setup / teardown of test data                 │
│   Config/Env   environment + base URL + auth resolution      │
│   Report       JUnit / JSON / HTML output                    │
└──────────────────────────────────────────────────────────────┘
```

1. **The CLI** is what a QA engineer types. It scaffolds files and **owns every
   run** - because authored `*pesto.T` tests must be transformed before they
   compile, `pesto run` is required (bare `go test` will not run the project).
2. **The runtime library** is ordinary Go packages that the generated wrappers and
   authored tests import (`github.com/bulkashmak/pesto/...`). This is what actually
   issues requests and makes assertions.

## Anatomy of a Pesto project

`pesto init acme-api-tests` produces a complete, opinionated layout:

```
acme-api-tests/
├── pesto.yaml              # project config: default env, conventions
├── go.mod
├── envs/
│   ├── dev.yaml            # base URL, auth, variables for staging
│   └── test.yaml
├── clients/                # typed API clients (hand-written or generated)
│   └── users.go
├── tests/                  # test cases, split by LEVEL (a core concept)
│   ├── contract/           # schema-driven, mirrors the API surface
│   │   └── users/
│   │       ├── get/        # GET /users  - schema + generated cases
│   │       └── post/       # POST /users
│   └── e2e/                # classic tests, organized by feature/domain
│       └── transfer/
│           └── transfer_test.go
├── fixtures/               # reusable test data + setup/teardown
│   └── users.go
├── helpers/                # shared project-specific helpers
└── reports/                # generated test reports (gitignored)
```

The layout is the same on every project. That sameness is the point: a QA
engineer moving between two Pesto projects already knows where everything lives.
Tests are split by **level** - `contract/` (request/response validation, mirrors
the URL path or RPC method) and `e2e/` (business logic, organized by domain). See
[concepts.md](concepts.md) and [`wip/design-test-levels.md`](wip/design-test-levels.md).

## Execution flow

What happens when a QA engineer runs `pesto run --env staging`:

1. **Resolve config.** The CLI reads `pesto.yaml` and `environments/staging.yaml`
   and exposes the resolved values (base URL, auth, variables) to the runtime.
2. **Transform.** The CLI scans the `tests/` tree for `func TestXxx(t *pesto.T)`
   functions and generates standard `func TestXxx(t *testing.T)` wrappers that
   build the `*pesto.T` and invoke the authored body. (How this transform works -
   AST rewrite vs. build-tagged generated files - is an open question, see
   [`wip/design-test-context.md`](wip/design-test-context.md).)
3. **Run the tests.** The CLI invokes the Go test toolchain over the transformed
   package, passing through filters (`--filter`, tags) and parallelism settings.
4. **Each test runs.** The generated wrapper hands the authored body a `*pesto.T`
   already wired with a `Client` pointed at the selected environment. It builds
   requests, sends them, and asserts on the responses.
5. **Collect results.** The CLI captures pass/fail/timing and writes reports
   (JUnit XML for CI, plus human-readable formats) to `reports/`.

## How config reaches a test

Environments are resolved by the CLI and made available to the runtime so a test
never hard-codes a URL:

```
environments/staging.yaml ─┐
pesto.yaml ────────────────┼─► resolved config ─► pesto.T.Client (base URL, auth)
--env / flags ─────────────┘
```

A test written against `/users` runs unchanged against staging or production; only
`--env` changes.

## What's deliberately out of scope of the core

- **Backend coupling.** Pesto never imports the application under test; it only
  speaks its API.
- **A bespoke runner.** Go's `testing` package still executes the tests; Pesto
  generates the wrappers and drives the toolchain, but does not reimplement it.
- **Non-functional testing.** Load and performance are out of scope (see
  [overview.md](overview.md)).

Open questions about each layer are tracked in [`wip/`](wip/).
