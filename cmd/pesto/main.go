// Command pesto is the Pesto CLI: it scaffolds projects and owns the run
// (transform + `go test`). It only wires the command registry; commands live in
// internal/cli/handlers. See docs/design/active/design-cli.md.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/cli/handlers"
)

const version = "0.0.0-dev"

func main() {
	state := &cli.State{
		Version: version,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	}

	cmds := commands.New()
	cmds.Register(commands.Entry{
		Name:        "init",
		Usage:       "init <name> [--module <path>]",
		Description: "scaffold a new test project",
		Handler:     handlers.HandleInit,
	})
	cmds.Register(commands.Entry{
		Name:        "generate",
		Usage:       "generate <e2e|suite|client> <name> [--root <dir>]",
		Description: "scaffold a test, suite, or client inside a project",
		Details: `  pesto generate e2e <domain>/<name>   an E2E test, e.g. e2e transfer/happy_path
  pesto generate suite <domain>        an E2E domain folder with shared helpers
  pesto generate client <name>         a typed client stub under clients/
`,
		Handler: handlers.HandleGenerate,
	})
	cmds.Register(commands.Entry{
		Name:        "run",
		Usage:       "run [--env <name>] [--root <dir>] [--report junit] [--out <path>] [go test args...]",
		Description: "transform *pesto.T tests and drive `go test`",
		Handler:     handlers.HandleRun,
	})
	cmds.Register(commands.Entry{
		Name:        "version",
		Usage:       "version",
		Description: "print the pesto version",
		Handler:     handlers.HandleVersion,
	})
	cmds.Register(commands.Entry{
		Name:        "help",
		Usage:       "help",
		Description: "show this help",
		Handler:     handlers.NewHelp(cmds),
	})

	if len(os.Args) < 2 {
		handlers.PrintUsage(os.Stderr, cmds)
		os.Exit(2)
	}

	name := os.Args[1]
	if name == "-h" || name == "--help" {
		name = "help"
	}

	err := cmds.Run(state, commands.Command{Name: name, Args: os.Args[2:]})
	os.Exit(exitCode(err, cmds))
}

// exitCode reports err and maps it to the process exit code: 0 on success,
// 2 for usage errors, the carried code for *commands.ExitError, 1 otherwise.
func exitCode(err error, cmds *commands.Commands) int {
	if err == nil {
		return 0
	}

	var exitErr *commands.ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Err != nil {
			fmt.Fprintf(os.Stderr, "pesto: %v\n", exitErr.Err)
		}
		return exitErr.Code
	}

	fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
	var usageErr *commands.UsageError
	if errors.As(err, &usageErr) {
		if usageErr.Usage != "" {
			fmt.Fprintf(os.Stderr, "\nUsage: pesto %s\n", usageErr.Usage)
		} else {
			fmt.Fprintln(os.Stderr)
			handlers.PrintUsage(os.Stderr, cmds)
		}
		return 2
	}
	return 1
}
