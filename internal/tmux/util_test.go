package tmux

import "testing"

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
