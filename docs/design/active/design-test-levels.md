# Design note: test levels

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

> Direction: decided; mechanics open.

> **Implemented (Phase 4, REST contracts):** the public `contract` package
> (`contract.Run`) and `pesto generate contract <METHOD> <path>`. User docs:
> [concepts.md](../../concepts.md#contract-tests---testscontract) and
> [reference/contract.md](../../reference/contract.md). Decisions taken while
> implementing are under "Phase 4 decisions" below; the remaining open
> questions are marked.

Test **levels** are a core concept: every test belongs to a level, and the level
determines its location, structure, and how much Pesto generates. We start with
two levels - **Contract** and **E2E** - with room to add more later (e.g.
integration, smoke).

## E2E tests

Classic, hand-written tests for business logic and real user journeys. Ordinary
`*pesto.T` tests using Client / Expect / Fixtures.

- **Location:** `tests/e2e/`
- **Organized by:** feature / domain, *not* the API surface. Money transfer →
  `tests/e2e/transfer/`.
- **Authoring:** Go code, scaffolded by `pesto generate` (see
  [design-authoring.md](design-authoring.md)).

E2E is the well-understood level; most open questions live under Contract.

## Contract tests

Validate the **request/response contract of a single endpoint** - shape, required
vs. optional fields, data types, field-level constraints - with **no business
logic**. They are **schema-driven**: Pesto builds requests from a schema and
validates responses against it, including a negative case for each declared
violation.

- **Location:** `tests/contract/`
- **Organized by:** the API surface. REST mirrors the URL path; RPC mirrors the
  method name.

```
GET  /users           ->  tests/contract/users/get/
POST /users           ->  tests/contract/users/post/
GET  /users/{id}      ->  tests/contract/users/get_by_id/
UserService.GetUser   ->  tests/contract/userservice/getuser/
```

### Schema language: JSON Schema + a thin Pesto envelope

Field shapes are described with **standard JSON Schema (draft 2020-12)** - a real
spec with mature Go validators (e.g. `santhosh-tekuri/jsonschema`), so we don't
invent a field-validation language. JSON Schema covers required/optional, data
types, formats, and constraints (`enum`, `minimum`, `pattern`,
`additionalProperties`, nested objects/arrays) directly.

JSON Schema validates *a JSON document*, not an HTTP exchange, so Pesto wraps it in
a small **envelope** that adds the parts JSON Schema has no concept of: HTTP
method/path/query/headers/status, which schema is request vs. response, and the
response expected when a request violates the schema.

```yaml
# tests/contract/users/post/contract.yaml   (the Pesto envelope)
request:
  body: { $ref: ./request.schema.json }      # standard JSON Schema
response:
  success:
    status: 201
    body: { $ref: ./response.schema.json }
  on_violation:
    status: 422                               # default for any derived violation
```

```jsonc
// request.schema.json - plain JSON Schema, nothing Pesto-specific
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name", "email"],
  "additionalProperties": false,
  "properties": {
    "name":  { "type": "string",  "minLength": 1 },
    "email": { "type": "string",  "format": "email" },
    "age":   { "type": "integer", "minimum": 0 }
  }
}
```

### Violations are derived from the schema, not hand-listed

Because the schema is standard JSON Schema, Pesto can **mechanically derive** the
negative cases instead of asking the author to enumerate them:

| Schema keyword | Generated violation |
| --- | --- |
| `required` | omit each required field |
| `type` | send the wrong type |
| `enum` | send a value outside the set |
| `minimum` / `maxLength` / `pattern` | send a boundary/format-violating value |
| `additionalProperties: false` | send an unexpected extra field |

From one schema Pesto derives:

1. a **happy-path request** (valid values for every field) → expect `success`;
2. **negative variants**, one per derived violation → expect the `on_violation`
   response (the envelope default, with optional per-field overrides).

### Happy-path values come from the schema's own examples (decided, drafted)

Constraint-based value generation - inventing a string that satisfies `pattern`,
a valid `format: email`, a number inside `minimum`/`maximum` - is a project of its
own (it's the core of every property-based-testing library) and is explicitly
**out of scope for v1**; it moves to later/maybe. Instead, the happy-path request
takes each field's value from the schema itself, in priority order:

1. `const`
2. `examples[0]`
3. `default`
4. for `enum`: the first member

A required field with none of these fails **when the contract case is built**,
with an error telling the author which field needs an example - Pesto never
silently invents a value. This keeps generated requests predictable and makes the
schema self-documenting.

Values the schema cannot know - a valid `{id}` path param, an auth token - come
from the environment (its static test data / `auth`), not from created data; see
the v1 fixtures split in [design-fixtures.md](design-fixtures.md).

### Body values from static data: `request.data` (decided, drafted)

Some body fields must hold a value that exists in the target environment - an
`account_id` the card is attached to, a `currency` the tenant has enabled. A
schema `examples` value cannot express that: it is the same in every environment,
so the contract would only pass where account `42` happens to exist. These fields
read the environment's static test data, exactly like `path_params`:

```yaml
# contract.yaml
path: /users/{id}/cards
path_params:
  id: users.default.id
request:
  body: request.schema.json
  data:
    account_id: accounts.default.id     # body field -> data reference
    holder.user_id: users.default.id    # nested field, dotted path
```

```yaml
# environments/dev/data/accounts.yaml
default:
  id: 42
```

- **Keys are body field paths**, dotted for nested objects - the same names the
  violation cases use (`violation/type/holder.user_id`). Each must name a
  property the request schema declares (following nested objects and local
  `$ref`s); an unknown field is an envelope error, so a typo cannot silently
  send nothing. Array items are not addressable, matching the v1 violation
  scope.
- **Data overrides the schema's values.** The happy-path body is built from the
  schema as today, then each listed field is set from data. A listed field needs
  no `examples`; a parent object that would otherwise be omitted is included so
  the field has somewhere to live.
- **The schema decides the JSON type.** Data values are text, so each is
  converted to its field's `type`: `integer`, `number`, `boolean`, or `string`
  (also when `type` is absent or lists `string`). `"42"` for an integer field is
  sent as `42`. A value that does not convert fails the contract before any
  request, naming the field, the reference, and the value. The filled body is
  then validated against the schema like any happy-path body, so data that
  breaks a constraint (`minLength`, `enum`) is reported as such.
- **Violations start from the filled body.** Every violation changes one field
  of the valid body, so it has to be the body with data in it - otherwise a
  violation of `limit` would also send an invalid `account_id` and prove
  nothing. A listed field gets its own violations as usual
  (`violation/type/account_id`). Consequence: violations are derived when the
  contract runs, after data is resolved, not when it is loaded.
- **Resolved up front.** Every `path_params` and `request.data` reference is
  resolved before the first request; a missing one fails the contract, naming
  the field and the environment's file.

Rejected:

- **References inside the schema** (`"examples": ["${accounts.default.id}"]`).
  The schema must stay plain JSON Schema that other tools - and OpenAPI seeding
  - can read; environment wiring belongs in the envelope.
- **A Go hook for computed or created values** (`contract.Run(t, ...,
  contract.Set("account_id", acc.ID))`). A contract test checks one operation
  against data that already exists; a flow that must first create an account
  is an E2E test. Contract tests never create data.

Not covered: query parameters and headers - the envelope has neither yet; when
it gains them, they read data the same way.

**Implemented (2026-10-22):** as drafted. `load` builds the body template with
a placeholder per `request.data` field (so those fields need no example) and
checks each field against the schema; `fill` converts and sets the values,
validates the body, and derives the violations, and `contract.Run` calls it
after reading the data - still before any request. Without `request.data` the
cases are built at load time as before. Promoted to
[reference/contract.md](../../reference/contract.md#body-fields-from-static-data).

### Relationship to OpenAPI

OpenAPI and JSON Schema are not competing choices: **OpenAPI = JSON Schema (body
shapes) + the HTTP envelope (paths/methods/responses)**, and OpenAPI 3.1's schema
objects *are* JSON Schema 2020-12. So if a team has an OpenAPI spec, Pesto can seed
both the envelope and the schemas from it; if not, the author writes a JSON Schema
plus the small envelope by hand. Same validation core either way.

## Phase 4 decisions

Taken when implementing REST contracts (2026-10-20), after challenging the
first idea - an interactive, question-by-question `generate contract` that also
asked for the API type, a schema path, and a client:

- **Generated bundle + runtime engine, not generated cases.** `generate contract`
  writes `contract.yaml`, starter schemas, and a one-line `contract_test.go`
  calling `contract.Run`; the engine derives every case from the schemas each run.
  Emitting one Go test per violation was rejected: it goes stale the moment the
  schema changes and needs a regenerate/merge story. The visible test file keeps
  discovery on the normal transform path - no transform changes.
- **Arguments, not prompts.** `pesto generate contract POST /users/{id}/cards`
  already carries the API type, method, and path; a wizard would ask for what
  the arguments say and cannot be scripted. Matches design-cli's "no interactive
  prompts".
- **Schemas live in the operation folder**, scaffolded as starters - at
  generate time the schema usually does not exist yet, and there are two
  (request and response; GET has no request body). Importing an existing schema
  or seeding from OpenAPI stays open.
- **No client choice.** An environment has one `base_url`, and a typed client
  would hide the very fields the violation cases break; the engine sends raw
  requests through `t.Client.Do`. Multiple services per environment would be a
  config change (`services:`), deferred until a real need.
- **REST only in v1.** "RPC" could mean JSON-RPC, an in-house HTTP RPC, or gRPC -
  each with a different envelope and error convention (JSON-RPC errors often
  come back as HTTP 200). Deferred until a concrete RPC target exists; see
  [design-protocols.md](design-protocols.md).
- **Folder mapping:** each literal segment normalized, each `{param}` a
  `by_<param>` folder, the lower-cased method last (and the package name):
  `POST /users/{id}/cards` → `users/by_id/cards/post/`. The drafted
  `get_by_id` form did not generalize to mid-path parameters.
- **Path params from static test data:** `path_params: {id: users.default.id}`
  names a data reference in `environments/<env>/data/`; the generator suggests
  `<previous segment>.default.<param>`. Read via `t.Data`. (First cut used
  environment `variables` and `t.Var`; superseded before release - see
  [design-fixtures.md](design-fixtures.md#decided-drafted-static-vs-dynamic-test-data).)
- **Envelope:** plain relative schema paths (`body: request.schema.json`), not
  `{ $ref: ... }`; strict YAML (unknown keys are errors); a single
  `on_violation.status`.
- **Derived violations (v1):** `required`, `type`, `enum`, numeric bounds,
  length bounds, `additionalProperties: false`, recursing into nested objects
  and local `$ref`s. Each candidate is validated against the schema and dropped
  if still valid. Not derived yet: `pattern`, `format`, array items.
- **Validator:** `santhosh-tekuri/jsonschema/v6` (2020-12, format assertion),
  imported only by the `contract` package so the root runtime stays on yaml.v3.
- **Runtime additions:** `T.Run(name, func(*T))` (shadows `testing.T.Run`,
  still reachable as `t.T.Run`), `T.Data`, `T.LookupData`, `WithData`,
  `Client.Do`.

## Open questions

### Schema format & source

**Decided:** body shapes use **JSON Schema 2020-12**, wrapped in a thin Pesto
envelope for the HTTP bits and violation expectations; seedable from OpenAPI 3.1.
Remaining open:

- ~~**Envelope format.**~~ *Decided:* YAML envelope + separate schema files,
  one folder per operation (see Phase 4 decisions). Original question: YAML envelope referencing `*.schema.json` files (shown
  above) vs. a single file embedding the schemas inline. One file per operation or
  request/response split?
- **Violation → response mapping.** Is a single `on_violation.status` enough, or do
  we need per-field / per-violation expected responses (e.g. missing field → 422,
  bad auth → 401)? How is that expressed without re-inventing what JSON Schema
  already says?
- **OpenAPI version.** 3.1 aligns with JSON Schema 2020-12; 3.0 uses a divergent
  schema dialect. Support only 3.1 for seeding, or convert 3.0?
- **Where do valid example values come from?** *Decided (drafted):* from the
  schema's own `const`/`examples`/`default` only, failing loudly otherwise -
  constraint-based generation is out of scope for v1. See "Happy-path values"
  above.
- ~~**Validator dependency.**~~ *Decided:* `santhosh-tekuri/jsonschema/v6`. Original: Confirm `santhosh-tekuri/jsonschema` (or similar)
  covers draft 2020-12 incl. `format` assertions, which are optional in the spec.

### Generated Go vs. interpreted at runtime

- *Decided:* schema interpreted at run time, with a visible generated test
  file calling the engine (see Phase 4 decisions). Original question:
- Does `tests/contract/users/get/` hold a **schema file that the contract engine
  interprets at run time**, or does `pesto generate` emit a **Go test** from the
  schema? Interpreted = less code, schema is the source of truth; generated =
  visible, debuggable, editable Go. Possibly: schema is source of truth, transform
  produces the test (consistent with [design-test-context.md](../shipped/design-test-context.md)).

### Naming & disambiguation

- **File vs. folder per operation.** `tests/contract/users/TestGetUsers.go` vs.
  `tests/contract/users/get/`. Recommending the **folder-per-operation** form: an
  endpoint's contract is a bundle (schema + generated cases + local overrides), and
  method-named subfolders (`get/`, `post/`) disambiguate methods on the same path
  cleanly.
- *Decided:* `{param}` → `by_<param>` folder (see Phase 4 decisions).
  Original: **Path params & casing.** `GET /users/{id}` → `get_by_id/`? How are `{params}`,
  hyphens, and mixed case mapped to folder names deterministically?
- **RPC mapping** (deferred with RPC contracts). `Service.Method` → `service/method/`. Confirm the casing rule
  and how streaming RPCs are represented (ties into
  [design-protocols.md](design-protocols.md)).

### Scope & semantics

- **Auth/setup for contract tests.** *Decided (drafted):* auth comes from the
  environment's `auth` config and referenced resources from the environment's
  static test data - contract tests never create data in v1. See the v1 scope split in
  [design-fixtures.md](design-fixtures.md).
- **Where does the contract boundary stop?** Status + body shape + types, yes.
  Cross-field rules ("end_date after start_date")? Probably E2E, not contract.
- **Response validation depth.** Required fields and types for sure; full schema
  (nested objects, arrays, enums)? JSON Schema / OpenAPI component validation reuse?

## CLI implications

- `pesto generate contract <METHOD> <path>` (implemented, blank starter schemas;
  OpenAPI seeding still open) and
  `pesto generate e2e <domain>/<name>`. The level becomes part of the generate
  command - see [design-cli.md](design-cli.md).

## Future levels

The level concept is open-ended: integration, smoke, or load (out of scope for
now, see [overview.md](../../overview.md)) could be added without changing the model -
each gets a `tests/<level>/` root and its own structure rules.
