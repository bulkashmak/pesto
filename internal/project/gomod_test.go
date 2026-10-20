package project

import (
	"os"
	"path/filepath"
	"testing"
)

const mod = "github.com/bulkashmak/pesto"

func writeGoMod(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModuleRequirement(t *testing.T) {
	cases := []struct {
		name, gomod string
		want        Requirement
	}{
		{
			name:  "single-line require",
			gomod: "module x\n\ngo 1.24\n\nrequire github.com/bulkashmak/pesto v0.1.0\n",
			want:  Requirement{Version: "v0.1.0"},
		},
		{
			name: "require block with comments and other modules",
			gomod: "module x\n\nrequire (\n\tgopkg.in/yaml.v3 v3.0.1 // indirect\n" +
				"\tgithub.com/bulkashmak/pesto v0.2.0 // pinned\n)\n",
			want: Requirement{Version: "v0.2.0"},
		},
		{
			name:  "replaced to a local checkout",
			gomod: "module x\n\nrequire github.com/bulkashmak/pesto v0.0.0\n\nreplace github.com/bulkashmak/pesto => ../pesto\n",
			want:  Requirement{Version: "v0.0.0", Replaced: true},
		},
		{
			name:  "replace block",
			gomod: "module x\n\nrequire github.com/bulkashmak/pesto v0.1.0\n\nreplace (\n\tgithub.com/bulkashmak/pesto v0.1.0 => ../pesto\n)\n",
			want:  Requirement{Version: "v0.1.0", Replaced: true},
		},
		{
			name:  "not required; prefix match is not a match",
			gomod: "module x\n\nrequire github.com/bulkashmak/pesto-extra v1.0.0\n",
			want:  Requirement{},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeGoMod(t, dir, c.gomod)
			got, err := ModuleRequirement(dir, mod)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestModuleRequirementUsesParentGoMod(t *testing.T) {
	parent := t.TempDir()
	writeGoMod(t, parent, "module x\n\nrequire github.com/bulkashmak/pesto v0.3.0\n")
	child := filepath.Join(parent, "qa")
	if err := os.Mkdir(child, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ModuleRequirement(child, mod)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != "v0.3.0" {
		t.Errorf("got %+v, want version from the parent go.mod", got)
	}
}

func TestModuleRequirementNoGoMod(t *testing.T) {
	got, err := ModuleRequirement(tempDir(t), mod)
	if err != nil || got != (Requirement{}) {
		t.Fatalf("got %+v, %v; want zero requirement, nil", got, err)
	}
}
