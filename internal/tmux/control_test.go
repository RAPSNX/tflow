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
		{"set-option", "-g", "mouse", "on"},
		{"unbind-key", "-q", "-n", "MouseDown1Pane"},
		{"unbind-key", "-q", "-n", "MouseDown2Pane"},
		{"unbind-key", "-q", "-n", "MouseDown3Pane"},
		{"unbind-key", "-q", "-n", "MouseDrag1Pane"},
		{"unbind-key", "-q", "-n", "DoubleClick1Pane"},
		{"unbind-key", "-q", "-n", "TripleClick1Pane"},
		{"unbind-key", "-q", "-n", "MouseDown1Status"},
		{"unbind-key", "-q", "-n", "MouseDown3Status"},
		{"unbind-key", "-q", "-n", "MouseDown3StatusLeft"},
		{"unbind-key", "-q", "-n", "MouseDrag1Status"},
		{"unbind-key", "-q", "-n", "WheelUpStatus"},
		{"unbind-key", "-q", "-n", "WheelDownStatus"},
		{"unbind-key", "-q", "-T", "copy-mode", "MouseDown1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode", "MouseDrag1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode", "MouseDragEnd1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode", "DoubleClick1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode", "TripleClick1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode-vi", "MouseDown1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode-vi", "MouseDrag1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode-vi", "MouseDragEnd1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode-vi", "DoubleClick1Pane"},
		{"unbind-key", "-q", "-T", "copy-mode-vi", "TripleClick1Pane"},
		{"bind-key", "-n", "WheelUpPane", "if-shell", "-F", "#{||:#{alternate_on},#{pane_in_mode},#{mouse_any_flag}}", "send-keys -M", "copy-mode -e"},
		{"bind-key", "-T", "copy-mode", "WheelUpPane", "send-keys", "-X", "-N", "5", "scroll-up"},
		{"bind-key", "-T", "copy-mode", "WheelDownPane", "send-keys", "-X", "-N", "5", "scroll-down"},
		{"bind-key", "-T", "copy-mode-vi", "WheelUpPane", "send-keys", "-X", "-N", "5", "scroll-up"},
		{"bind-key", "-T", "copy-mode-vi", "WheelDownPane", "send-keys", "-X", "-N", "5", "scroll-down"},
		{"unbind-key", "-q", "-n", "C-f"},
		{"bind-key", "-n", "C-Space", "switch-client", "-T", "tflow-command"},
		{"bind-key", "-T", "tflow-command", "h", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' navigate-prev"},
		{"bind-key", "-T", "tflow-command", "l", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' navigate-next"},
		{"bind-key", "-T", "tflow-command", "o", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-menu"},
		{"bind-key", "-T", "tflow-command", "Escape", "switch-client", "-T", "root"},
		{"bind-key", "-T", "tflow-command", "C-c", "switch-client", "-T", "root"},
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

func TestEnsureControlModeDoesNotBindCtrlF(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	for _, call := range calls {
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-n" && call[2] == "C-f" {
			t.Fatalf("EnsureControlMode must not bind C-f: %#v", call)
		}
	}
}

func TestSetSessionTopBar(t *testing.T) {
	var got []string
	manager := Manager{Run: func(args ...string) (string, error) {
		got = append([]string(nil), args...)
		return "", nil
	}}

	if err := manager.SetSessionTopBar("tflow-p-1", "top-bar-content"); err != nil {
		t.Fatalf("SetSessionTopBar error: %v", err)
	}
	want := []string{"set-option", "-t", "tflow-p-1", "status-left", "top-bar-content"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("call = %#v, want %#v", got, want)
	}

	if err := manager.SetSessionTopBar("", "content"); err == nil {
		t.Fatal("expected error for empty session name")
	}
}

func TestFormatTopBar(t *testing.T) {
	p := Palette{
		Surface0: "#313244",
		Subtext:  "#a6adc8",
		Text:     "#cdd6f4",
		Mantle:   "#181825",
	}

	// 0 sessions
	if got := p.FormatTopBar(nil, 0); got != "" {
		t.Fatalf("FormatTopBar(nil) = %q, want empty", got)
	}

	// 1 session (alone)
	single := p.FormatTopBar([]string{"only"}, 0)
	if !strings.Contains(single, "only") || strings.Contains(single, "prev") {
		t.Fatalf("single FormatTopBar = %q", single)
	}
	if strings.Count(single, "") != 1 || strings.Count(single, "") != 1 {
		t.Fatalf("single FormatTopBar should have exactly one pill: %q", single)
	}

	// 3 sessions, middle active
	three := p.FormatTopBar([]string{"first", "second", "third"}, 1)
	if !strings.Contains(three, "first") || !strings.Contains(three, "second") || !strings.Contains(three, "third") {
		t.Fatalf("three FormatTopBar = %q", three)
	}
	// Verify previous is "first", active is "second", next is "third"
	firstIdx := strings.Index(three, "first")
	secondIdx := strings.Index(three, "second")
	thirdIdx := strings.Index(three, "third")
	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Fatalf("expected order first < second < third, got indices: %d, %d, %d in %q", firstIdx, secondIdx, thirdIdx, three)
	}

	// 3 sessions, first active (wraparound previous is third)
	wrap := p.FormatTopBar([]string{"first", "second", "third"}, 0)
	thirdWrapIdx := strings.Index(wrap, "third")
	firstWrapIdx := strings.Index(wrap, "first")
	secondWrapIdx := strings.Index(wrap, "second")
	if !(thirdWrapIdx < firstWrapIdx && firstWrapIdx < secondWrapIdx) {
		t.Fatalf("expected wraparound third < first < second, got indices: %d, %d, %d in %q", thirdWrapIdx, firstWrapIdx, secondWrapIdx, wrap)
	}
}
