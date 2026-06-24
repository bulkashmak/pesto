# Overview

Pesto is a backend test-automation framework for QA engineers. This document
explains the problem it solves and the philosophy behind its design. For how it's
built, see [architecture.md](architecture.md); for the building blocks you work
with, see [concepts.md](concepts.md)

## The problem

QA engineers automate test cases against backend APIs. In practice, every team
builds this automation differently, and every new project starts from zero:

- There's no shared project structure, so layout, naming, and conventions are
  reinvented each time
- Onboarding is slow because the setup is bespoke to each project

The result: time spent on plumbing instead of on tests

## The audience

Pesto is made for black-box testing
It lives in a separate repo from the target app

The main target audience are QA engineers, but essentially any engineer
can use Pesto if they want to automate their black-box tests

## Philosophy

Pesto borrows three ideas from tools that already solved structure well, and one
from the choice of language

### 1. Speed and low overhead - from Go

Execution speed, start up time and efficiency are important for automated tests
Engineers and CI run tests frequently, every day, every hour
Nobody wants to wait 5 minutes just at start up when they run a single test

Go solves this problems:

- **Compiled, single static binary**
- **Low memory**
- **Parallel by default**. Goroutines make running large suites concurrently natural
- **Editor-agnostic**. No heavy IDE required. You can use any text editor with
  Go LSP support
- **Static typing**. Handling types in requests and responses is essential in
  testing

### 2. Structure from the start - like Vue

When you scaffold a Vue project, you get a complete, opinionated structure
immediately: you know where things go before you write a line of code. Pesto's
`pesto init` does the same for a test project - config, environments, clients,
tests, fixtures, and reports all have a defined home

### 3. Code generation - like Django

Django's `manage.py` lets you generate apps and boilerplate so you focus on logic.
Pesto's `pesto generate` scaffolds tests, API clients, and suites that follow the
project's conventions, so a QA engineer writes assertions - not setup

### 4. Convention over configuration

Following from the two above: there's one obvious way to lay out a Pesto project,
and the framework assumes it. Configuration exists for the things that genuinely
vary between teams (environments, base URLs, auth), not for re-deciding structure
on every project

## Non-goals

- **Not a load/performance tool.** Pesto is for functional API test automation,
  not benchmarking or load generation.
- **Not coupled to any backend language or framework.** It tests over the wire.
- **Not a no-code tool.** Tests are Go code (scaffolded for you). This is a
  deliberate trade-off favoring power and type-safety over a zero-code barrier.
  An accessibility layer may be reconsidered later - see
  [`wip/design-authoring.md`](wip/design-authoring.md)
