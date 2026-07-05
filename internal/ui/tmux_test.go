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
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] section #[fg=#94e2d5]#S #[bg=#181825,fg=#313244,nobold]"},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
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

func TestSyncSessionProjectsSetsProjectMarker(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	err := manager.SyncSessionProjects(map[string]string{
		"dev":   "small",
		"api":   "",
		"blank": "  ",
	})
	if err != nil {
		t.Fatalf("SyncSessionProjects returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "dev", "@tflow-project", "small"},
		{"set-option", "-t", "api", "@tflow-project", "default"},
		{"set-option", "-t", "blank", "@tflow-project", "default"},
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

func TestRenameSessionUsesTmuxRenameSession(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.RenameSession("dev", "lala"); err != nil {
		t.Fatalf("RenameSession returned error: %v", err)
	}

	want := []string{"rename-session", "-t", "dev", "lala"}
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
