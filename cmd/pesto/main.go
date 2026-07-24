// Command pesto is the Pesto CLI: it scaffolds projects and owns the run
// (transform + `go test`). See docs/design/active/design-cli.md.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/internal/runner"
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
	case "version":
		fmt.Printf("pesto %s\n", version)
	case "init", "generate":
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
  pesto run [--env <name>] [--root <dir>] [go test args...]
  pesto version

Commands:
  run        transform *pesto.T tests and drive `+"`go test`"+`
  version    print the pesto version
`)
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
