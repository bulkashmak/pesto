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

// HandleGenerate scaffolds a file inside an existing project:
// `pesto generate <e2e|suite|client> <name> [--root <dir>]`. Without --root,
// the project root is found by searching upward from the working directory.
func HandleGenerate(s *cli.State, cmd commands.Command) error {
	flags, positional, err := splitArgs(cmd.Args, "root")
	if err != nil {
		return err
	}
	if len(positional) != 2 {
		return commands.Usagef("generate needs a kind and a name")
	}
	kind, name := positional[0], positional[1]

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
