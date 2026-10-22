package handlers

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/generate"
	"github.com/bulkashmak/pesto/internal/project"
)

// HandleGenerate scaffolds files inside an existing project:
// `pesto generate <e2e|suite|client> <name> [--root <dir>]` or
// `pesto generate contract <METHOD> <path> [--root <dir>]`. Without --root,
// the project root is found by searching upward from the working directory.
func HandleGenerate(s *cli.State, cmd commands.Command) error {
	flags, positional, err := splitArgs(cmd.Args, "root")
	if err != nil {
		return err
	}
	if len(positional) == 0 {
		return commands.Usagef("generate needs a kind and a name")
	}
	kind, args := positional[0], positional[1:]
	switch {
	case kind == "contract" && len(args) != 2:
		return commands.Usagef("generate contract needs a method and a path, e.g. generate contract POST /users/{id}/cards")
	case kind != "contract" && len(args) != 1:
		return commands.Usagef("generate needs a kind and a name")
	}

	workDir, err := s.Dir()
	if err != nil {
		return err
	}
	root := flags["root"]
	if root == "" {
		if root, err = project.FindRoot(workDir); err != nil {
			return err
		}
	} else if err := project.Check(root); err != nil {
		return err
	}

	var created []string
	switch kind {
	case "e2e":
		created, err = generate.E2E(root, args[0])
	case "suite":
		created, err = generate.Suite(root, args[0])
	case "client":
		created, err = generate.Client(root, args[0])
	case "contract":
		created, err = generate.Contract(root, args[0], args[1])
	default:
		return commands.Usagef("unknown generate kind %q (supported: e2e, suite, client, contract)", kind)
	}
	if errors.Is(err, generate.ErrInvalidName) {
		return &commands.UsageError{Err: err}
	}
	if err != nil {
		return err
	}
	for _, path := range created {
		// Show discovered (absolute) paths relative to where the user is.
		if filepath.IsAbs(path) {
			if rel, err := filepath.Rel(workDir, path); err == nil {
				path = rel
			}
		}
		fmt.Fprintf(s.Stdout, "created %s\n", path)
	}
	return nil
}
