// Package version reports the version of the running pesto binary, as stamped
// by the Go toolchain into its build info.
package version

import (
	"regexp"
	"runtime/debug"
)

// Devel is reported when the binary carries no module version.
const Devel = "devel"

// Current returns the pesto CLI version. `go install .../cmd/pesto@v0.1.0`
// stamps "v0.1.0"; a local build is stamped from VCS (a pseudo-version,
// possibly with "+dirty"); anything else reports Devel.
func Current() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return Devel
	}
	return fromBuildInfo(info.Main.Version)
}

func fromBuildInfo(v string) string {
	if v == "" || v == "(devel)" {
		return Devel
	}
	return v
}

var (
	semverRE = regexp.MustCompile(`^v\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)
	pseudoRE = regexp.MustCompile(`\d{14}-[0-9a-f]{12}$`)
)

// IsRelease reports whether v is a tagged release (v1.2.3 or v1.2.3-rc.1) - a
// version any project can require from the module proxy. Pseudo-versions,
// "+dirty" builds and Devel are not: they may name commits that were never
// pushed.
func IsRelease(v string) bool {
	return semverRE.MatchString(v) && !pseudoRE.MatchString(v)
}
