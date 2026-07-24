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

## Open questions

- **Creation strategy.** Do fixtures create data *through the API under test*
  (realistic, but couples setup to the same endpoints being tested) or through a
  side channel (seed endpoint, DB, admin API)?
- **Teardown reliability.** If a test panics mid-way, how do we guarantee cleanup?
  Lean on `t.Cleanup`? What about data created but whose ID we didn't capture?
- **Isolation under parallelism.** Namespacing per test (unique prefixes/tenants)?
  Shared read-only fixtures vs. per-test mutable fixtures?
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
