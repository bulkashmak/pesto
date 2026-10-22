# Core concepts

These are the building blocks a QA engineer works with in Pesto. For how they fit
together, see [architecture.md](architecture.md); to start hands-on, see
[guides/getting-started.md](guides/getting-started.md).

Pesto is under active construction. Building blocks marked **(planned)** are
designed but not yet implemented - see the [roadmap](design/roadmap.md). Available
today: `pesto.T`, the `Client`, `Expect`/`Decode`, environments + auth, REST
contract tests, and the `init`/`generate`/`run` CLI with JUnit reports.

## Test levels

Test **levels** are a core concept in Pesto, not a convention you bolt on. Every
test belongs to a level, and the level decides where the test lives, how it is
structured, and how much Pesto does for you. The first two levels are **Contract**
and **E2E**. The full design - especially the schema-driven contract engine - is
in [`design/active/design-test-levels.md`](design/active/design-test-levels.md).

### Contract tests - `tests/contract/`

Contract tests validate the **request/response contract of a single endpoint** -
shape, required vs. optional fields, data types, and field-level constraints - with
**no business logic**. They are **schema-driven**: you describe what a request and
response should look like, and Pesto automatically builds requests from that schema
and validates the responses against it, including negative cases for each declared
violation.

Schemas are written in **standard JSON Schema** (draft 2020-12) for the request and
response bodies, wrapped in a thin Pesto envelope for the HTTP details (method,
path, status) and the response expected on violation. Pesto **derives** the invalid
variants from the schema itself - a missing required field, a wrong type, an
out-of-range or bad-format value, an unexpected extra field - so you don't list
them by hand. From one schema it builds the happy-path request *and* those negative
cases and checks the API responds correctly to each. Happy-path values come from
the schema's own `examples` (or `const`, `default`, `enum`) - Pesto never invents
them. Seeding schemas from an OpenAPI 3.1 spec is **(planned)**.

Contract tests **follow the API surface**: they mirror the URL path, with each
`{param}` as a `by_<param>` folder and the method as the last folder.
`pesto generate contract <METHOD> <path>` scaffolds one:

```
GET    /users              ->  tests/contract/users/get/
POST   /users              ->  tests/contract/users/post/
GET    /users/{id}         ->  tests/contract/users/by_id/get/
POST   /users/{id}/cards   ->  tests/contract/users/by_id/cards/post/
```

Contract tests cover REST APIs; RPC contracts are **(planned)**. The envelope
format and the derived cases are in the
[contract reference](reference/contract.md).

### E2E tests - `tests/e2e/`

E2E tests are classic, hand-written tests that exercise **business logic and
real user journeys** across one or more endpoints. They are ordinary `*pesto.T`
tests using the Client, Expect, and Fixtures described below.

E2E tests **do not follow the API surface**. They are organized by **feature and
domain**. If the app exposes a money-transfer feature, its tests live together:

```
tests/e2e/transfer/        # everything about the transfer feature
tests/e2e/onboarding/
```

| | Contract | E2E |
| --- | --- | --- |
| Validates | request/response shape & constraints | business logic & user journeys |
| Author writes | a schema | Go test code |
| Requests | auto-generated from schema | hand-written |
| Organized by | API surface (URL path) | feature / domain |
| Folder | `tests/contract/` | `tests/e2e/` |

## `pesto.T` - the test context

Every Pesto test takes a `*pesto.T`. It wraps Go's `*testing.T` and adds
everything a test needs already wired for the selected environment:

```go
func TestCreateUser(t *pesto.T) {
    // t.Client   - HTTP client pointed at the current environment
    // t.Data(ref) - static test data, e.g. t.Data("users.default.id")
    // t.Run(name, func(t *pesto.T)) - a subtest with the same client
    // t.Fixtures  - access to fixtures / test data             (planned)
    // plus the usual t.Log, t.Fatal, t.Parallel, ...
}
```

Because it embeds `*testing.T`, anything you can do in a standard Go test still
works. The `*pesto.T` signature is not discoverable by bare `go test`, so tests
are launched with `pesto run`, which transforms them into standard `*testing.T`
tests before compiling - see
[`design/shipped/design-test-context.md`](design/shipped/design-test-context.md).

## Client - the fluent HTTP layer

The client builds and sends requests with a chainable API and knows the
environment's base URL and auth, so tests stay about behavior, not plumbing:

```go
resp := t.Client.POST("/users").
    JSON(map[string]any{"name": "Bob"}).
    Header("X-Request-Id", "abc").
    Send()
```

Clients can be **scaffolded** (`pesto generate client users`) to give a typed,
domain-specific surface (e.g. `users.New(t.Client).Create(payload)`) instead of
raw paths - the Django-style codegen at work. The stub embeds `*pesto.Client`;
you add one method per operation.

## Expect - fluent assertions

Assertions read top-to-bottom and accumulate failures with clear messages:

