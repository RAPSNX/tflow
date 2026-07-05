package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("it's"), `'it'"'"'s'`; got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}

func TestAttachCommandUsesTflowSocket(t *testing.T) {
	cmd, err := tmuxSessionManager{}.AttachCommand("dev")
	if err != nil {
		t.Fatalf("AttachCommand returned error: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-L tflow") {
		t.Fatalf("attach command = %q, want tflow socket", got)
	}
}

func TestEnsureControlModeBindsToggleKey(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.EnsureControlMode("/tmp/tflow"); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-g", "default-shell", "/bin/zsh"},
		{"set-option", "-g", "default-command", "exec '/bin/zsh' -l"},
		{"bind-key", "-n", "C-f", "run-shell", "exec '/tmp/tflow' toggle-menu"},
	}
	for _, want := range wants {
		found := false
		for _, call := range calls {
			if strings.Join(call, "\x00") == strings.Join(want, "\x00") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing call %v in %#v", want, calls)
		}
	}
}

func TestToggleMenuKillsExistingPane(t *testing.T) {
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				return "@1", nil
			case "list-panes":
				return "%5\t1\n", nil
			case "kill-pane":
				if args[2] != "%5" {
					t.Fatalf("kill-pane target = %q", args[2])
				}
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}
}
