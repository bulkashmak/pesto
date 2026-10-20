// Package commands is the pesto CLI command registry: commands are registered
// by name with a handler and help metadata, then dispatched by Run.
package commands

import (
	"errors"
	"fmt"

	"github.com/bulkashmak/pesto/internal/cli"
)

// Command is one invocation: the command name and the arguments after it.
type Command struct {
	Name string
	Args []string
}

// Handler implements a command.
type Handler func(*cli.State, Command) error

// Entry is a registered command: its handler plus the metadata help prints.
type Entry struct {
	// Name is what the user types, e.g. "init".
	Name string
	// Usage is the full synopsis without the leading "pesto ",
	// e.g. "init <name> [--module <path>]".
	Usage string
	// Description is a one-line summary for the command list.
	Description string
	// Details is optional extra help text (examples, subcommands), printed
	// under its own heading.
	Details string
	Handler Handler
}

// Commands is the registry of available commands.
type Commands struct {
	Store map[string]Entry
	order []string
}

// New returns an empty registry.
func New() *Commands {
	return &Commands{Store: make(map[string]Entry)}
}

// Register adds a command. Registration order is the order help lists them in.
func (c *Commands) Register(e Entry) {
	if _, exists := c.Store[e.Name]; !exists {
		c.order = append(c.order, e.Name)
	}
	c.Store[e.Name] = e
}

// Entries returns the registered commands in registration order.
func (c *Commands) Entries() []Entry {
	entries := make([]Entry, 0, len(c.order))
	for _, name := range c.order {
		entries = append(entries, c.Store[name])
	}
	return entries
}

// Run dispatches cmd to its handler. A *UsageError returned by the handler is
// filled in with the command's usage so the caller can print it.
func (c *Commands) Run(s *cli.State, cmd Command) error {
	e, ok := c.Store[cmd.Name]
	if !ok {
		return &UsageError{Err: fmt.Errorf("unknown command %q", cmd.Name)}
	}

	err := e.Handler(s, cmd)
	var usageErr *UsageError
	if errors.As(err, &usageErr) && usageErr.Usage == "" {
		usageErr.Usage = e.Usage
	}
	return err
}

// UsageError reports that a command was invoked incorrectly (exit code 2).
type UsageError struct {
	Err error
	// Usage is the synopsis of the command that failed; Run fills it in.
	Usage string
}

func (e *UsageError) Error() string { return e.Err.Error() }

func (e *UsageError) Unwrap() error { return e.Err }

// Usagef returns a *UsageError with a formatted message.
func Usagef(format string, args ...any) error {
	return &UsageError{Err: fmt.Errorf(format, args...)}
}

// ExitError makes the process exit with Code. A nil Err means the failure was
// already reported (e.g. by `go test`) and nothing more should be printed.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit status %d", e.Code)
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }
