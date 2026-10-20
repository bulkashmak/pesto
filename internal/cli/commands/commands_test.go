package commands_test

import (
	"errors"
	"testing"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
)

func TestRunDispatchesToHandler(t *testing.T) {
	cmds := commands.New()
	var got commands.Command
	cmds.Register(commands.Entry{Name: "echo", Handler: func(_ *cli.State, cmd commands.Command) error {
		got = cmd
		return nil
	}})

	if err := cmds.Run(&cli.State{}, commands.Command{Name: "echo", Args: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	if got.Name != "echo" || len(got.Args) != 2 {
		t.Fatalf("handler got %+v", got)
	}
}

func TestRunUnknownCommandIsUsageError(t *testing.T) {
	err := commands.New().Run(&cli.State{}, commands.Command{Name: "nope"})
	var usageErr *commands.UsageError
	if !errors.As(err, &usageErr) {
		t.Fatalf("want *UsageError, got %v", err)
	}
	if usageErr.Usage != "" {
		t.Fatalf("unknown command should carry no usage, got %q", usageErr.Usage)
	}
}

func TestRunFillsUsage(t *testing.T) {
	cmds := commands.New()
	cmds.Register(commands.Entry{Name: "init", Usage: "init <name>", Handler: func(*cli.State, commands.Command) error {
		return commands.Usagef("init needs a project name")
	}})

	err := cmds.Run(&cli.State{}, commands.Command{Name: "init"})
	var usageErr *commands.UsageError
	if !errors.As(err, &usageErr) || usageErr.Usage != "init <name>" {
		t.Fatalf("want usage filled in, got %#v", err)
	}
}

func TestEntriesKeepRegistrationOrder(t *testing.T) {
	cmds := commands.New()
	for _, name := range []string{"init", "run", "help"} {
		cmds.Register(commands.Entry{Name: name})
	}
	cmds.Register(commands.Entry{Name: "run", Description: "re-registered"})

	entries := cmds.Entries()
	if len(entries) != 3 || entries[0].Name != "init" || entries[1].Name != "run" || entries[2].Name != "help" {
		t.Fatalf("unexpected order: %+v", entries)
	}
	if entries[1].Description != "re-registered" {
		t.Fatalf("re-registering should replace the entry")
	}
}
