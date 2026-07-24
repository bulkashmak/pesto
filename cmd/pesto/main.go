// Command pesto is the Pesto CLI: it scaffolds projects and owns the run
// (transform + `go test`). See docs/design/active/design-cli.md.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/internal/runner"
	"github.com/bulkashmak/pesto/internal/scaffold"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "run":
		os.Exit(cmdRun(os.Args[2:]))
	case "init":
		os.Exit(cmdInit(os.Args[2:]))
	case "version":
		fmt.Printf("pesto %s\n", version)
	case "generate":
		fmt.Fprintf(os.Stderr, "pesto %s: not implemented yet\n", os.Args[1])
		os.Exit(1)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "pesto: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `pesto - black-box backend test automation

Usage:
  pesto init <name> [--module <path>]
  pesto run [--env <name>] [--root <dir>] [go test args...]
  pesto version

Commands:
  init       scaffold a new test project
  run        transform *pesto.T tests and drive `+"`go test`"+`
  version    print the pesto version
`)
}

// cmdInit scaffolds a new project directory.
func cmdInit(args []string) int {
	var name, module string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--module" || arg == "-module":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "pesto: --module needs a value")
				return 2
			}
			i++
			module = args[i]
		case strings.HasPrefix(arg, "--module="):
			module = strings.TrimPrefix(arg, "--module=")
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "pesto: unknown flag %q\n", arg)
			return 2
		default:
			if name != "" {
				fmt.Fprintf(os.Stderr, "pesto: unexpected argument %q\n", arg)
				return 2
			}
			name = arg
		}
	}

	if name == "" {
		fmt.Fprintln(os.Stderr, "pesto: init needs a project name\n\nUsage: pesto init <name> [--module <path>]")
		return 2
	}
	if module == "" {
		module = filepath.Base(name)
	}

	// TODO: inject the CLI's released version once tags exist; v0.0.0 relies on the
	// user running `go get github.com/bulkashmak/pesto@latest` (or a replace).
	if err := scaffold.Create(scaffold.Options{Dir: name, Module: module, PestoVersion: "v0.0.0"}); err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 1
	}

	fmt.Printf(`Created Pesto project in %s/

Next steps:
  cd %s
  go get %s@latest   # resolve the pesto runtime dependency
  pesto run
`, name, name, scaffold.RuntimeImportPath())
	return 0
}

func cmdRun(args []string) int {
	env, root, keep, passthrough, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 2
	}

	// Resolve the environment name: explicit flag, else pesto.yaml's default_env.
	if env == "" {
		if cfg, cfgErr := pesto.LoadConfig(root); cfgErr == nil {
			env = cfg.DefaultEnv
		}
	}
	// Fail fast if a named environment cannot be resolved.
	if env != "" {
		if _, envErr := pesto.LoadEnvironment(root, env); envErr != nil {
			fmt.Fprintf(os.Stderr, "pesto: %v\n", envErr)
			return 1
		}
	}

	code, err := runner.Run(runner.Options{
		Root:   root,
		Env:    env,
		Args:   passthrough,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Keep:   keep,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 1
	}
	return code
}

// parseRunArgs pulls pesto's own flags (--env, --root, --keep) out of args and
// passes everything else through to `go test`, so `pesto run --env dev -run X -v`
// works. Both `--flag value` and `--flag=value` forms are accepted.
func parseRunArgs(args []string) (env, root string, keep bool, passthrough []string, err error) {
	root = "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name := strings.TrimLeft(arg, "-")
		key, inlineVal, hasInline := strings.Cut(name, "=")

		takeValue := func() (string, error) {
			if hasInline {
				return inlineVal, nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag --%s needs a value", key)
			}
			i++
			return args[i], nil
		}

		switch {
		case strings.HasPrefix(arg, "-") && key == "env":
			if env, err = takeValue(); err != nil {
				return
			}
		case strings.HasPrefix(arg, "-") && key == "root":
			if root, err = takeValue(); err != nil {
				return
			}
		case strings.HasPrefix(arg, "-") && key == "keep":
			keep = true
		default:
			passthrough = append(passthrough, arg)
		}
	}
	return env, root, keep, passthrough, nil
}
