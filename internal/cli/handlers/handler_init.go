package handlers

import (
	"fmt"
	"path/filepath"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/scaffold"
)

// HandleInit scaffolds a new project directory: `pesto init <name> [--module <path>]`.
func HandleInit(s *cli.State, cmd commands.Command) error {
	flags, positional, err := splitArgs(cmd.Args, "module")
	if err != nil {
		return err
	}
	switch {
	case len(positional) == 0:
		return commands.Usagef("init needs a project name")
	case len(positional) > 1:
		return commands.Usagef("unexpected argument %q", positional[1])
	}

	name := positional[0]
	module := flags["module"]
	if module == "" {
		module = filepath.Base(name)
	}

	// TODO: inject the CLI's released version once tags exist; v0.0.0 relies on the
	// user running `go get github.com/bulkashmak/pesto@latest` (or a replace).
	if err := scaffold.Create(scaffold.Options{Dir: name, Module: module, PestoVersion: "v0.0.0"}); err != nil {
		return err
	}

	fmt.Fprintf(s.Stdout, `Created Pesto project in %s/

Next steps:
  cd %s
  go get %s@latest   # resolve the pesto runtime dependency
  pesto run
`, name, name, scaffold.RuntimeImportPath())
	return nil
}
