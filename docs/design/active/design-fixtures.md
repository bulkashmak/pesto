# Design note: fixtures & test data

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

Fixtures are how tests get the data they need and clean up after
themselves. Getting isolation right is what makes a suite trustworthy.

## Goals

- **Reusable setup.** Creating "a user", "an order", etc. shouldn't be copy-pasted
  into every test.
- **Automatic teardown.** Whatever a test creates should be removed after, so runs
  don't pollute the environment or each other.
- **Isolation.** Parallel tests must not step on each other's data.

## Sketch

```go
func TestListUsers(t *pesto.T) {
    user := t.Fixtures.Users.Create()   // created now
    // ... registered for automatic cleanup when the test ends
    resp := t.Client.GET("/users").Send()
    resp.Expect().Status(200).JSON("data").Contains(user.ID)
}
```

## Decided (drafted): v1 scope splits across Phases 4 and 5

Contract tests (Phase 4) need just enough setup to hit a real endpoint - a valid
token and sometimes a pre-existing referenced resource - while the full fixture
lifecycle is Phase 5. To keep the roadmap boundary honest:

- **Phase 4 ships a minimal primitive only.** Auth comes from the environment
  (already built); referenced values (a valid `{id}`, a tenant) come from the
  environment's static test data (see below - this replaces the earlier
  environment `variables`); and any helper that *does* create data registers its
  teardown with `t.Cleanup` immediately after the create succeeds. No `t.Fixtures`
  accessor, no factories, no namespacing.
- **Phase 5 ships the real thing.** `t.Fixtures` on `pesto.T`, factories/builders,
  isolation namespacing, suite-level lifetimes. Phase 5 is also the gate for
  parallel-by-default (see [design-parallelism.md](design-parallelism.md)):
  `t.Parallel()` injection stays off until isolation makes it safe.
- **Teardown base mechanism is `t.Cleanup`.** It runs on failure and on panic
  within the test goroutine, which covers the realistic cases; the open questions
  below narrow to the gaps (un-captured IDs, process death).

## Decided (drafted): static vs. dynamic test data

Pesto names two kinds of test data:

- **Static** - predefined data that is permanent in the target's database: the
  user `42` that every environment is seeded with, a tenant, a product catalogue.
  Tests only *read* it; they never create, change or delete it.
- **Dynamic** - data a test creates, updates or deletes while it runs. It exists
  only for the duration of a test (or suite) and is the subject of everything
  else in this note: factories, teardown, isolation.

### Why static data lives in the environment

The first cut put referenced values in environment `variables`
(`variables: {users_id: "42"}`) and read them with `t.Var`. That mixed two
things in one file: *where and how to connect* (`base_url`, auth) and *what data
exists there*. Static data deserves its own home.

A separate `./testdata/static` tree was considered and rejected. Static data is a
fact **about an environment** - user `42` exists in dev's database, staging's may
be `9001` - so a standalone tree would need a per-environment layer mirroring
`environments/` anyway, and every lookup would need both an environment name and
a data path. Keeping it inside the environment makes that pairing the folder
structure.

Dynamic data is the opposite: a "new user" payload has the same shape
everywhere, so its templates and factories stay environment-agnostic, in
`fixtures/` (Phase 5). The rule of thumb:

| Depends on the environment | Works in any environment |
| --- | --- |
| `environments/<env>/` - connection + static data | `fixtures/` - dynamic data templates and factories |

### Naming

"Static" and "dynamic" stay as the *concepts* in docs. The names users type -
the folder and the API - are plain `data` (`environments/<env>/data/`,
`t.Data`): it is the more obvious word, and inside an environment folder there
is no dynamic data to confuse it with. `static/` was the first draft.

### Layout

An environment becomes a folder:

```
environments/
├── dev/
│   ├── env.yaml            # base_url, auth, headers
│   └── data/
│       ├── users.yaml      # records permanent in dev's database
│       └── cards.yaml
└── staging/
    ├── env.yaml
    └── data/
        └── users.yaml
```

- `env.yaml` holds what `<env>.yaml` holds today, minus `variables`.
- `data/` is optional. Each file is one entity; its top-level keys are named
  records; a record is a map of fields:

  ```yaml
  # environments/dev/data/users.yaml
  default:
    id: 42
    email: qa-default@example.com
    password: ${DEV_DEFAULT_USER_PASSWORD}
  blocked:
    id: 43
  ```

