package project

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Requirement is how a project's go.mod pulls in a module.
type Requirement struct {
	// Version is the required version; empty if the module is not required.
	Version string
	// Replaced reports a replace directive for the module (e.g. a local
	// checkout), which makes Version meaningless for comparisons.
	Replaced bool
}

// ModuleRequirement reads the go.mod governing root (in root or the nearest
// parent) and reports how it requires modPath. A missing go.mod yields a zero
// Requirement. It understands single-line and block require/replace
// directives, which is all go.mod allows for them.
func ModuleRequirement(root, modPath string) (Requirement, error) {
	var req Requirement
	path, ok := findGoMod(root)
	if !ok {
		return req, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return req, err
	}
	defer f.Close()

	block := "" // directive of the enclosing ( ... ) block, if any
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line, _, _ := strings.Cut(sc.Text(), "//")
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if block != "" {
			if fields[0] == ")" {
				block = ""
				continue
			}
			fields = append([]string{block}, fields...)
		} else if len(fields) == 2 && fields[1] == "(" {
			block = fields[0]
			continue
		}

		if len(fields) < 3 || fields[1] != modPath {
			continue
		}
		switch fields[0] {
		case "require":
			req.Version = fields[2]
		case "replace":
			req.Replaced = true
		}
	}
	return req, sc.Err()
}

// findGoMod returns the go.mod in dir or its nearest parent.
func findGoMod(dir string) (string, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for d := dir; ; d = filepath.Dir(d) {
		if p := filepath.Join(d, "go.mod"); exists(p) {
			return p, true
		}
		if filepath.Dir(d) == d {
			return "", false
		}
	}
}
