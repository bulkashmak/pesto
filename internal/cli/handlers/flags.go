// Package handlers implements the pesto CLI commands, one handler per file.
// Handlers stay thin: they parse arguments, call the engine packages
// (scaffold, generate, runner) and print the result to the cli.State writers.
package handlers

import (
	"strings"

	"github.com/bulkashmak/pesto/internal/cli/commands"
)

// splitArgs separates the value flags named in known from positional
// arguments. Both `--flag value` and `--flag=value` (and single-dash) forms are
// accepted; any other flag is a usage error.
func splitArgs(args []string, known ...string) (map[string]string, []string, error) {
	flags := make(map[string]string)
	var positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") {
			positional = append(positional, arg)
			continue
		}
		key, val, hasVal := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if !contains(known, key) {
			return nil, nil, commands.Usagef("unknown flag %q", arg)
		}
		if !hasVal {
			if i+1 >= len(args) {
				return nil, nil, commands.Usagef("--%s needs a value", key)
			}
			i++
			val = args[i]
		}
		flags[key] = val
	}
	return flags, positional, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
