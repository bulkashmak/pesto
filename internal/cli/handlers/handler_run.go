package handlers

import (
	"fmt"
	"strings"

	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/project"
	"github.com/bulkashmak/pesto/internal/runner"
	"github.com/bulkashmak/pesto/internal/transform"
	"github.com/bulkashmak/pesto/internal/version"
)

// HandleRun transforms *pesto.T tests and drives `go test`. A failing test run
// is returned as a *commands.ExitError carrying go test's exit code.
//
// Without --root, the project root is found by searching upward from the
// working directory, and go test runs from the working directory, so package
// patterns behave as in `go test` (a bare `pesto run` covers the current
// subtree). With --root, everything runs from that root.
func HandleRun(s *cli.State, cmd commands.Command) error {
	cfg, err := parseRunArgs(cmd.Args)
	if err != nil {
		return err
	}

	var runDir string
	if cfg.root == "" {
		if runDir, err = s.Dir(); err != nil {
			return err
		}
		if cfg.root, err = project.FindRoot(runDir); err != nil {
			return err
		}
	} else if err := project.Check(cfg.root); err != nil {
		return err
	}

	warnVersionMismatch(s, cfg.root)

	// Resolve the environment name: explicit flag, else pesto.yaml's default_env.
	if cfg.env == "" {
		if pc, cfgErr := pesto.LoadConfig(cfg.root); cfgErr == nil {
			cfg.env = pc.DefaultEnv
		}
	}
	// Fail fast if a named environment cannot be resolved.
	if cfg.env != "" {
		if _, err := pesto.LoadEnvironment(cfg.root, cfg.env); err != nil {
			return err
		}
	}

	code, err := runner.Run(runner.Options{
		Root:       cfg.root,
		Dir:        runDir,
		Env:        cfg.env,
		Args:       cfg.passthrough,
		Stdout:     s.Stdout,
		Stderr:     s.Stderr,
		Keep:       cfg.keep,
		Report:     cfg.report,
		ReportPath: cfg.out,
	})
	if err != nil {
		return err
	}
	if code != 0 {
		return &commands.ExitError{Code: code}
	}
	return nil
}

// warnVersionMismatch tells the user when a released CLI drives a project
// built against a different runtime release: the transform generates code
// against the runtime API, so the two should move together. It only warns -
// a newer CLI usually still handles an older runtime - and stays quiet for
// development builds and replaced (local) runtimes, where versions mean nothing.
func warnVersionMismatch(s *cli.State, root string) {
	if !version.IsRelease(s.Version) {
		return
	}
	req, err := project.ModuleRequirement(root, transform.RuntimeImportPath)
	if err != nil || req.Replaced || req.Version == "" || req.Version == s.Version {
		return
	}
	fmt.Fprintf(s.Stderr, "pesto: warning: this CLI is %[1]s but the project requires %[2]s %[3]s;\n"+
		"  align them with `go get %[2]s@%[1]s` or `go install %[2]s/cmd/pesto@%[3]s`\n",
		s.Version, transform.RuntimeImportPath, req.Version)
}

// runConfig holds pesto's own `run` flags, separated from go test passthrough.
// An empty root means --root was not given.
type runConfig struct {
	env, root, report, out string
	keep                   bool
	passthrough            []string
}

// parseRunArgs pulls pesto's own flags out of args and passes everything else
// through to `go test`, so `pesto run --env dev --report junit -run X -v` works.
// Both `--flag value` and `--flag=value` forms are accepted. It cannot use
// splitArgs, which rejects unknown flags.
func parseRunArgs(args []string) (runConfig, error) {
	var cfg runConfig
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
				return "", commands.Usagef("flag --%s needs a value", key)
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
