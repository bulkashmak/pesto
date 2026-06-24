# Pesto

A structured, fast black-box backend test-automation framework for QA engineers
Built in Go

---

## Why Pesto exists

Every company and every project reinvents its test-automation setup from scratch
Pesto takes the parts that work elsewhere and combines them:

- **Speed and low overhead — from Go.** A single static binary, low memory,
  fast parallel execution, and no requirement for a heavy IDE.
- **Structure from the start — like Vue.** `pesto init` gives you a complete,
  opinionated project layout immediately.
- **Code generation — like Django.** `pesto generate` scaffolds tests, clients,
  and suites so you write test logic, not boilerplate.

## What it looks like

```bash
# Scaffold a new test project (opinionated structure, ready to run)
pesto init acme-api-tests

# Generate a test for an endpoint
pesto generate test users/create

# Run the whole suite against staging, in parallel
pesto run --env staging
```

A generated test reads like this:

```go
func TestCreateUser(t *pesto.T) {
    resp := t.Client.POST("/users").
        JSON(map[string]any{"name": "Bob"}).
        Send()

    resp.Expect().
        Status(201).
        JSON("id").Exists()
}
```

You run it with `pesto run`, which transforms these into standard Go tests, drives
Go's test engine, and collects reports. Under the hood it's the Go `testing`
package — Pesto adds the structure, a typed client, fluent assertions,
environments, and reporting on top.

## Core ideas

| Goal | How Pesto delivers it |
| --- | --- |
| Fast | Go: compiled, low-RAM, parallel by default, editor-agnostic |
| Structured | `pesto init` lays out the project the same way every time |
| No boilerplate | `pesto generate` scaffolds tests, clients, and suites |
| Independent | Lives in its own repo, talks to the backend over HTTP |
| Familiar | Runs on Go's standard `testing` engine, via `pesto run` |

## Test levels

Pesto organizes tests by **level** as a core concept:

- **Contract** (`tests/contract/`) — schema-driven request/response validation for a
  single endpoint, with no business logic. Bodies are described in **JSON Schema**
  (seedable from OpenAPI); Pesto auto-builds the valid request and derives a
  negative case per field violation, checking the API responds correctly to each.
  Organized to mirror the API surface (`GET /users` → `tests/contract/users/get/`).
- **E2E** (`tests/e2e/`) — classic hand-written tests for business logic and user
  journeys, organized by feature/domain (`tests/e2e/transfer/`).

## Documentation

- [`docs/overview.md`](docs/overview.md) — the problem and the philosophy
- [`docs/architecture.md`](docs/architecture.md) — how Pesto is put together
- [`docs/concepts.md`](docs/concepts.md) — the core building blocks
- [`docs/wip/`](docs/wip/) — roadmap and in-progress design notes

## Scope (first pass)

- **Protocol:** REST / HTTP+JSON, first-class. gRPC and GraphQL may follow.
- **Authoring:** Go test files, scaffolded by the Pesto CLI.
- **Runtime:** Go 1.22+.

## License

TBD.
