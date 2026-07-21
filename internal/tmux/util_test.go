package tmux

import (
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if got, want := ShellQuote("it's"), `'it'"'"'s'`; got != want {
		t.Fatalf("ShellQuote = %q, want %q", got, want)
	}
}

func TestNormalizeCWDExpandsHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	if got := NormalizeCWD("~/project"); got != "/tmp/home/project" {
		t.Fatalf("NormalizeCWD = %q, want /tmp/home/project", got)
	}
}

func TestSessionNamesUseOpaquePrefixes(t *testing.T) {
	if got, want := PersistentSessionName("8f42ac91"), "tflow-p-8f42ac91"; got != want {
		t.Fatalf("PersistentSessionName = %q, want %q", got, want)
	}
	if got, want := VolatileSessionName("instance-1", "8f42ac91"), "tflow-v-instance-1-8f42ac91"; got != want {
		t.Fatalf("VolatileSessionName = %q, want %q", got, want)
	}
}

func TestNormalizeSessionLabelPreservesCasingAndSpaces(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"preserves case", "MyProject Label", "MyProject Label"},
		{"trims surrounding whitespace", "  Deploy Tool  ", "Deploy Tool"},
		{"keeps internal spaces", "Deploy   Tool", "Deploy   Tool"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeSessionLabel(tc.label); got != tc.want {
				t.Fatalf("NormalizeSessionLabel(%q) = %q, want %q", tc.label, got, tc.want)
			}
		})
	}
}

func TestNormalizeSessionLabelStripsDelimiterAndControlCharacters(t *testing.T) {
	cases := []struct {
		name  string
		label string
		want  string
	}{
		{"embedded tab", "Foo\tBar", "FooBar"},
		{"embedded newline", "Foo\nBar", "FooBar"},
		{"embedded carriage return", "Foo\rBar", "FooBar"},
		{"leading/trailing tab and newline", "\t\nFoo Bar\n\t", "Foo Bar"},
		{"other control characters", "Foo\x00\x1bBar", "FooBar"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeSessionLabel(tc.label)
			if got != tc.want {
				t.Fatalf("NormalizeSessionLabel(%q) = %q, want %q", tc.label, got, tc.want)
			}
			if strings.ContainsAny(got, "\t\n\r") {
				t.Fatalf("NormalizeSessionLabel(%q) = %q, still contains a delimiter character", tc.label, got)
			}
		})
	}
}
