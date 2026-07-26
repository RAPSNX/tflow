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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolve(test.linkedVersion, test.moduleVersion); got != test.want {
				t.Fatalf("resolve(%q, %q) = %q, want %q", test.linkedVersion, test.moduleVersion, got, test.want)
			}
		})
	}
}
