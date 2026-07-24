// Package pesto is a structured, fast black-box backend test-automation
// framework for QA engineers.
//
// A Pesto test is an ordinary Go test. It wraps *testing.T in a [T] that carries
// a fluent HTTP [Client] and response assertions, while still exposing the
// underlying *testing.T so any assertion library (stdlib, testify, gotest.tools,
// go-cmp) works unchanged:
//
//	func TestCreateUser(t *testing.T) {
//		pt := pesto.New(t, pesto.WithBaseURL("http://localhost:8080"))
//
//		resp := pt.Client.POST("/users").
//			JSON(map[string]any{"name": "Bob"}).
//			Send()
//
//		resp.Expect().
//			Status(201).
//			JSON("id").Exists().
//			JSON("name").Equals("Bob")
//	}
//
// Pesto's assertions are a convenience, never a requirement: a test can always
// [Response.Decode] the body into a struct and assert with any library it prefers.
package pesto
