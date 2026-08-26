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
  (already built); referenced values (a valid `{id}`, a tenant) come from
  environment `variables`; and any helper that *does* create data registers its
  teardown with `t.Cleanup` immediately after the create succeeds. No `t.Fixtures`
  accessor, no factories, no namespacing.
- **Phase 5 ships the real thing.** `t.Fixtures` on `pesto.T`, factories/builders,
  isolation namespacing, suite-level lifetimes. Phase 5 is also the gate for
  parallel-by-default (see [design-parallelism.md](design-parallelism.md)):
  `t.Parallel()` injection stays off until isolation makes it safe.
- **Teardown base mechanism is `t.Cleanup`.** It runs on failure and on panic
  within the test goroutine, which covers the realistic cases; the open questions
  below narrow to the gaps (un-captured IDs, process death).

## Open questions

- **Creation strategy.** Do fixtures create data *through the API under test*
  (realistic, but couples setup to the same endpoints being tested) or through a
  side channel (seed endpoint, DB, admin API)?
- **Teardown gaps beyond `t.Cleanup`.** Data created but whose ID we didn't
  capture; the process dying mid-run. Orphan-sweeping? Run-scoped prefixes so
  leftovers are identifiable?
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
