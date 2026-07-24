// Package report converts `go test -json` output into a JUnit XML report - the
// CI-standard test-report schema (consumed by Jenkins, GitLab, GitHub Actions,
// and others), not the JUnit tool. It is CLI-only.
package report

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// Report is the root <testsuites> element.
type Report struct {
	XMLName  xml.Name `xml:"testsuites"`
	Tests    int      `xml:"tests,attr"`
	Failures int      `xml:"failures,attr"`
	Errors   int      `xml:"errors,attr"`
	Skipped  int      `xml:"skipped,attr"`
	Time     string   `xml:"time,attr"`
	Suites   []Suite  `xml:"testsuite"`
}

// Suite is one <testsuite> (a Go package).
type Suite struct {
	Name     string `xml:"name,attr"`
	Tests    int    `xml:"tests,attr"`
	Failures int    `xml:"failures,attr"`
	Errors   int    `xml:"errors,attr"`
	Skipped  int    `xml:"skipped,attr"`
	Time     string `xml:"time,attr"`
	Cases    []Case `xml:"testcase"`
}

// Case is one <testcase> (a Go test or subtest).
type Case struct {
	Name      string   `xml:"name,attr"`
	Classname string   `xml:"classname,attr"`
	Time      string   `xml:"time,attr"`
	Failure   *Message `xml:"failure,omitempty"`
	Error     *Message `xml:"error,omitempty"`
	Skipped   *struct{} `xml:"skipped,omitempty"`
}

// Message carries a failure/error message attribute and captured output body.
type Message struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

// event is a single line of `go test -json` output.
type event struct {
	Action  string
	Package string
	Test    string
	Elapsed float64
	Output  string
}

// FromGoTestJSON reads a `go test -json` stream, aggregates it into a JUnit
// [Report], and (if live is non-nil) forwards each event's raw Output to live so
// the console still shows normal `go test` text.
func FromGoTestJSON(r io.Reader, live io.Writer) (*Report, error) {
	type caseAgg struct {
		c      *Case
		output strings.Builder
	}
	type suiteAgg struct {
		name    string
		order   []string
		cases   map[string]*caseAgg
		pkgOut  strings.Builder
		action  string
		elapsed float64
	}

	suiteOrder := []string{}
	suites := map[string]*suiteAgg{}

	getSuite := func(pkg string) *suiteAgg {
		s := suites[pkg]
		if s == nil {
			s = &suiteAgg{name: pkg, cases: map[string]*caseAgg{}}
			suites[pkg] = s
			suiteOrder = append(suiteOrder, pkg)
		}
		return s
	}

	dec := json.NewDecoder(r)
	for {
		var ev event
		if err := dec.Decode(&ev); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go test json: %w", err)
		}
		if live != nil && ev.Output != "" {
			io.WriteString(live, ev.Output)
		}
		if ev.Package == "" {
			continue
		}
		s := getSuite(ev.Package)

		if ev.Test == "" {
			switch ev.Action {
			case "output":
				s.pkgOut.WriteString(ev.Output)
			case "pass", "fail", "skip":
				s.action = ev.Action
				s.elapsed = ev.Elapsed
			}
			continue
		}

		ca := s.cases[ev.Test]
		if ca == nil {
			ca = &caseAgg{c: &Case{Name: ev.Test, Classname: ev.Package}}
			s.cases[ev.Test] = ca
			s.order = append(s.order, ev.Test)
		}
		switch ev.Action {
		case "output":
			ca.output.WriteString(ev.Output)
		case "pass", "fail", "skip":
			ca.c.Time = secs(ev.Elapsed)
			switch ev.Action {
			case "fail":
				ca.c.Failure = &Message{Message: "failed", Body: ca.output.String()}
			case "skip":
				ca.c.Skipped = &struct{}{}
			}
		}
	}

	rep := &Report{}
	var total float64
	for _, pkg := range suiteOrder {
		s := suites[pkg]
		suite := Suite{Name: s.name, Time: secs(s.elapsed)}
		for _, name := range s.order {
			c := s.cases[name].c
			suite.Cases = append(suite.Cases, *c)
			suite.Tests++
			switch {
			case c.Failure != nil:
				suite.Failures++
			case c.Skipped != nil:
				suite.Skipped++
			}
		}
		// A package that failed to build (or otherwise failed) with no failing
		// test cases becomes a synthetic error case so the failure is visible.
		if s.action == "fail" && suite.Failures == 0 {
			suite.Cases = append(suite.Cases, Case{
				Name:      "build",
				Classname: s.name,
				Time:      secs(0),
				Error:     &Message{Message: "package failed", Body: s.pkgOut.String()},
			})
			suite.Tests++
			suite.Errors++
		}

		rep.Suites = append(rep.Suites, suite)
		rep.Tests += suite.Tests
		rep.Failures += suite.Failures
		rep.Errors += suite.Errors
		rep.Skipped += suite.Skipped
		total += s.elapsed
	}
	rep.Time = secs(total)
	return rep, nil
}

// XML renders the report as indented XML with a header.
func (r *Report) XML() ([]byte, error) {
	body, err := xml.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(body, '\n')...), nil
}

func secs(f float64) string { return fmt.Sprintf("%.3f", f) }
