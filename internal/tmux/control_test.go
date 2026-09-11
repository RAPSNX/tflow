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
		Yellow:   "#f9e2af",
	}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-interval", "2"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left-length", "200"},
		{"set-option", "-g", "status-right-length", "30"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] session #[fg=#94e2d5]#{?@tflow-session-label,#{@tflow-session-label},#S} #[bg=#181825,fg=#313244,nobold]"},
		{"set-option", "-g", "status-right", "#{?#{==:#{client_key_table},tflow-command},#[fg=#f9e2af]#[bg=#181825]#[bg=#f9e2af]#[fg=#181825]#[bold] COMMAND #[nobold]#[fg=#f9e2af]#[bg=#181825]#[default],}#(TFLOW_CURRENT_SESSION='#{session_name}' exec '/tmp/tflow' attention-scan)"},
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
		{"set-window-option", "-g", "monitor-activity", "on"},
		{"set-hook", "-g", "alert-activity", "run-shell " + ShellQuote("TFLOW_CURRENT_SESSION='#{session_name}' exec '/tmp/tflow' session-activity")},
		{"set-hook", "-g", "client-session-changed", "run-shell " + ShellQuote("TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_LAST_VISITED_SESSION='#{client_last_session}' exec '/tmp/tflow' session-visited")},
		{"unbind-key", "-q", "-n", "C-f"},
		{"bind-key", "-n", "C-Space", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-command-menu"},
		{"bind-key", "-T", "tflow-command", "Escape", "switch-client", "-T", "root"},
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

func TestEnsureControlModeInstallsAttentionHooks(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	monitorOn := false
	for _, call := range calls {
		if len(call) == 4 && call[0] == "set-window-option" && call[1] == "-g" && call[2] == "monitor-activity" && call[3] == "on" {
			monitorOn = true
		}
	}
	if !monitorOn {
		t.Fatalf("missing global monitor-activity on in %#v", calls)
	}

	hasHook := func(name, subcommand string) bool {
		for _, call := range calls {
			if len(call) != 4 || call[0] != "set-hook" || call[1] != "-g" || call[2] != name {
				continue
			}
			if strings.Contains(call[3], "run-shell") && strings.Contains(call[3], CurrentSessionEnv) && strings.Contains(call[3], subcommand) {
				return true
			}
		}
		return false
	}
	if !hasHook("alert-activity", "session-activity") {
		t.Fatalf("missing alert-activity hook for session-activity in %#v", calls)
	}
	if !hasHook("client-session-changed", "session-visited") {
		t.Fatalf("missing client-session-changed hook for session-visited in %#v", calls)
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

func TestSetSessionAttention(t *testing.T) {
	var got []string
	manager := Manager{Run: func(args ...string) (string, error) {
		got = append([]string(nil), args...)
		return "", nil
	}}

	if err := manager.SetSessionAttention("tflow-p-1", true); err != nil {
		t.Fatalf("SetSessionAttention error: %v", err)
	}
	want := []string{"set-option", "-t", "tflow-p-1", "@tflow-attention", "1"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("call = %#v, want %#v", got, want)
	}

	if err := manager.SetSessionAttention("tflow-p-1", false); err != nil {
		t.Fatalf("SetSessionAttention error: %v", err)
	}
	want = []string{"set-option", "-t", "tflow-p-1", "@tflow-attention", "0"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("call = %#v, want %#v", got, want)
	}

	if err := manager.SetSessionAttention("", true); err == nil {
		t.Fatal("expected error for empty session name")
	}
}

func TestMarkSessionVisitedClearsAttentionAndStampsWatermark(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if args[0] == "list-windows" {
			// Two windows; the watermark must be the max across them, not
			// wall-clock time, so a plain "activity > watermark" comparison
			// stays unambiguous even when a visit and some activity land in
			// the same one-second tmux clock tick.
			return "150\n300\n", nil
		}
		return "", nil
	}}

	if err := manager.MarkSessionVisited("tflow-p-1"); err != nil {
		t.Fatalf("MarkSessionVisited error: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("calls = %#v, want exactly 3 tmux calls", calls)
	}
	wantClear := []string{"set-option", "-t", "tflow-p-1", "@tflow-attention", "0"}
	if strings.Join(calls[0], " ") != strings.Join(wantClear, " ") {
		t.Fatalf("first call = %#v, want %#v", calls[0], wantClear)
	}
	wantQuery := []string{"list-windows", "-t", "tflow-p-1", "-F", "#{window_activity}"}
	if strings.Join(calls[1], " ") != strings.Join(wantQuery, " ") {
		t.Fatalf("second call = %#v, want %#v", calls[1], wantQuery)
	}
	wantStamp := []string{"set-option", "-t", "tflow-p-1", "@tflow-visited-at", "300"}
	if strings.Join(calls[2], " ") != strings.Join(wantStamp, " ") {
		t.Fatalf("third call = %#v, want %#v", calls[2], wantStamp)
	}

	if err := manager.MarkSessionVisited(""); err == nil {
		t.Fatal("expected error for empty session name")
	}
}

func topBarPalette() Palette {
	return Palette{
		Surface0: "#313244",
		Subtext:  "#a6adc8",
		Text:     "#cdd6f4",
		Blue:     "#89b4fa",
		Mantle:   "#181825",
	}
}

func TestFormatTopBar(t *testing.T) {
	p := topBarPalette()

	// 0 sessions
	if got := p.FormatTopBar("demo", nil, nil, nil, 0); got != "" {
		t.Fatalf("FormatTopBar(nil) = %q, want empty", got)
	}

	// 1 session (alone)
	single := p.FormatTopBar("demo", []string{"only"}, nil, nil, 0)
	if !strings.Contains(single, "only") {
		t.Fatalf("single FormatTopBar = %q", single)
	}
	if strings.Count(single, "only") != 1 {
		t.Fatalf("expected single label to appear exactly once, got: %q", single)
	}
	if strings.Count(single, "\ue0b6") != 2 || strings.Count(single, "\ue0b4") != 1 {
		t.Fatalf("single FormatTopBar should have a project pill and one session pill: %q", single)
	}

	// 2 sessions, first active: each section appears once
	twoFirst := p.FormatTopBar("demo", []string{"first", "second"}, nil, nil, 0)
	if strings.Count(twoFirst, "first") != 1 || strings.Count(twoFirst, "second") != 1 {
		t.Fatalf("2 sessions (first active) should each appear once: %q", twoFirst)
	}
	firstIdx := strings.Index(twoFirst, "first")
	secondIdx := strings.Index(twoFirst, "second")
	if !(firstIdx < secondIdx) {
		t.Fatalf("expected first < second, got: %q", twoFirst)
	}
	if !strings.Contains(twoFirst, "#[bg=#313244,fg=#cdd6f4,bold] #[fg=#89b4fa]>_#[fg=#a6adc8] first #[bg=#181825,fg=#313244,nobold]") {
		t.Fatalf("first should be formatted as active pill: %q", twoFirst)
	}

	// 2 sessions, second active: each section appears once
	twoSecond := p.FormatTopBar("demo", []string{"first", "second"}, nil, nil, 1)
	if strings.Count(twoSecond, "first") != 1 || strings.Count(twoSecond, "second") != 1 {
		t.Fatalf("2 sessions (second active) should each appear once: %q", twoSecond)
	}
	firstIdx = strings.Index(twoSecond, "first")
	secondIdx = strings.Index(twoSecond, "second")
	if !(firstIdx < secondIdx) {
		t.Fatalf("expected first < second, got: %q", twoSecond)
	}
	if !strings.Contains(twoSecond, "#[bg=#313244,fg=#cdd6f4,bold] #[fg=#89b4fa]>_#[fg=#a6adc8] second #[bg=#181825,fg=#313244,nobold]") {
		t.Fatalf("second should be formatted as active pill: %q", twoSecond)
	}

	// 3 sessions, middle active
	three := p.FormatTopBar("demo", []string{"first", "second", "third"}, nil, nil, 1)
	if strings.Count(three, "first") != 1 || strings.Count(three, "second") != 1 || strings.Count(three, "third") != 1 {
		t.Fatalf("three FormatTopBar should contain each session once: %q", three)
	}
	firstIdx = strings.Index(three, "first")
	secondIdx = strings.Index(three, "second")
	thirdIdx := strings.Index(three, "third")
	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Fatalf("expected order first < second < third, got: %q", three)
	}
	if !strings.Contains(three, "#[bg=#313244,fg=#cdd6f4,bold] #[fg=#89b4fa]>_#[fg=#a6adc8] second #[bg=#181825,fg=#313244,nobold]") {
		t.Fatalf("second should be formatted as active pill: %q", three)
	}

	// 4 sessions, end active
	four := p.FormatTopBar("demo", []string{"s1", "s2", "s3", "s4"}, nil, nil, 3)
	for _, s := range []string{"s1", "s2", "s3", "s4"} {
		if strings.Count(four, s) != 1 {
			t.Fatalf("session %q should appear exactly once in %q", s, four)
		}
	}
	if !strings.Contains(four, "#[bg=#313244,fg=#cdd6f4,bold] #[fg=#89b4fa]>_#[fg=#a6adc8] s4 #[bg=#181825,fg=#313244,nobold]") {
		t.Fatalf("s4 should be active pill: %q", four)
	}
}

func TestFormatTopBarRendersProjectSectionBeforeSessions(t *testing.T) {
	p := topBarPalette()

	got := p.FormatTopBar("demo", []string{"code", "git"}, nil, nil, 0)
	want := "#[bg=#313244,fg=#89b4fa,bold] demo #[bg=#181825,fg=#313244,nobold]\ue0b0"
	if !strings.Contains(got, want) {
		t.Fatalf("FormatTopBar() = %q, want project section %q", got, want)
	}
	if strings.Index(got, "demo") > strings.Index(got, "code") {
		t.Fatalf("project section should precede the sessions: %q", got)
	}
}

func TestFormatTopBarRendersEmptyProjectSectionWithoutProject(t *testing.T) {
	p := topBarPalette()

	got := p.FormatTopBar("", []string{"scratch"}, nil, nil, 0)
	want := "#[bg=#313244,fg=#89b4fa,bold]  #[bg=#181825,fg=#313244,nobold]\ue0b0"
	if !strings.Contains(got, want) {
		t.Fatalf("FormatTopBar() = %q, want empty project section %q", got, want)
	}
}

func TestFormatTopBarRendersTypeIconsWithoutWordedChip(t *testing.T) {
	p := Palette{Surface0: "#313244", Subtext: "#a6adc8", Text: "#cdd6f4", Blue: "#89b4fa", Teal: "#94e2d5", Yellow: "#f9e2af", Mantle: "#181825"}

	got := p.FormatTopBar("demo", []string{"code", "git", "agent"}, []string{"", "git", "agent"}, nil, 0)
	if !strings.Contains(got, "#[fg=#89b4fa]>_#[fg=#a6adc8]") {
		t.Fatalf("missing terminal icon: %q", got)
	}
	if !strings.Contains(got, "#[fg=#94e2d5]\u2387#[fg=#a6adc8]") {
		t.Fatalf("missing git icon: %q", got)
	}
	if !strings.Contains(got, "#[fg=#f9e2af]\u2726#[fg=#a6adc8]") {
		t.Fatalf("missing agent icon: %q", got)
	}
	if strings.Contains(got, "CODE") || strings.Contains(got, "GIT") || strings.Contains(got, "AGENT") {
		t.Fatalf("top bar must not carry the worded sidebar chip: %q", got)
	}
}

func TestFormatTopBarRendersAttentionIndependentOfActiveAndType(t *testing.T) {
	p := topBarPalette()
	p.Red = "#f38ba8"

	got := p.FormatTopBar("demo", []string{"code", "git"}, []string{"", "git"}, []bool{false, true}, 0)
	if !strings.Contains(got, "#[fg=#f38ba8]!") {
		t.Fatalf("expected a red attention mark for the inactive git session, got: %q", got)
	}

	// Attention on the active pill too: independent of selection.
	got = p.FormatTopBar("demo", []string{"code", "git"}, []string{"", "git"}, []bool{true, false}, 0)
	if !strings.Contains(got, "#[fg=#f38ba8]!") {
		t.Fatalf("expected a red attention mark on the active session, got: %q", got)
	}

	// No attention anywhere: no red marks at all.
	got = p.FormatTopBar("demo", []string{"code", "git"}, []string{"", "git"}, []bool{false, false}, 0)
	if strings.Contains(got, "#f38ba8") {
		t.Fatalf("expected no attention mark, got: %q", got)
	}
}

func TestFormatTopBarEscapesTmuxFormatSyntaxInLabels(t *testing.T) {
	p := topBarPalette()

	got := p.FormatTopBar("demo", []string{"#(touch /tmp/tflow-review) #[fg=red]"}, nil, nil, 0)
	want := "##(touch /tmp/tflow-review) ##[fg=red]"
	if !strings.Contains(got, want) {
		t.Fatalf("FormatTopBar() = %q, want escaped label %q", got, want)
	}
}

func TestFormatTopBarEscapesTmuxFormatSyntaxInProject(t *testing.T) {
	p := topBarPalette()

	got := p.FormatTopBar("#(touch /tmp/tflow-review)", []string{"only"}, nil, nil, 0)
	if !strings.Contains(got, "##(touch /tmp/tflow-review)") {
		t.Fatalf("FormatTopBar() = %q, want escaped project name", got)
	}
	if strings.Contains(got, " #(touch") {
		t.Fatalf("FormatTopBar() = %q, left an unescaped project format", got)
	}
}
