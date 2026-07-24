# Design note: assertions

- **Status:** active
- **Owner:** bulat
- **Promoted to:** -

The assertion API is the most-used surface in any test, so its ergonomics matter a
lot. But it is **optional, never mandatory**: engineers are never forced to use it.
They can always decode the response into a model and assert with the standard
library (or testify, gotest.tools, go-cmp, ...). Pesto's assertions are a
convenience, not a lock-in. This is the load-bearing principle of the note.

## Goals

- **Not a walled garden.** The fluent API is for convenience, never a requirement.
  A user must always be able to decode the response to a plain struct and assert
  with any library they prefer - stdlib included. See the escape hatch below.
- **Readable top-to-bottom.** A QA engineer should read a test like a checklist.
- **Good failure messages.** On failure, say what was expected, what was received,
  and where (path/field).
- **Soft vs. hard failures.** Sometimes you want to stop at the first failure;
  sometimes you want all mismatches reported at once.

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

For this to work, whatever Pesto hands the test must expose the underlying
`*testing.T` (e.g. an embedded field), so `require.Equal(t, ...)` and any other
library can be called directly. That same `*testing.T` is what the
parallel/isolation transform operates on - which is why the choice of assertion
style is **independent** of parallel execution: a test can decode to a struct and
use stdlib assertions and still get `t.Parallel()` injected. See
[design-parallelism.md](design-parallelism.md) and
[design-test-context.md](design-test-context.md).

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
