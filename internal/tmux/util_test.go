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

func TestSessionNamesUseOpaquePrefixes(t *testing.T) {
	if got, want := PersistentSessionName("8f42ac91"), "tflow-p-8f42ac91"; got != want {
		t.Fatalf("PersistentSessionName = %q, want %q", got, want)
	}
	if got, want := VolatileSessionName("instance-1", "8f42ac91"), "tflow-v-instance-1-8f42ac91"; got != want {
		t.Fatalf("VolatileSessionName = %q, want %q", got, want)
	}
}
