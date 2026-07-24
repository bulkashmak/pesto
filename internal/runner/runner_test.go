package runner

import (
	"bytes"
	"strings"
	"testing"
)

// TestRunTransformsAndPasses is the end-to-end proof of Phase 2: an authored
// func TestPing(t *pesto.T) in testdata/basic is transformed and run by the Go
// toolchain, and passes.
func TestRunTransformsAndPasses(t *testing.T) {
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:   "testdata/basic",
		Args:   []string{"-run", "TestPing", "-v", "./..."},
		Stdout: &out,
		Stderr: &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run pipeline: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}
	if !strings.Contains(combined, "TestPing") || !strings.Contains(combined, "PASS") {
		t.Errorf("expected TestPing to run and PASS:\n%s", combined)
	}
}

// TestRunNoTests reports a clear error when a project has no Pesto tests.
func TestRunNoTests(t *testing.T) {
	_, err := Run(Options{Root: t.TempDir(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error when no Pesto tests are present")
	}
}
