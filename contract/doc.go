// Package contract runs schema-driven contract tests for a single REST
// endpoint.
//
// A contract is a folder holding a contract.yaml envelope (method, path, expected
// statuses) and JSON Schema (draft 2020-12) files for the request and response
// bodies. `pesto generate contract` scaffolds one, with a test that calls [Run]:
//
//	func TestContract(t *pesto.T) { contract.Run(t, "contract.yaml") }
//
// From the schemas, Run derives the cases when the test runs, so they follow the
// schema as it changes:
//
//   - happy_path: a request built from the schema's own const, examples,
//     default, or enum values, expecting the success status and a response body
//     that matches the response schema;
//   - one violation case per derived schema violation (a missing required field,
//     a wrong type, a value outside an enum or a numeric or length bound, an
//     unexpected field), each expecting the on_violation status.
//
// Pesto never invents values: a required field with no const, examples, default,
// or enum fails the contract with a message naming that field.
package contract
