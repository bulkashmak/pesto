// Package cli holds the shared state of the pesto CLI. The command registry
// lives in internal/cli/commands and the command implementations in
// internal/cli/handlers; cmd/pesto wires them together.
package cli

import "io"

// State is passed to every command handler. It carries what handlers share
// instead of reaching for globals, so commands can be tested in-process.
type State struct {
	// Version is the pesto CLI version.
	Version string
	// Stdout receives regular command output.
	Stdout io.Writer
	// Stderr receives diagnostics.
	Stderr io.Writer
}
