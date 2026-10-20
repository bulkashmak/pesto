package handlers

import (
	"fmt"
	"path/filepath"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/scaffold"
	"github.com/bulkashmak/pesto/internal/version"
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

	// A released CLI pins its own version, so the project's runtime matches the
	// CLI that created it. A development build has no version a project could
	// resolve; v0.0.0 is a placeholder that `go get ...@latest` replaces.
	pin := version.IsRelease(s.Version)
	runtimeVersion := "v0.0.0"
	if pin {
		runtimeVersion = s.Version
	}
	if err := scaffold.Create(scaffold.Options{Dir: name, Module: module, PestoVersion: runtimeVersion}); err != nil {
		return err
	}

	resolve := fmt.Sprintf("go get %s@latest   # resolve the pesto runtime dependency", scaffold.RuntimeImportPath())
	if pin {
		resolve = fmt.Sprintf("go mod tidy   # download the pesto runtime (%s)", runtimeVersion)
	}
	fmt.Fprintf(s.Stdout, `Created Pesto project in %s/

Next steps:
  cd %s
  %s
  pesto run
`, name, name, resolve)
	return nil
}
