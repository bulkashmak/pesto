# Getting started

This walks you from nothing to a passing Pesto test and a CI report. It reflects
what is implemented today (Phases 1-3); see the [roadmap](../design/roadmap.md) for
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

## 6. Scaffold your own test

Scaffold a test into a new domain; the domain's suite file (a home for its shared
helpers) is created automatically on first use:

```bash
pesto generate e2e users/create
# created tests/e2e/users/suite_test.go
# created tests/e2e/users/create_test.go
```

Then fill in the generated `func TestCreate(t *pesto.T)`, using the fluent
client + assertions - or decode into a struct and assert with any library you
like:

```go
func TestCreate(t *pesto.T) {
	resp := t.Client.POST("/users").
		JSON(map[string]any{"name": "Bob"}).
		Send()

	resp.Expect().
		Status(201).
		JSON("id").Exists().
		JSON("name").Equals("Bob")
}
```

Run just this test while iterating:

```bash
pesto run -run TestCreate -v
```

`pesto generate suite <domain>` (a domain folder by itself) and
`pesto generate client <name>` (a typed client stub under `clients/`) round out
the scaffolding - see the [CLI reference](../reference/cli.md) for the naming
rules.

For the building blocks, see [concepts.md](../concepts.md).
