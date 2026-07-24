# Design note: authoring model

- **Status:** active
- **Owner:** bulat
- **Promoted to:** —

> Direction: decided for v1, revisit later.

## Decision

For the first version, **tests are Go code**, scaffolded by the `pesto generate`
CLI. A QA engineer edits real `*_test.go` files; the CLI removes the boilerplate.

```go
func TestCreateUser(t *pesto.T) {
    resp := t.Client.POST("/users").
        JSON(map[string]any{"name": "Bob"}).
        Send()
    resp.Expect().Status(201).JSON("id").Exists()
}
```

## Why Go code (not YAML/DSL) for v1

- **Power and escape hatches.** Real control flow, helpers, and the full Go
  ecosystem are available when a test gets complicated.
- **Type safety.** Mistakes surface at compile time.
- **One execution model.** Everything is just Go, so there's no second engine to
  build, debug, and document.
- **Codegen closes the ergonomics gap.** The main downside of "QA must write Go"
  is boilerplate; `pesto generate` is exactly the answer to that.

## The trade-off we're accepting

Requiring Go raises the barrier for QA engineers who don't code. We accept this
for v1 in exchange for power and a single, simple architecture.

## What might change later

A declarative layer (YAML/DSL) could sit **on top of** the Go runtime for the
common, simple cases - compiling down to the same execution model rather than
being a parallel engine. This stays a "later/maybe" item (see
[roadmap.md](../roadmap.md)) and should only be built if real usage shows the Go
barrier is blocking adoption.

Open questions if we ever revisit:

- Where's the line between "simple enough for YAML" and "needs Go"?
- Can a YAML test drop into Go (escape hatch) without a rewrite?
- Does codegen go the other way too - scaffold Go *from* a YAML description?
