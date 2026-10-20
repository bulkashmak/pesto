package handlers

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bulkashmak/pesto/internal/cli"
	"github.com/bulkashmak/pesto/internal/cli/commands"
)

func newState() (*cli.State, *bytes.Buffer) {
	var out bytes.Buffer
	return &cli.State{Version: "1.2.3", Stdout: &out, Stderr: &out}, &out
}

func isUsageErr(err error) bool {
	var usageErr *commands.UsageError
	return errors.As(err, &usageErr)
}

func TestSplitArgs(t *testing.T) {
	flags, pos, err := splitArgs([]string{"demo", "--module", "a/b", "-root=x", "extra"}, "module", "root")
	if err != nil {
		t.Fatal(err)
	}
	if flags["module"] != "a/b" || flags["root"] != "x" {
		t.Fatalf("flags = %v", flags)
	}
	if !reflect.DeepEqual(pos, []string{"demo", "extra"}) {
		t.Fatalf("positional = %v", pos)
	}

	if _, _, err := splitArgs([]string{"--bogus"}, "module"); !isUsageErr(err) {
		t.Fatalf("unknown flag: want usage error, got %v", err)
	}
	if _, _, err := splitArgs([]string{"--module"}, "module"); !isUsageErr(err) {
		t.Fatalf("missing value: want usage error, got %v", err)
	}
}

func TestParseRunArgs(t *testing.T) {
	cfg, err := parseRunArgs([]string{"--env", "dev", "--report=junit", "-run", "TestX", "-v", "--keep", "./..."})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.env != "dev" || cfg.report != "junit" || !cfg.keep || cfg.root != "" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if want := []string{"-run", "TestX", "-v", "./..."}; !reflect.DeepEqual(cfg.passthrough, want) {
		t.Fatalf("passthrough = %v, want %v", cfg.passthrough, want)
	}
	if _, err := parseRunArgs([]string{"--out"}); !isUsageErr(err) {
		t.Fatalf("missing value: want usage error, got %v", err)
	}
}

func TestHandleVersion(t *testing.T) {
	s, out := newState()
	if err := HandleVersion(s, commands.Command{Name: "version"}); err != nil {
		t.Fatal(err)
	}
	if out.String() != "pesto 1.2.3\n" {
		t.Fatalf("output = %q", out.String())
	}
}

func TestHandleInit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "demo")
	s, out := newState()
	if err := HandleInit(s, commands.Command{Name: "init", Args: []string{dir, "--module", "example.com/demo"}}); err != nil {
		t.Fatal(err)
	}
	gomod, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gomod), "module example.com/demo") {
		t.Fatalf("go.mod = %s", gomod)
	}
	if !strings.Contains(out.String(), "Created Pesto project") {
		t.Fatalf("output = %q", out.String())
	}

	if err := HandleInit(s, commands.Command{Name: "init"}); !isUsageErr(err) {
		t.Fatalf("no name: want usage error, got %v", err)
	}
}

func TestHandleGenerate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/p\n\ngo 1.24\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, out := newState()
	err := HandleGenerate(s, commands.Command{Name: "generate", Args: []string{"e2e", "transfer/happy_path", "--root", root}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "tests", "e2e", "transfer", "happy_path_test.go")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "created ") {
		t.Fatalf("output = %q", out.String())
	}

	if err := HandleGenerate(s, commands.Command{Name: "generate", Args: []string{"bogus", "x"}}); !isUsageErr(err) {
		t.Fatalf("unknown kind: want usage error, got %v", err)
	}
	if err := HandleGenerate(s, commands.Command{Name: "generate", Args: []string{"e2e"}}); !isUsageErr(err) {
		t.Fatalf("missing name: want usage error, got %v", err)
	}
	for _, bad := range []string{"9bad/x", "a/b/c"} {
		if err := HandleGenerate(s, commands.Command{Name: "generate", Args: []string{"e2e", bad, "--root", root}}); !isUsageErr(err) {
			t.Errorf("invalid name %q: want usage error, got %v", bad, err)
		}
	}
}

func TestPrintUsageListsRegisteredCommands(t *testing.T) {
	cmds := commands.New()
	cmds.Register(commands.Entry{Name: "init", Usage: "init <name>", Description: "scaffold", Handler: HandleInit})
	cmds.Register(commands.Entry{Name: "help", Usage: "help", Description: "show help", Details: "  more\n", Handler: NewHelp(cmds)})

	s, out := newState()
	if err := cmds.Run(s, commands.Command{Name: "help"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"pesto init <name>", "init   scaffold", "help   show help", "help:\n  more"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("help output missing %q:\n%s", want, out.String())
		}
	}
}

func TestHandleGenerateFromSubdirectory(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"go.mod", "pesto.yaml"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("module example.com/p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sub := filepath.Join(root, "tests", "e2e")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	s, out := newState()
	s.WorkDir = sub
	if err := HandleGenerate(s, commands.Command{Name: "generate", Args: []string{"client", "users"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "clients", "users", "users.go")); err != nil {
		t.Fatalf("client not created at the project root: %v", err)
	}
	want := "created " + filepath.Join("..", "..", "clients", "users", "users.go") + "\n"
	if out.String() != want {
		t.Fatalf("output = %q, want %q (relative to the working directory)", out.String(), want)
	}
}

func TestHandleRunOutsideProject(t *testing.T) {
	s, _ := newState()
	s.WorkDir = t.TempDir()
	err := HandleRun(s, commands.Command{Name: "run"})
	if err == nil || !strings.Contains(err.Error(), "no Pesto project found") {
		t.Fatalf("want project-not-found error, got %v", err)
	}
}

func TestHandlersRejectInvalidRoot(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	empty := t.TempDir()

	for _, c := range []struct {
		name    string
		handler commands.Handler
		args    []string
	}{
		{"run", HandleRun, nil},
		{"generate", HandleGenerate, []string{"client", "x"}},
	} {
		s, _ := newState()
		err := c.handler(s, commands.Command{Name: c.name, Args: append(c.args, "--root", missing)})
		if err == nil || !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("%s --root missing: got %v", c.name, err)
		}
		err = c.handler(s, commands.Command{Name: c.name, Args: append(c.args, "--root", empty)})
		if err == nil || !strings.Contains(err.Error(), "is not a Pesto project") {
			t.Errorf("%s --root empty: got %v", c.name, err)
		}
	}
}
