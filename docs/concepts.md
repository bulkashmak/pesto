# Core concepts

These are the building blocks a QA engineer works with in Pesto. The shapes shown
here are illustrative of the intended design - the framework is not yet
implemented. For how they fit together, see [architecture.md](architecture.md)

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
cases and checks the API responds correctly to each. If you have an OpenAPI 3.1
spec, schemas can be seeded from it.

Contract tests **follow the API surface**. For REST they mirror the URL path; for
RPC they mirror the method name:

```
GET  /users           ->  tests/contract/users/get/
POST /users           ->  tests/contract/users/post/
GET  /users/{id}      ->  tests/contract/users/get_by_id/
UserService.GetUser   ->  tests/contract/userservice/getuser/
```

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
| Organized by | API surface (URL path / RPC method) | feature / domain |
| Folder | `tests/contract/` | `tests/e2e/` |

## `pesto.T` - the test context

Every Pesto test takes a `*pesto.T`. It wraps Go's `*testing.T` and adds
everything a test needs already wired for the selected environment:

```go
func TestCreateUser(t *pesto.T) {
    // t.Client   - HTTP client pointed at the current environment
    // t.Env       - resolved environment config + variables
    // t.Fixtures  - access to fixtures / test data
    // plus the usual t.Log, t.Fatal, t.Parallel, ...
}
```

Because it embeds `*testing.T`, anything you can do in a standard Go test still
works. The `*pesto.T` signature is not discoverable by bare `go test`, so tests
are launched with `pesto run`, which transforms them into standard `*testing.T`
tests before compiling - see
[`design/active/design-test-context.md`](design/active/design-test-context.md).

## Client - the fluent HTTP layer

The client builds and sends requests with a chainable API and knows the
environment's base URL and auth, so tests stay about behavior, not plumbing:

```go
resp := t.Client.POST("/users").
    JSON(map[string]any{"name": "Bob"}).
    Header("X-Request-Id", "abc").
    Send()
```

Clients can be **generated** (`pesto generate client users`) to give a typed,
domain-specific surface (e.g. `clients.Users(t).Create(payload)`) instead of raw
paths - the Django-style codegen at work.

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

An environment captures everything that differs between where you run the tests -
base URL, auth, and free-form variables:

```yaml
# envs/staging.yaml
base_url: https://staging.acme.example
auth:
  type: bearer
  token: ${ACME_STAGING_TOKEN}   # from env var / secret
variables:
  default_account: 42
```

You select one at run time (`pesto run --env staging`). The same test runs against
any environment without modification.

## Fixtures

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
(`tests/e2e/<domain>/`) with shared setup. Generated with
`pesto generate suite transfer`, it gives a domain its own folder, conventions,
and shared hooks. (Contract tests are organized by API surface instead - see
[Test levels](#test-levels).)

## The CLI

The `pesto` command is the entry point for everything a QA engineer does:

| Command | Purpose |
| --- | --- |
| `pesto init <name>` | Scaffold a new test project (full structure). |
| `pesto generate test <path>` | Scaffold a test file from conventions. |
| `pesto generate client <name>` | Scaffold a typed API client. |
| `pesto generate suite <name>` | Scaffold a test suite for a domain. |
| `pesto run [--env <e>] [--filter <f>]` | Run tests for an environment, with reports. |

The full command surface is being designed in
[`design/active/design-cli.md`](design/active/design-cli.md)

## Reports

After a run, Pesto writes results to `reports/`: JUnit XML for CI integration plus
human-readable output. Reporting formats are tracked in the
[roadmap](design/roadmap.md)
