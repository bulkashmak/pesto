# AGENTS.md

Baseline context for AI coding agents working in this repository.

**What is this repo**
A structured, fast black-box backend test-automation framework for QA engineers
Built in Go

## Documentation

Docs live in three layers, each with a different audience and lifecycle. Keep them
separate - never mix working notes into published docs

**1. In-source (godoc)** - the API reference, for anyone reading the code

- `doc.go` per package with a `// Package ...` comment
- Doc comment on every exported identifier, starting with its name
- Prefer runnable `Example` functions - they're compiled and tested, so they
  can't drift. Published docs link to this layer, they don't duplicate it

**2. End-user docs - `docs/`** - for people using Pesto. Published and stable;
describes only what's true right now. Content lands here only once implemented

- `docs/overview.md` - the problem and philosophy
- `docs/architecture.md` - how Pesto is built
- `docs/concepts.md` - the building blocks
- `docs/guides/` - task-oriented how-tos ("how do I write a contract test")
- `docs/reference/` - dry lookups: CLI commands, config keys, env format

**3. Working/design docs - `docs/design/`** - for the team and maintainer.
Iterative, user-story/task style. Split by lifecycle state:

- `docs/design/active/` - being iterated on or implemented
- `docs/design/parked/` - useful but postponed; a backlog to revisit, not a
  graveyard
- `docs/design/shipped/` - implemented and promoted; kept as a decision record

Every design note carries a status header:

```markdown
- **Status:** active | parked | shipped
- **Owner:** <name>
- **Promoted to:** docs/concepts.md#assertions   (filled once shipped)
```

**The pipeline (3 → 2)**: a note starts in `docs/design/active/`, gets iterated,
then implemented. On ship, extract the durable user-facing parts into `docs/`
(that's promotion - not a file move), move the note to `docs/design/shipped/`, and
mark it `Status: shipped`. The messy *why* (debate, rejected alternatives) stays in
the design note; the settled *what/how* lives in the end-user doc. A design note
and its end-user doc are two different documents, not one document over time