- Values are YAML scalars (strings, numbers, booleans) and support the same
  `${VAR}` expansion as `env.yaml`, so a static user's password stays out of git.
  Fields may nest (`address.city`); lists are not supported in v1.
- The flat `environments/<env>.yaml` form shipped in v0.1.0 keeps working and
  means "an environment with no static data". Having both `<env>.yaml` and
  `<env>/` is an error, so there is never a question of which one wins.

### Referencing static data

A reference is a dotted path: `<file>.<record>.<field>[.<nested>...]` -
`users.default.id` reads field `id` of record `default` in `data/users.yaml`
of the selected environment.

From Go, `t.Data` replaces `t.Var`:

```go
func TestGetUser(t *pesto.T) {
    id := t.Data("users.default.id")       // "42"
    resp := t.Client.GET("/users/" + id).Send()
    resp.Expect().Status(200)
}
```

- `t.Data` returns the value formatted as a string (numbers in their YAML
  form), since its main uses are paths, query strings and headers.
- A missing reference is **fatal** (`t.Fatalf`), not a soft failure: the test
  cannot meaningfully continue. The message names the environment and the file,
  e.g. `pesto: data users.default.id: no record "default" in
  environments/staging/data/users.yaml`.
- A comma-ok form (`t.LookupData`) covers the rare optional case.

From contract tests, `path_params` values become data references:

```yaml
# contract.yaml
path: /users/{id}/cards
path_params:
  id: users.default.id
```

`contract.Run` resolves every reference before sending any request, so a missing
record fails the contract up front.

### What goes away

- **`variables` in environment files, and `t.Var` / `pesto.WithVariables`.**
  `variables` parsed in v0.1.0 but no test could read it (`t.Var` never shipped),
  so removing it breaks nobody. Unknown keys are ignored by the environment
  loader, so an old file with `variables` still loads.
- **Option for tests:** `pesto.WithData(map[string]any)` replaces
  `WithVariables`, so unit tests can inject static data without files.

### Scope

Static data is part of the Phase 4 minimal primitive (contract `path_params`
depend on it). Dynamic data stays Phase 5.

**Implemented (2026-10-22):** as drafted above - `data.go` (loader + lookup),
`T.Data` / `T.LookupData` / `WithData`, folder environments in `config.go`,
data references in contract `path_params`, `pesto generate contract` emitting
`<segment>.default.<param>` (a leading `{param}` gets the placeholder file
`resources`), and `pesto init` scaffolding `environments/<env>/data/todos.yaml`.
Promoted to [concepts.md#test-data](../../concepts.md#test-data) and
[reference/config.md#static-test-data](../../reference/config.md#static-test-data).
Dynamic data (the rest of this note) is still open.

### Deferred

- **A shared layer** (`environments/_shared/data/`) for data identical across
  environments. Copies are accepted until people actually ask: values usually
  differ, and copies are easy to read.
- **Typed records** - decoding a whole record into a struct
  (`t.DataRecord("users.default", &u)`).
- **Generating data files** (`pesto generate data users`) - copying a file by
  hand is enough for now.

## Open questions

- **Creation strategy.** Do fixtures create data *through the API under test*
  (realistic, but couples setup to the same endpoints being tested) or through a
  side channel (seed endpoint, DB, admin API)?
- **Teardown gaps beyond `t.Cleanup`.** Data created but whose ID we didn't
  capture; the process dying mid-run. Orphan-sweeping? Run-scoped prefixes so
  leftovers are identifiable?
- **Isolation under parallelism.** Namespacing per test (unique prefixes/tenants)?
  (Shared read-only data is static data, above; this question is about dynamic
  data.)
- **Factories / builders.** A `UserFactory` with overridable defaults - what does
  the builder API look like, and how does `pesto generate` scaffold one?
- **Shared/suite-level fixtures.** Expensive setup created once per suite vs. once
  per test - how is the lifetime declared?
- **Environments without write access.** Read-only production-like environments:
  do fixtures degrade to "find existing data" instead of "create"?

## Related

- Generation of fixtures/factories ties into [design-cli.md](design-cli.md).
- Cleanup semantics interact with assertions' soft-failure mode
  ([design-assertions.md](design-assertions.md)).
