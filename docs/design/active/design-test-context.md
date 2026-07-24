# Design note: test context & the transform

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

> Direction: authoring model decided; mechanism open.

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

## Open questions (the mechanism)

- **AST rewrite vs. build tags.** Rewrite the source AST in a build dir, or keep
  authored files behind a build tag and emit a tagged companion file with the
  wrappers? Which keeps line numbers / stack traces pointing at the authored file?
- **Where do generated files live?** Temp dir (clean, but harder to inspect) vs.
  `*_gen_test.go` on disk (inspectable, but clutters the repo / needs gitignore).
- **`go vet` noise.** A `func TestX(t *pesto.T)` in source triggers vet's
  "wrong signature for Test" warning. Rename authored funcs during transform, hide
  them behind a build tag, or suppress vet?
- **Stack traces & editor "run test".** How do failures map back to the authored
  line? Does the IDE gutter "run test" affordance still work, or only `pesto run`?
- **`pesto.New(t)` escape hatch.** Keep a public `pesto.New(t *testing.T) *pesto.T`
  so the runtime is usable from a plain `*testing.T` test (useful for dogfooding in
  Phase 1, before the transform exists, and as a manual escape hatch)?

## Related

- Entry point and flags: [design-cli.md](design-cli.md)
- Build order: [roadmap.md](../roadmap.md) (transform lands in Phase 2)
