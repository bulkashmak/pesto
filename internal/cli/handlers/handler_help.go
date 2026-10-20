package handlers

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
)

// NewHelp returns the help handler, which lists every command registered in
// cmds. It is a constructor because help needs the registry it belongs to.
func NewHelp(cmds *commands.Commands) commands.Handler {
	return func(s *cli.State, cmd commands.Command) error {
		PrintUsage(s.Stdout, cmds)
		return nil
	}
}

// PrintUsage writes the top-level usage, generated from the registry.
func PrintUsage(w io.Writer, cmds *commands.Commands) {
	entries := cmds.Entries()

	fmt.Fprint(w, "pesto - black-box backend test automation\n\nUsage:\n")
	for _, e := range entries {
		fmt.Fprintf(w, "  pesto %s\n", e.Usage)
	}

	fmt.Fprint(w, "\nCommands:\n")
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	for _, e := range entries {
		fmt.Fprintf(tw, "  %s\t%s\n", e.Name, e.Description)
	}
	tw.Flush()

	for _, e := range entries {
		if e.Details != "" {
			fmt.Fprintf(w, "\n%s:\n%s", e.Name, e.Details)
		}
	}
}
