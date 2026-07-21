package tmux

import (
	"strings"
	"testing"
)

func TestSocketArgsUsesRunningSocketPathWhenTmuxEnvMatches(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/tflow,49726,0")

	got := socketArgs()
	want := []string{"-S", "/tmp/tmux-1000/tflow"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("socketArgs() = %#v, want %#v", got, want)
	}
}

func TestSocketArgsFallsBackWhenTmuxEnvUnset(t *testing.T) {
	t.Setenv("TMUX", "")

	got := socketArgs()
	want := []string{"-L", socketName}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("socketArgs() = %#v, want %#v", got, want)
	}
}

func TestSocketArgsFallsBackForUnrelatedSocket(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1234,0")

	got := socketArgs()
	want := []string{"-L", socketName}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("socketArgs() = %#v, want %#v", got, want)
	}
}

func TestShellTmuxCommandUsesResolvedSocket(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/tflow,49726,0")

	got := shellTmuxCommand("display-popup", "-C", "-c", "@2")
	want := "tmux -S '/tmp/tmux-1000/tflow' 'display-popup' '-C' '-c' '@2'"
	if got != want {
		t.Fatalf("shellTmuxCommand() = %q, want %q", got, want)
	}
}

func TestShellTmuxCommandFallsBackToSocketNameWithoutTmuxEnv(t *testing.T) {
	t.Setenv("TMUX", "")

	got := shellTmuxCommand("display-popup", "-C", "-c", "@2")
	want := "tmux -L 'tflow' 'display-popup' '-C' '-c' '@2'"
	if got != want {
		t.Fatalf("shellTmuxCommand() = %q, want %q", got, want)
	}
}
