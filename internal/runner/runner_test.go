package runner

import (
	"bytes"
	"os"
	"path/filepath"
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

// TestRunWritesJUnitReport runs the fixture with --report junit and checks the
// XML report is written and reflects the passing test.
func TestRunWritesJUnitReport(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "junit.xml")
	var out, errOut bytes.Buffer
	code, err := Run(Options{
		Root:       "testdata/basic",
		Args:       []string{"-run", "TestPing"},
		Report:     "junit",
		ReportPath: reportPath,
		Stdout:     &out,
		Stderr:     &errOut,
	})
	combined := out.String() + errOut.String()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, combined)
	}
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, combined)
	}

	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("report not written: %v", err)
	}
	xml := string(data)
	for _, want := range []string{`<testsuites`, `<testcase name="TestPing"`} {
		if !strings.Contains(xml, want) {
			t.Errorf("report missing %q\n---\n%s", want, xml)
		}
	}
	// The normal console output is still reproduced alongside the report.
	if !strings.Contains(combined, "TestPing") {
		t.Errorf("console output not reproduced:\n%s", combined)
	}
}

// TestRunNoTests reports a clear error when a project has no Pesto tests.
func TestRunNoTests(t *testing.T) {
	_, err := Run(Options{Root: t.TempDir(), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if err == nil {
		t.Fatal("expected an error when no Pesto tests are present")
	}
}
