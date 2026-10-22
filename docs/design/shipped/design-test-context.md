# Design note: test context & the transform

- **Status:** shipped
- **Owner:** bulat
- **Promoted to:** docs/architecture.md (the transform), docs/concepts.md#pestot---the-test-context

> Direction: authoring model decided; **mechanism decided** (overlay + AST rewrite,
> see below).
>
> **Implemented (Phase 2):** `pesto.Run`, `internal/transform`, and the overlay-based
> `pesto run` are built and transform `*pesto.T` tests end to end.
>
> **Shipped.** The remaining `pesto.T` accessors are tracked elsewhere: `t.Env`
> (environment `variables`; shipped instead as `t.Data`, static test data) with
> the Phase 4 setup primitive and `t.Fixtures` in
> Phase 5 - both in [design-fixtures.md](../active/design-fixtures.md).

## Decision

Tests are authored with a clean, single-handle signature:

```go
func TestCreateUser(t *pesto.T) {
    resp := t.Client.POST("/users").JSON(payload).Send()
    resp.Expect().Status(201).JSON("id").Exists()
}
```

`pesto.T` embeds `*testing.T` and adds `Client`, `Env`, and `Fixtures`, all wired
to the selected environment.

## The constraint that forces a transform

Go's test runner only discovers `func TestXxx(t *testing.T)`. A `*pesto.T`
parameter is **not** a runnable test signature, so bare `go test` cannot run a
Pesto project. We accept this and make **`pesto run` the required entry point**.

Before compiling, `pesto run` performs a **codegen/transform**: each authored
`func TestXxx(t *pesto.T)` gets a generated standard wrapper:

```go
// generated
func TestCreateUser(t *testing.T) {
    pesto.Run(t, authoredTestCreateUser)   // builds *pesto.T, runs the body
}
```

Go's `testing` engine then runs the wrappers normally - Pesto reuses Go's
parallelism, sub-tests, and failure semantics; it does not reimplement a runner.

## Why this (option 1) over a visible wrapper

The alternative was to make authors write the wrapper themselves (e.g.
`func TestX(t *testing.T) { pesto.Run(t, func(t *pesto.T){…}) }`), which keeps
plain `go test` working. We chose the transform instead because:

- **Cleanest possible authoring surface.** The test signature is exactly
  `func TestX(t *pesto.T)` - no closure or `pesto.New(t)` ceremony in every file.
- **The wrapper is mechanical** - perfect work for codegen, nothing a human should
  hand-write per test.
- **Pesto already owns the run** (env resolution, reporting), so requiring
  `pesto run` is a small additional cost.

Trade-off accepted: **no bare `go test`**, and we take on a transform step that
must stay fast and debuggable.

## Mechanism (decided): overlay + AST rewrite

`pesto run` never edits files in the repo. It rewrites each `_test.go` in memory
and passes the rewritten versions to the compiler via `go test -overlay`.

For each package with authored tests, the transform produces two overlay entries:

1. **The authored file, rewritten in place.** Each `func TestX(t *pesto.T)` is
   renamed to an unexported body function (`func pestoBody_TestX(t *pesto.T)`) by a
   precise, byte-level edit at the function-name token, so **every other byte and
   line break is preserved**. A `//line <authored-path>:1` directive is prepended so
   compiler diagnostics and panics still point at the authored source.
2. **A generated wrappers file** added to the same package (a path that does not
   exist on disk, supplied entirely by the overlay). It imports `testing` + `pesto`
   and declares the real `func TestX(t *testing.T) { pesto.Run(t, pestoBody_TestX) }`
   for each test. Keeping wrappers in their own generated file means imports are
   clean and the authored file only ever gets the minimal rename.

This resolves the earlier open questions:

- **AST rewrite vs. build tags → AST rewrite via overlay.** No build-tag juggling,
  no duplicate-symbol clash (the authored `TestX` is renamed), authored files
  untouched on disk.
- **Where generated files live → nowhere on disk.** They exist only in a temp dir
  referenced by the overlay, so there is nothing to gitignore. A `--keep`/debug flag
  can dump them for inspection.
- **`go vet` noise → gone.** The compiler sees valid `func TestX(t *testing.T)`
  wrappers; `pesto run` controls the `go test` invocation (and can scope vet).
- **Stack traces → authored source**, via the `//line` directives.
- **Editor "run test" → only `pesto run`.** An IDE will not recognize
  `func TestX(t *pesto.T)` as a test; this is the accepted cost of the clean
  signature. `pesto run --filter` covers running a subset.

**`pesto.New(t)` stays public** as the escape hatch: the runtime is usable from a
plain `func TestX(t *testing.T)` without the transform (this is how Phase 1 is
tested today), and `pesto.Run(t, body)` is the entry point the generated wrappers
call.

## Related

- Entry point and flags: [design-cli.md](../active/design-cli.md)
- Build order: [roadmap.md](../roadmap.md) (transform lands in Phase 2)
