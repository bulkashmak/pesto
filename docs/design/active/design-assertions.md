# Design note: assertions

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

The assertion API is the most-used surface in any test, so its
ergonomics matter a lot.

## Goals

- **Readable top-to-bottom.** A QA engineer should read a test like a checklist.
- **Good failure messages.** On failure, say what was expected, what was received,
  and where (path/field).
- **Soft vs. hard failures.** Sometimes you want to stop at the first failure;
  sometimes you want all mismatches reported at once.
- **Not a walled garden.** Pesto's fluent API is for convenience, never a
  requirement. A user must always be able to drop down to a plain struct and
  assert with any library they prefer.

## Two layers

### 1. Fluent `Expect()` - for quick inline checks

```go
resp.Expect().
    Status(201).
    Header("Content-Type").Contains("application/json").
    JSON("id").Exists().
    JSON("name").Equals("Bob").
    JSON("roles").Length(2)
```

### 2. Decode to a struct - the escape hatch

Decode the body into a user-defined struct, then assert with **any** library
(testify, gotest.tools, go-cmp, stdlib). This is a first-class, supported path -
not a fallback.

```go
var user User
resp.Decode(&user)   // unmarshal into struct; fails the test on decode error
require.Equal(t, "Bob", user.Name)
```

## Open questions

- **Fluent chain vs. explicit assert calls.** A chain reads nicely but can be
  awkward when an assertion needs to capture a value for later use. Do we offer
  both a chain and a `value := resp.JSON("id").String()` extraction form?
- **JSON path syntax.** Dotted path (`data.items.0.id`), JSONPath, or both?
- **Schema assertions.** Validate a response against a JSON Schema / OpenAPI
  component? How is the schema referenced?
- **Soft assertions.** How does a test opt into "collect all failures"? Per-test,
  per-block, or per-assertion?
- **Custom matchers.** How does a project add domain-specific matchers and keep
  the chain ergonomic?
- **Eventual consistency.** Do assertions get a built-in `Eventually(...)`
  poll/retry wrapper, or is that a separate helper?
- **`Decode` signature.** Fail-the-test-on-error (`resp.Decode(&v)`, uses the
  test's `*testing.T`) vs. error-returning (`err := resp.Decode(&v)`) for callers
  who want to assert on malformed bodies - offer both? Plus a typed-generic form,
  `user := pesto.Decode[User](resp)`?
- **Partial / repeated decode.** Decode a sub-path into a struct
  (`resp.DecodePath("data.user", &u)`)? Is the body re-readable so `Expect()` and
  `Decode` can both be used on the same response?

## Non-goals

- A general-purpose assertion library for unit tests - this is response-shaped and
  HTTP-aware on purpose.
