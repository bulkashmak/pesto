package pesto

import "net/http"

// Auth applies authentication to an outgoing request. Built-in implementations are
// created with [BearerAuth] and [BasicAuth]; custom schemes can implement the
// interface directly.
type Auth interface {
	// apply sets the authentication on the request (typically an Authorization
	// header). It runs only when the request has no explicit Authorization header,
	// so a per-request Header call always wins.
	apply(*http.Request)
}

type bearerAuth struct{ token string }

func (b bearerAuth) apply(r *http.Request) {
	r.Header.Set("Authorization", "Bearer "+b.token)
}

type basicAuth struct{ username, password string }

func (a basicAuth) apply(r *http.Request) {
	r.SetBasicAuth(a.username, a.password)
}

// BearerAuth authenticates requests with an "Authorization: Bearer <token>" header.
func BearerAuth(token string) Auth { return bearerAuth{token: token} }

// BasicAuth authenticates requests with HTTP Basic credentials.
func BasicAuth(username, password string) Auth {
	return basicAuth{username: username, password: password}
}
