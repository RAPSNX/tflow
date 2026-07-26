package buildinfo

import "testing"

func TestResolve(t *testing.T) {
	tests := []struct {
		name          string
		linkedVersion string
		moduleVersion string
		want          string
	}{
		{name: "release linker value", linkedVersion: "v0.1.0-alpha.1", moduleVersion: "v0.1.0-alpha.1", want: "v0.1.0-alpha.1"},
		{name: "module install", linkedVersion: "devel", moduleVersion: "v0.1.0-alpha.1", want: "v0.1.0-alpha.1"},
		{name: "development build", linkedVersion: "devel", moduleVersion: "(devel)", want: "devel"},
		{name: "nix development build", linkedVersion: "devel-9bddddb", moduleVersion: "(devel)", want: "devel-9bddddb"},
		{name: "pseudo-version ahead of a tag falls back to devel", linkedVersion: "devel", moduleVersion: "v0.1.0-alpha.1.0.20260726141959-8abb593b2a90", want: "devel"},
		{name: "pseudo-version with no prior tag falls back to devel", linkedVersion: "devel", moduleVersion: "v0.0.0-20260726141959-8abb593b2a90", want: "devel"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolve(test.linkedVersion, test.moduleVersion); got != test.want {
				t.Fatalf("resolve(%q, %q) = %q, want %q", test.linkedVersion, test.moduleVersion, got, test.want)
			}
		})
	}
}

func TestIsPseudoVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "release tag", version: "v0.1.0-alpha.1", want: false},
		{name: "pseudo-version ahead of a tag", version: "v0.1.0-alpha.1.0.20260726141959-8abb593b2a90", want: true},
		{name: "pseudo-version with no prior tag", version: "v0.0.0-20260726141959-8abb593b2a90", want: true},
		{name: "pseudo-version with incompatible suffix", version: "v2.0.0-20260726141959-8abb593b2a90+incompatible", want: true},
		{name: "pseudo-version with dirty suffix", version: "v0.1.0-alpha.1.0.20260726141959-8abb593b2a90+dirty", want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isPseudoVersion(test.version); got != test.want {
				t.Fatalf("isPseudoVersion(%q) = %v, want %v", test.version, got, test.want)
			}
		})
	}
}
