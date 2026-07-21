package tmux

import (
	"strings"
	"testing"
)

func TestEnsureControlModeBindsToggleKey(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{
		Surface0: "#313244",
		Subtext:  "#a6adc8",
		Text:     "#cdd6f4",
		Blue:     "#89b4fa",
		Mantle:   "#181825",
		Teal:     "#94e2d5",
	}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] session #[fg=#94e2d5]#{?@tflow-session-label,#{@tflow-session-label},#S} #[bg=#181825,fg=#313244,nobold]"},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
		{"set-window-option", "-g", "remain-on-exit", "on"},
		{"set-option", "-g", "default-shell", "/bin/zsh"},
		{"set-option", "-g", "default-command", "exec '/bin/zsh' -l"},
		{"bind-key", "-n", "C-f", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-menu"},
		{"bind-key", "-n", "C-q", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' open-quit"},
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

func TestEnsureControlModeDoesNotBakeProcessInstanceEnvIntoToggleKey(t *testing.T) {
	t.Setenv(CurrentInstanceEnv, "instance-1")

	var got []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			if len(args) >= 4 && args[0] == "bind-key" {
				got = append([]string(nil), args...)
			}
			return "", nil
		},
	}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}
	if len(got) < 5 {
		t.Fatalf("bind args = %#v, want bind-key command", got)
	}
	if strings.Contains(got[4], CurrentInstanceEnv+"='instance-1'") {
		t.Fatalf("bind args = %#v, should not bake process instance env into the shared key binding", got)
	}
}

func TestEnsureControlModeInstallsClientLifecycleHooks(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	found := false
	for _, call := range calls {
		if len(call) != 4 || call[0] != "set-hook" || call[1] != "-g" || call[2] != "client-detached" {
			continue
		}
		if strings.Contains(call[3], "run-shell") && strings.Contains(call[3], CurrentSessionEnv) && strings.Contains(call[3], CurrentClientEnv) && strings.Contains(call[3], "cleanup-client") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing client-detached hook for cleanup-client in %#v", calls)
	}

	for _, call := range calls {
		if len(call) == 4 && call[0] == "set-hook" && call[2] == "client-attached" {
			t.Fatalf("client-attached hook must not be installed; instance ID is resolved from the session, not remembered per client: %#v", call)
		}
	}
}
