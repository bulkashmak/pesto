// Package project locates the root of a Pesto project, so CLI commands work
// from any directory inside it.
package project

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindRoot walks up from start and returns the nearest directory containing
// pesto.yaml. A project without pesto.yaml (the config is optional) is found by
// its go.mod instead; pesto.yaml wins because a module may hold several
// projects. The returned path is absolute.
func FindRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	moduleRoot := ""
	for d := dir; ; d = filepath.Dir(d) {
		if exists(filepath.Join(d, "pesto.yaml")) {
			return d, nil
		}
		if moduleRoot == "" && exists(filepath.Join(d, "go.mod")) {
			moduleRoot = d
		}
		if filepath.Dir(d) == d {
			break
		}
	}
	if moduleRoot != "" {
		return moduleRoot, nil
	}
	return "", fmt.Errorf("no Pesto project found in %s or any parent directory (no pesto.yaml or go.mod); run `pesto init` or pass --root", dir)
}

// Check verifies that root, given explicitly (e.g. via --root), is an existing
// directory holding a Pesto project: pesto.yaml or, since the config is
// optional, go.mod.
func Check(root string) error {
	info, err := os.Stat(root)
	switch {
	case os.IsNotExist(err):
		return fmt.Errorf("project root %s does not exist", root)
	case err != nil:
		return fmt.Errorf("project root %s: %w", root, err)
	case !info.IsDir():
		return fmt.Errorf("project root %s is not a directory", root)
	}
	if !exists(filepath.Join(root, "pesto.yaml")) && !exists(filepath.Join(root, "go.mod")) {
		return fmt.Errorf("%s is not a Pesto project (no pesto.yaml or go.mod); run `pesto init` or pass a different --root", root)
	}
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
