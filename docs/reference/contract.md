# Contract reference

A contract test checks one REST operation against JSON Schema. It is a folder
under `tests/contract/` - scaffolded by
[`pesto generate contract`](cli.md#pesto-generate-contract-method-path) - holding:

| File | Purpose |
| --- | --- |
| `contract.yaml` | the envelope: method, path, path parameters, expected statuses |
| `request.schema.json` | JSON Schema (draft 2020-12) for the request body |
| `response.schema.json` | JSON Schema for the success response body |
| `contract_test.go` | `func TestContract(t *pesto.T) { contract.Run(t, "contract.yaml") }` |

`pesto run` runs it like any other test. The cases are derived from the schemas
each time the test runs, so editing a schema is enough - there is nothing to
regenerate.

## `contract.yaml`

```yaml
method: POST
path: /users/{id}/cards
path_params:
  id: users.default.id          # static data: environments/<env>/data/users.yaml
request:
  body: request.schema.json
  data:
    account_id: accounts.default.id   # body field filled from static data
response:
  success:
    status: 201
    body: response.schema.json
  on_violation:
    status: 422
```

| Key | Required | Meaning |
| --- | --- | --- |
| `method` | yes | GET, POST, PUT, PATCH or DELETE |
| `path` | yes | request path, relative to `base_url`; may contain `{param}` |
| `path_params` | one per `{param}` | maps each parameter to the static data **reference** that supplies its value |
| `request.body` | no | request schema file; without it the request has no body and no violation cases |
| `request.data` | no | maps body fields to static data references - see [Body fields from static data](#body-fields-from-static-data) |
| `response.success.status` | yes | status expected for the valid request |
| `response.success.body` | no | schema the success response body must match |
| `response.on_violation.status` | with `request.body` | status expected for every request that breaks the request schema |

Schema paths are relative to `contract.yaml`. Unknown keys are an error, so a
typo cannot silently drop an expectation.

Path parameter values come from the selected environment's
[static test data](config.md#static-test-data), never from created data:

```yaml
# environments/dev/data/users.yaml
default:
  id: 42
```

A missing reference fails the test before any request is sent.

### Body fields from static data

Some body fields must hold a value that exists in the environment - the account
a card is attached to, say. An `examples` value can't express that, because it
is the same in every environment. List such fields under `request.data`:

```yaml
request:
  body: request.schema.json
  data:
    account_id: accounts.default.id
    holder.user_id: users.default.id    # nested fields use dotted paths
```

- Each key is a body field path that the request schema declares (nested
  objects and local `$ref`s are followed). An unknown field, an object, or an
  array is an error when the contract loads. Array items can't be targeted.
- The value is converted to the field's schema `type`: `integer`, `number`,
  `boolean`, or `string` (also when `type` is absent or includes `string`). For
  `"type": "integer"`, data `42` is sent as `42`, not `"42"`. A value that does
  not convert fails the contract before any request:

  ```
  pesto: contract contract.yaml: request.data.account_id: data accounts.default.id: "abc" is not a valid integer
  ```

- A listed field needs no `examples`. If it sits in an optional object, the
  object is included so the field has somewhere to live.
- Like `path_params`, every reference is read before the first request, and a
  missing one names the field and the environment's file.

Values always come from data that already exists. A contract test never creates
data first - that is an E2E test.

## The cases

`contract.Run` runs these as subtests of `TestContract`.

### `happy_path`

A valid request body built from the request schema's own values, plus the
fields listed under [`request.data`](#body-fields-from-static-data), which are
filled from the environment's static data. For every other field the value is,
in order: `const`, the first of `examples`, `default`, the first of `enum`. An object with none of these is built from its properties: every required
property must produce a value, and optional ones are included when they can.
Arrays use their `items` example.

Pesto never invents values. A required field without a value source fails the
contract with the field named:

```
pesto: contract contract.yaml: build the happy-path request from request.schema.json:
  number: no example value: add "examples" (or const, default, enum) to its schema
```

The built body is checked against the schema before it is sent, so an example
that breaks its own schema is reported as such. The response must have
`success.status` and, when `success.body` is set, match that schema.

### `violation/<kind>/<field>`

One request per violation derived from the request schema. Each one changes a
single field of the valid body - including its `request.data` values - and each
must get `on_violation.status`.

| Kind | Derived from | The request... |
| --- | --- | --- |
| `required` | `required` | omits the field |
| `type` | `type` | sends a value of another JSON type |
| `enum` | `enum` | sends a value outside the set |
| `minimum` / `maximum` | `minimum`, `maximum`, `exclusiveMinimum`, `exclusiveMaximum` | sends a value just past the bound |
| `min_length` / `max_length` | `minLength`, `maxLength` | sends a string one character too short or too long |
| `additional_properties` | `additionalProperties: false` | adds an unknown field |

Fields are named by their dotted path (`holder.name`); `body` is the whole body.
Nested objects and local `$ref`s (`#/$defs/...`) are followed. A derived request
the schema still accepts - say, a wrong type when the field allows several - is
dropped, so every case is a real violation.

Not derived yet: `pattern`, `format`, and array-item violations. `format` is
still asserted when validating bodies.

A failing case shows what was sent and received:

```
--- FAIL: TestContract/violation/required/limit
    status = 201, want 422 for a request that breaks the schema (required/limit)
    request body:  {"number":"4111111111111111"}
    response body: {"id": 7}
```

Run one case with `go test`'s subtest filter, e.g.
`pesto run -run 'TestContract/violation/required' ./tests/contract/...`.
