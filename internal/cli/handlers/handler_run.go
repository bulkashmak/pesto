package handlers

import (
	"strings"

	"github.com/bulkashmak/pesto"
	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
	"github.com/bulkashmak/pesto/internal/runner"
)

// HandleRun transforms *pesto.T tests and drives `go test`. A failing test run
// is returned as a *commands.ExitError carrying go test's exit code.
func HandleRun(s *cli.State, cmd commands.Command) error {
	cfg, err := parseRunArgs(cmd.Args)
	if err != nil {
		return err
	}

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

// runConfig holds pesto's own `run` flags, separated from go test passthrough.
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
