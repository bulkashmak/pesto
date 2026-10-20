package handlers

import (
	"fmt"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
)

// HandleVersion prints the CLI version.
func HandleVersion(s *cli.State, cmd commands.Command) error {
	if len(cmd.Args) != 0 {
		return commands.Usagef("version takes no arguments")
	}
	fmt.Fprintf(s.Stdout, "pesto %s\n", s.Version)
	return nil
}
