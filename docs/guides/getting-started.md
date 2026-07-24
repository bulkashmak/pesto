# Getting started

This walks you from nothing to a passing Pesto test and a CI report. It reflects
what is implemented today (Phases 1-2); see the [roadmap](../design/roadmap.md) for
what is planned.

## 1. Install the CLI

The `pesto` command is a normal Go binary installed from the `cmd/pesto` package:

```bash
go install github.com/bulkashmak/pesto/cmd/pesto@latest
```

> Pre-release note: until the first tagged release exists, install from a checkout:
> `git clone https://github.com/bulkashmak/pesto && cd pesto && go install ./cmd/pesto`

The runtime library (`github.com/bulkashmak/pesto`) is a separate, importable Go
package - your test project depends on it, but it is not installed as a binary.

## 2. Scaffold a project

```bash
pesto init acme-api-tests
cd acme-api-tests
```

This writes a complete, opinionated layout: `pesto.yaml`, `environments/`,
`tests/e2e/example/`, `clients/`, `fixtures/`, `helpers/`, `reports/`, plus a
`go.mod` and a runnable example test.

Resolve the runtime dependency the way the printed next-steps suggest:

```bash
go get github.com/bulkashmak/pesto@latest
```

## 3. Run it

```bash
pesto run
```

Pesto transforms the authored `func TestGetTodo(t *pesto.T)` into a standard Go
test and drives `go test`. The scaffolded example targets a public sandbox API, so
it passes immediately (when online):

```
=== RUN   TestGetTodo
--- PASS: TestGetTodo (0.17s)
PASS
```

> `pesto run` is required - authored tests use a `*pesto.T` signature that bare
> `go test` will not discover. See
> [architecture.md](../architecture.md) for why.

## 4. Produce a CI report

```bash
pesto run --report junit
```

This writes `reports/junit.xml` in the CI-standard JUnit schema (Jenkins, GitLab,
GitHub Actions, ... ingest it directly) while still printing the normal console
output.

## 5. Point it at your own API

Edit `environments/dev.yaml`:

```yaml
base_url: https://api.staging.acme.example
auth:
  type: bearer
  token: ${ACME_TOKEN}   # expanded from the environment at run time
```

Select an environment per run:

```bash
pesto run --env dev
```

See the [config reference](../reference/config.md) for the full file format and the
[CLI reference](../reference/cli.md) for every command and flag.

## 6. Write a test

Replace the example with your own, using the fluent client + assertions - or decode
into a struct and assert with any library you like:

```go
package users

import "github.com/bulkashmak/pesto"

func TestCreateUser(t *pesto.T) {
	resp := t.Client.POST("/users").
		JSON(map[string]any{"name": "Bob"}).
		Send()

	resp.Expect().
		Status(201).
		JSON("id").Exists().
		JSON("name").Equals("Bob")
}
```

For the building blocks, see [concepts.md](../concepts.md).