```go
resp.Expect().
    Status(201).
    Header("Content-Type").Contains("application/json").
    JSON("id").Exists().
    JSON("name").Equals("Bob")
```

The assertion vocabulary (status, headers, JSON path, schema, timing) is an open
design area - see [`design/active/design-assertions.md`](design/active/design-assertions.md).

### Decode to a struct and assert with any library

The fluent `Expect()` API is for quick, inline checks. When a response is complex,
or you simply prefer another assertion library, decode the body into your own
struct and assert however you like - Pesto gets out of the way:

```go
type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

var user User
resp.Decode(&user)   // unmarshal body into the struct; fails the test on error

// from here, use whatever you want - testify, gotest.tools, go-cmp, or stdlib:
require.Equal(t, "Bob", user.Name)
require.NotEmpty(t, user.ID)
```

`resp.Decode` reports a decode failure through the test's `*testing.T` (so a bad
body fails the test with a clear message) and leaves the assertions entirely to
you. This is the escape hatch for anything the fluent API doesn't cover.

## Environments

An environment captures everything that differs between where you run the tests:
where and how to connect, and the test data that exists there. Each one is a
folder:

```
environments/staging/
├── env.yaml          # base URL, auth, headers
└── data/             # static test data (optional)
    └── users.yaml
```

```yaml
# environments/staging/env.yaml
base_url: https://staging.acme.example
auth:
  type: bearer
  token: ${ACME_STAGING_TOKEN}   # expanded from env var / secret at load time
```

You select one at run time (`pesto run --env staging`). The same test runs against
any environment without modification. String values support `${VAR}` expansion so
secrets stay out of the files. An environment with no data can also be a single
`environments/<name>.yaml`. Full format: [reference/config.md](reference/config.md).

## Test data

Pesto distinguishes two kinds of test data:

- **Static** - predefined data that is permanent in the target's database: the
  user every environment is seeded with, a tenant, a catalogue. Tests only read it.
- **Dynamic** - data a test creates, updates or deletes while it runs. That is
  what [fixtures](#fixtures-planned) are for.

Static data differs between environments (user `42` in dev may be `9001` in
staging), so it lives in the environment's `data/` folder. Each file is one
entity, holding named records:

```yaml
# environments/staging/data/users.yaml
default:
  id: 9001
  email: qa-default@acme.example
  password: ${STAGING_DEFAULT_PASSWORD}
```

A test reads a value with `t.Data("<file>.<record>.<field>")`:

```go
func TestGetUser(t *pesto.T) {
    id := t.Data("users.default.id")   // "9001" on staging
    t.Client.GET("/users/" + id).Send().Expect().Status(200)
}
```

A missing reference fails the test immediately, naming the environment's file and
what is missing. Contract tests fill `{param}` path parameters and request
body fields (`request.data`) the same way. Full
format: [reference/config.md](reference/config.md#static-test-data).

## Fixtures (planned)

Fixtures provide reusable test data and setup/teardown so tests don't repeat
themselves and clean up after themselves:

```go
func TestListUsers(t *pesto.T) {
    user := t.Fixtures.Users.Create()   // created here, torn down automatically
    resp := t.Client.GET("/users").Send()
    resp.Expect().Status(200).JSON("data").Contains(user.ID)
}
```

The exact fixture lifecycle and isolation model is being designed in
[`design/active/design-fixtures.md`](design/active/design-fixtures.md)

## Suites

A suite groups related E2E tests under a feature/domain folder
(`tests/e2e/<domain>/`) with a `suite_test.go` for shared helpers. Generated with
`pesto generate suite transfer` - or implicitly by the first
`pesto generate e2e transfer/<name>` - it gives a domain its own folder and
conventions. (Contract tests are organized by API surface instead - see
[Test levels](#test-levels).)

## The CLI

The `pesto` command is the entry point for everything a QA engineer does:

| Command | Purpose | Status |
| --- | --- | --- |
| `pesto init <name>` | Scaffold a new test project (full structure). | available |
| `pesto run [--env <e>] [--report junit]` | Transform + run tests, with reports. | available |
| `pesto version` | Print the CLI version. | available |
| `pesto generate e2e <domain>/<name>` | Scaffold an E2E test. | available |
| `pesto generate suite <domain>` | Scaffold a test suite for a domain. | available |
| `pesto generate client <name>` | Scaffold a typed API client stub. | available |
| `pesto generate contract <api-path>` | Scaffold a schema-driven contract bundle. | planned |

Full command/flag details: [reference/cli.md](reference/cli.md). The command surface
is designed in [`design/active/design-cli.md`](design/active/design-cli.md).

## Reports

After a run, Pesto writes results to `reports/`: JUnit XML for CI integration plus
human-readable output. ("JUnit XML" is the CI-standard test-report schema that
Jenkins, GitLab, GitHub Actions, and others ingest - it's the format, not the
JUnit tool; Pesto does not use JUnit.) Reporting formats are tracked in the
[roadmap](design/roadmap.md)
