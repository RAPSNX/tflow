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

func TestNormalizeCWDExpandsHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	if got := normalizeCWD("~/project"); got != "/tmp/home/project" {
		t.Fatalf("normalizeCWD = %q, want /tmp/home/project", got)
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

func TestSwitchClientUsesExplicitClientWhenAvailable(t *testing.T) {
	t.Setenv(menuClientEnv, "@1")

	var got []string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			got = append([]string(nil), args...)
			return "", nil
		},
	}

	if err := manager.SwitchClient("dev"); err != nil {
		t.Fatalf("SwitchClient returned error: %v", err)
	}

	want := []string{"switch-client", "-c", "@1", "-t", "dev"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("switch-client args = %#v, want %#v", got, want)
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
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] session #[fg=#94e2d5]#S #[bg=#181825,fg=#313244,nobold]"},
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
		{"set-option", "-t", "api", "@tflow-project", ""},
		{"set-option", "-t", "blank", "@tflow-project", ""},
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

func TestListSessionsIncludesTemporaryMarker(t *testing.T) {
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			return "otter-temp\t1\t1\t1\nsmall\t2\t0\t0\n", nil
		},
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if !sessions[0].Temporary {
		t.Fatal("expected first session to be temporary")
	}
	if sessions[1].Temporary {
		t.Fatal("expected second session to be persistent")
	}
}

func TestSetSessionTemporaryTogglesTmuxOptions(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", true); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-t", "otter-temp", "client-attached", "set-option -t 'otter-temp' destroy-unattached on; set-hook -u -t 'otter-temp' client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "1"},
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

func TestSetSessionTemporaryClearsDeferredCleanupWhenMadePersistent(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", false); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-u", "-t", "otter-temp", "client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "0"},
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
				switch args[2] {
				case "#{window_id}":
					return "@1", nil
				case "#{session_name}":
					return "otter-temp", nil
				case "#{client_id}":
					return "@2", nil
				default:
					t.Fatalf("unexpected display-message format: %v", args)
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
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

func TestToggleMenuPassesCurrentClientToMenuProcess(t *testing.T) {
	var splitWindow []string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{window_id}":
					return "@1", nil
				case "#{session_name}":
					return "otter-temp", nil
				case "#{client_id}":
					return "@2", nil
				default:
					t.Fatalf("unexpected display-message format: %v", args)
				}
			case "list-panes":
				return "", nil
			case "split-window":
				splitWindow = append([]string(nil), args...)
				return "%7", nil
			case "set-option":
				return "", nil
			default:
				t.Fatalf("unexpected command: %v", args)
			}
			return "", nil
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	got := strings.Join(splitWindow, " ")
	if !strings.Contains(got, "TFLOW_CURRENT_CLIENT='@2'") {
		t.Fatalf("split-window command = %q, want current client env", got)
	}
}

func TestQuitAllDetachesExplicitClientWhenAvailable(t *testing.T) {
	t.Setenv(menuClientEnv, "@2")

	var got []string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			got = append([]string(nil), args...)
			return "", nil
		},
	}

	if err := manager.QuitAll("%7"); err != nil {
		t.Fatalf("QuitAll returned error: %v", err)
	}

	if len(got) != 2 || got[0] != "run-shell" {
		t.Fatalf("run command = %#v, want run-shell", got)
	}
	if !strings.Contains(got[1], "detach-client -t '@2'") {
		t.Fatalf("run-shell script = %q, want explicit client detach", got[1])
	}
}
