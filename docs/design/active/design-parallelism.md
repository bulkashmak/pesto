# Design note: parallel execution

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

Parallel execution is the core lever that keeps Pesto fast. This note describes
how Pesto runs tests concurrently and the key decision: parallelism is applied
**dynamically at transform time**, not written by hand into each test.

## The problem

Real suites are large - hundreds of tests. Run sequentially, a suite can take
anywhere from ~10 minutes to over an hour. That is too slow for something QA
engineers and CI run many times a day. Parallel execution is the standard answer:
run independent tests concurrently and the wall-clock time collapses toward
`sequential_time / concurrency`.

Pesto's tests are black-box API tests, so they are **I/O-bound** - almost all of
their time is spent waiting on an HTTP response, not burning CPU. Go handles this
case exceptionally well: a goroutine blocked on the network is nearly free, so
concurrency far above the CPU count is both safe and correct here.

## Why the standard library can't just flip a switch

Go's `testing` package makes parallelism an **opt-in, per-test** decision: a test
runs concurrently only if it calls `t.Parallel()`. There is no flag or environment
variable that makes every test in a package parallel. The `-parallel N` flag only
sets the *cap* on how many already-opted-in tests run at once; with no
`t.Parallel()` calls, it does nothing within a package.

So "parallel by default" cannot come from a stdlib switch. It has to be applied.

## The decision: dynamic injection at transform time

Pesto already transforms authored tests into standard Go tests before handing them
to `go test` (see [design-test-context.md](design-test-context.md)). That transform
is the injection point. The QA engineer never writes `t.Parallel()`; Pesto adds or
omits it based on a single **parallel option**.

- **Parallel enabled** → the transform injects `t.Parallel()` (plus per-test
  isolation setup) into **every** generated test. The whole suite runs concurrently.
- **Parallel disabled** → the transform injects nothing. Every test runs
  **sequentially**, in order.

The mode is a property of the run, decided by one option - not a per-test edit.

```go
// what the QA engineer authors:
func TestCreateUser(t *pesto.T) { ... }

// what `pesto run` emits when parallel is ENABLED:
func TestCreateUser(t *testing.T) {
    pt := pesto.New(t)
    pt.T.Parallel()          // injected only in parallel mode
    pt.Isolate()             // injected so parallel is SAFE - see design-fixtures.md
    // ... transformed body
}

// what `pesto run` emits when parallel is DISABLED:
func TestCreateUser(t *testing.T) {
    pt := pesto.New(t)
    // no t.Parallel() - runs sequentially
    // ... transformed body
}
```

This is what "dynamic" means: the *same* authored test compiles to a parallel or a
sequential test depending purely on the mode, with no change to the source the
engineer wrote.

## Safety: parallel is only correct with isolation

Injecting `t.Parallel()` blindly turns a slow-but-green suite into a
fast-but-flaky one. Parallel tests must not interfere: each must own its data
(unique users, tenants, resource IDs) so no two collide on shared backend state.

Therefore parallel mode couples the `t.Parallel()` injection with **isolation
setup** in the same transform. Parallelism and fixtures are effectively one
feature - see [design-fixtures.md](design-fixtures.md). Isolation is what makes
"parallel by default" trustworthy rather than a source of flakiness.

## Degree of parallelism

Because the tests are I/O-bound, the concurrency cap can sit well above the CPU
count. Pesto passes an appropriate `-parallel` value to `go test` under the hood.
Package-level parallelism (`go test -p` across packages) also applies for free
given Pesto's package-per-endpoint/feature layout, and multiplies with in-package
parallelism.

## Open questions

- **Where the option lives.** Global config (`pesto.yaml`), a `pesto run --parallel`
  / `--no-parallel` flag, an environment variable, or several with a clear
  precedence? A run-time flag is the most ergonomic for the CI-vs-local split.
- **Default mode.** Parallel-on-by-default matches the "fast by default" promise,
  but only once isolation is solid. Until then, sequential may be the safer default.
- **Degree default.** What `-parallel` value ships by default, and is it
  configurable / auto-derived from the environment?
- **Opt-out granularity.** A test or suite that genuinely can't be isolated needs
  to stay serial even in parallel mode. Mechanism TBD - e.g. a `// pesto:serial`
  marker the transform respects, or a resource declaration (a lighter version of
  JUnit's `@ResourceLock`). See the escape-hatch discussion in
  [design-fixtures.md](design-fixtures.md).
- **CLI surface.** How the flag/config is exposed and documented - see
  [design-cli.md](design-cli.md).

## Cross-references

- [design-test-context.md](design-test-context.md) - the transform that injects
  the parallel/isolation code
- [design-fixtures.md](design-fixtures.md) - the isolation that makes parallel safe
- [design-cli.md](design-cli.md) - how the parallel option is exposed
- [roadmap.md](../roadmap.md) - build order
