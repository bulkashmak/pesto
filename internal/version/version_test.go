package version

import "testing"

func TestFromBuildInfo(t *testing.T) {
	for in, want := range map[string]string{
		"":        Devel,
		"(devel)": Devel,
		"v0.1.0":  "v0.1.0",
		"v0.0.0-20261020120000-64fef63abcde+dirty": "v0.0.0-20261020120000-64fef63abcde+dirty",
	} {
		if got := fromBuildInfo(in); got != want {
			t.Errorf("fromBuildInfo(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRelease(t *testing.T) {
	for v, want := range map[string]bool{
		"v0.1.0":                               true,
		"v1.12.3":                              true,
		"v0.2.0-rc.1":                          true,
		Devel:                                  false,
		"0.1.0":                                false,
		"v0.1":                                 false,
		"v0.1.0+dirty":                         false,
		"v0.0.0-20261020120000-64fef63abcde":   false,
		"v0.1.1-0.20261020120000-64fef63abcde": false,
		"v0.0.0-20261020120000-64fef63abcde+dirty":  false,
		"v0.2.0-rc.1.0.20261020120000-64fef63abcde": false,
	} {
		if got := IsRelease(v); got != want {
			t.Errorf("IsRelease(%q) = %v, want %v", v, got, want)
		}
	}
}
