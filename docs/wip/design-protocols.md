# Design note: protocols (WIP)

Status: **open**. REST/HTTP+JSON is the committed first-class target. This note
tracks how (and whether) to extend beyond it.

## Committed: REST / HTTP+JSON

The `Client` and `Expect` APIs are designed around HTTP requests and JSON
responses first. This covers the common case for most backend teams and is the
only protocol in the initial scope.

## Candidates for later

### gRPC

Common in big-tech and microservice environments.

Open questions:

- How are tests written without hand-writing a ton of stub wiring? Generate a
  client from `.proto` files via `pesto generate client`?
- Does the assertion API generalize from "JSON path" to "proto message field", or
  do gRPC tests get their own `Expect` variant?
- Streaming RPCs - in scope or explicitly unsupported at first?

### GraphQL

Single-endpoint query APIs.

Open questions:

- Is GraphQL "just an HTTP+JSON POST" that the existing client already handles,
  with thin helpers for queries/variables - or does it deserve a dedicated client?
- Schema-aware assertions and typed queries from a GraphQL schema?

## The shared question

How much of the `Client` / `Expect` surface is **protocol-agnostic** vs.
**HTTP-specific**? Deciding this early (even while only building REST) keeps the
door open for gRPC/GraphQL without a rewrite. Candidate split:

- Protocol-agnostic: request lifecycle, response capture, assertion accumulation,
  reporting, environments, fixtures.
- Protocol-specific: how a request is built and sent, how a response is addressed
  (JSON path vs. proto field vs. GraphQL selection).
