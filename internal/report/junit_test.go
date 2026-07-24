package report

import (
	"strings"
	"testing"
)

// sampleJSON is a trimmed `go test -json` stream: one pass, one fail, one skip.
const sampleJSON = `
{"Action":"run","Package":"example.com/p","Test":"TestPass"}
{"Action":"output","Package":"example.com/p","Test":"TestPass","Output":"=== RUN   TestPass\n"}
{"Action":"pass","Package":"example.com/p","Test":"TestPass","Elapsed":0.01}
{"Action":"run","Package":"example.com/p","Test":"TestFail"}
{"Action":"output","Package":"example.com/p","Test":"TestFail","Output":"    boom\n"}
{"Action":"fail","Package":"example.com/p","Test":"TestFail","Elapsed":0.02}
{"Action":"run","Package":"example.com/p","Test":"TestSkip"}
{"Action":"skip","Package":"example.com/p","Test":"TestSkip","Elapsed":0}
{"Action":"fail","Package":"example.com/p","Elapsed":0.05}
`

func TestFromGoTestJSON(t *testing.T) {
	var live strings.Builder
	rep, err := FromGoTestJSON(strings.NewReader(sampleJSON), &live)
	if err != nil {
		t.Fatal(err)
	}

	if rep.Tests != 3 || rep.Failures != 1 || rep.Skipped != 1 {
		t.Errorf("totals: tests=%d failures=%d skipped=%d, want 3/1/1", rep.Tests, rep.Failures, rep.Skipped)
	}
	if len(rep.Suites) != 1 || rep.Suites[0].Name != "example.com/p" {
		t.Fatalf("suites = %+v", rep.Suites)
	}

	// The passthrough writer should have reproduced the raw output.
	if !strings.Contains(live.String(), "=== RUN   TestPass") {
		t.Errorf("live output not forwarded:\n%s", live.String())
	}

	// The failing case carries its output as the failure body.
	var fail *Case
	for i := range rep.Suites[0].Cases {
		if rep.Suites[0].Cases[i].Name == "TestFail" {
			fail = &rep.Suites[0].Cases[i]
		}
	}
	if fail == nil || fail.Failure == nil {
		t.Fatalf("TestFail missing failure element: %+v", rep.Suites[0].Cases)
	}
	if !strings.Contains(fail.Failure.Body, "boom") {
		t.Errorf("failure body missing output: %q", fail.Failure.Body)
	}

	xmlBytes, err := rep.XML()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(xmlBytes)
	for _, want := range []string{
		`<testsuites`, `<testsuite name="example.com/p"`,
		`<testcase name="TestPass"`, `<failure message="failed">`, `<skipped>`,
	} {
		if !strings.Contains(xml, want) {
			t.Errorf("xml missing %q\n---\n%s", want, xml)
		}
	}
}

func TestBuildFailureBecomesErrorCase(t *testing.T) {
	// Package fails with no test-level events (e.g. a build error).
	const buildFail = `
{"Action":"output","Package":"example.com/b","Output":"./x.go:1:1: syntax error\n"}
{"Action":"fail","Package":"example.com/b","Elapsed":0}
`
	rep, err := FromGoTestJSON(strings.NewReader(buildFail), nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors != 1 {
		t.Fatalf("errors = %d, want 1\n%+v", rep.Errors, rep)
	}
	if rep.Suites[0].Cases[0].Error == nil ||
		!strings.Contains(rep.Suites[0].Cases[0].Error.Body, "syntax error") {
		t.Errorf("build failure not captured as error case: %+v", rep.Suites[0].Cases)
	}
}
