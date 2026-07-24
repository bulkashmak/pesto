# Design note: test levels

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

> Direction: decided; mechanics open.

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

### Relationship to OpenAPI

OpenAPI and JSON Schema are not competing choices: **OpenAPI = JSON Schema (body
shapes) + the HTTP envelope (paths/methods/responses)**, and OpenAPI 3.1's schema
objects *are* JSON Schema 2020-12. So if a team has an OpenAPI spec, Pesto can seed
both the envelope and the schemas from it; if not, the author writes a JSON Schema
plus the small envelope by hand. Same validation core either way.

## Open questions

### Schema format & source

**Decided:** body shapes use **JSON Schema 2020-12**, wrapped in a thin Pesto
envelope for the HTTP bits and violation expectations; seedable from OpenAPI 3.1.
Remaining open:

- **Envelope format.** YAML envelope referencing `*.schema.json` files (shown
  above) vs. a single file embedding the schemas inline. One file per operation or
  request/response split?
- **Violation → response mapping.** Is a single `on_violation.status` enough, or do
  we need per-field / per-violation expected responses (e.g. missing field → 422,
  bad auth → 401)? How is that expressed without re-inventing what JSON Schema
  already says?
- **OpenAPI version.** 3.1 aligns with JSON Schema 2020-12; 3.0 uses a divergent
  schema dialect. Support only 3.1 for seeding, or convert 3.0?
- **Where do valid example values come from?** A happy-path request needs real-ish
  values (and possibly a real referenced resource, e.g. a valid `{id}`). From JSON
  Schema `examples`/`default`/`const`, a fixture, or a constraint-based generator?
- **Validator dependency.** Confirm `santhosh-tekuri/jsonschema` (or similar)
  covers draft 2020-12 incl. `format` assertions, which are optional in the spec.

### Generated Go vs. interpreted at runtime

- Does `tests/contract/users/get/` hold a **schema file that the contract engine
  interprets at run time**, or does `pesto generate` emit a **Go test** from the
  schema? Interpreted = less code, schema is the source of truth; generated =
  visible, debuggable, editable Go. Possibly: schema is source of truth, transform
  produces the test (consistent with [design-test-context.md](design-test-context.md)).

### Naming & disambiguation

- **File vs. folder per operation.** `tests/contract/users/TestGetUsers.go` vs.
  `tests/contract/users/get/`. Recommending the **folder-per-operation** form: an
  endpoint's contract is a bundle (schema + generated cases + local overrides), and
  method-named subfolders (`get/`, `post/`) disambiguate methods on the same path
  cleanly.
- **Path params & casing.** `GET /users/{id}` → `get_by_id/`? How are `{params}`,
  hyphens, and mixed case mapped to folder names deterministically?
- **RPC mapping.** `Service.Method` → `service/method/`. Confirm the casing rule
  and how streaming RPCs are represented (ties into
  [design-protocols.md](design-protocols.md)).

### Scope & semantics

- **Auth/setup for contract tests.** "No business logic" still needs a valid auth
  token and maybe a referenced resource to hit a real endpoint. How is that
  supplied without turning a contract test into an E2E test?
- **Where does the contract boundary stop?** Status + body shape + types, yes.
  Cross-field rules ("end_date after start_date")? Probably E2E, not contract.
- **Response validation depth.** Required fields and types for sure; full schema
  (nested objects, arrays, enums)? JSON Schema / OpenAPI component validation reuse?

## CLI implications

- `pesto generate contract <path>` (from an OpenAPI op or a blank schema) and
  `pesto generate e2e <domain>/<name>`. The level becomes part of the generate
  command - see [design-cli.md](design-cli.md).

## Future levels

The level concept is open-ended: integration, smoke, or load (out of scope for
now, see [overview.md](../../overview.md)) could be added without changing the model -
each gets a `tests/<level>/` root and its own structure rules.
