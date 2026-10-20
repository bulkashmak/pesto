package handlers

import (
	"errors"
	"fmt"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/generate"
)

// HandleGenerate scaffolds a file inside an existing project:
// `pesto generate <e2e|suite|client> <name> [--root <dir>]`.
func HandleGenerate(s *cli.State, cmd commands.Command) error {
	flags, positional, err := splitArgs(cmd.Args, "root")
	if err != nil {
		return err
	}
	if len(positional) != 2 {
		return commands.Usagef("generate needs a kind and a name")
	}
	root := flags["root"]
	if root == "" {
		root = "."
	}
	kind, name := positional[0], positional[1]

	var created []string
	switch kind {
	case "e2e":
		created, err = generate.E2E(root, name)
	case "suite":
		created, err = generate.Suite(root, name)
	case "client":
		created, err = generate.Client(root, name)
	case "contract":
		return errors.New("generate contract is planned (Phase 4); see docs/design/roadmap.md")
	default:
		return commands.Usagef("unknown generate kind %q (supported: e2e, suite, client)", kind)
	}
	if err != nil {
		return err
	}
	for _, path := range created {
		fmt.Fprintf(s.Stdout, "created %s\n", path)
	}
	return nil
}
