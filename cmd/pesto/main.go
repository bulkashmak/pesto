// Command pesto is the Pesto CLI: it scaffolds projects and owns the run
// (transform + `go test`). See docs/design/active/design-cli.md.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/internal/generate"
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
		os.Exit(cmdGenerate(os.Args[2:]))
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
  pesto generate <e2e|suite|client> <name> [--root <dir>]
  pesto run [--env <name>] [--root <dir>] [--report junit] [--out <path>] [go test args...]
  pesto version

Commands:
  init       scaffold a new test project
  generate   scaffold a test, suite, or client inside a project
  run        transform *pesto.T tests and drive `+"`go test`"+`
  version    print the pesto version

Generate:
  pesto generate e2e <domain>/<name>   an E2E test, e.g. e2e transfer/happy_path
  pesto generate suite <domain>        an E2E domain folder with shared helpers
  pesto generate client <name>         a typed client stub under clients/
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

// cmdGenerate scaffolds a file inside an existing project.
func cmdGenerate(args []string) int {
	root := "."
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--root" || arg == "-root":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "pesto: --root needs a value")
				return 2
			}
			i++
			root = args[i]
		case strings.HasPrefix(arg, "--root="):
			root = strings.TrimPrefix(arg, "--root=")
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(os.Stderr, "pesto: unknown flag %q\n", arg)
			return 2
		default:
			positional = append(positional, arg)
		}
	}
	if len(positional) != 2 {
		fmt.Fprintln(os.Stderr, "pesto: generate needs a kind and a name\n\nUsage: pesto generate <e2e|suite|client> <name> [--root <dir>]")
		return 2
	}
	kind, name := positional[0], positional[1]

	var created []string
	var err error
	switch kind {
	case "e2e":
		created, err = generate.E2E(root, name)
	case "suite":
		created, err = generate.Suite(root, name)
	case "client":
		created, err = generate.Client(root, name)
	case "contract":
		fmt.Fprintln(os.Stderr, "pesto: generate contract is planned (Phase 4); see docs/design/roadmap.md")
		return 1
	default:
		fmt.Fprintf(os.Stderr, "pesto: unknown generate kind %q (supported: e2e, suite, client)\n", kind)
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 1
	}
	for _, path := range created {
		fmt.Printf("created %s\n", path)
	}
	return 0
}

func cmdRun(args []string) int {
	cfg, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 2
	}

	// Resolve the environment name: explicit flag, else pesto.yaml's default_env.
	if cfg.env == "" {
		if pc, cfgErr := pesto.LoadConfig(cfg.root); cfgErr == nil {
			cfg.env = pc.DefaultEnv
		}
	}
	// Fail fast if a named environment cannot be resolved.
	if cfg.env != "" {
		if _, envErr := pesto.LoadEnvironment(cfg.root, cfg.env); envErr != nil {
			fmt.Fprintf(os.Stderr, "pesto: %v\n", envErr)
			return 1
		}
	}

	code, err := runner.Run(runner.Options{
		Root:       cfg.root,
		Env:        cfg.env,
		Args:       cfg.passthrough,
		Stdout:     os.Stdout,
		Stderr:     os.Stderr,
		Keep:       cfg.keep,
		Report:     cfg.report,
		ReportPath: cfg.out,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "pesto: %v\n", err)
		return 1
	}
	return code
}

// runConfig holds pesto's own `run` flags, separated from go test passthrough.
type runConfig struct {
	env, root, report, out string
	keep                   bool
	passthrough            []string
}

// parseRunArgs pulls pesto's own flags out of args and passes everything else
// through to `go test`, so `pesto run --env dev --report junit -run X -v` works.
// Both `--flag value` and `--flag=value` forms are accepted.
func parseRunArgs(args []string) (runConfig, error) {
	cfg := runConfig{root: "."}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			cfg.passthrough = append(cfg.passthrough, arg)
			continue
		}
		key, inlineVal, hasInline := strings.Cut(strings.TrimLeft(arg, "-"), "=")

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

		var err error
		switch key {
		case "env":
			cfg.env, err = takeValue()
		case "root":
			cfg.root, err = takeValue()
		case "report":
			cfg.report, err = takeValue()
		case "out":
			cfg.out, err = takeValue()
		case "keep":
			cfg.keep = true
		default:
			cfg.passthrough = append(cfg.passthrough, arg)
		}
		if err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}
