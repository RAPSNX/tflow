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
		{"set-option", "-g", "status-left", "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#89b4fa,bold] #{@tflow-project} #[bg=#181825,fg=#313244,nobold]\ue0b4  #[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#94e2d5,bold] #{?@tflow-session-label,#{@tflow-session-label},#S} #[bg=#181825,fg=#313244,nobold]\ue0b4"},
		{"set-option", "-g", "status-right", "#{?#{==:#{client_key_table},tflow-prefix},#[fg=#f9e2af]#[bg=#181825]#[bg=#f9e2af]#[fg=#181825]#[bold] COMMAND #[nobold]#[fg=#f9e2af]#[bg=#181825]#[default],}#(TFLOW_CURRENT_SESSION='#{session_name}' exec '/tmp/tflow' attention-scan)"},
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
		{"bind-key", "-n", "C-f", "switch-client", "-T", "tflow-prefix"},
		{"bind-key", "-T", "tflow-prefix", "f", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-command-menu"},
		{"bind-key", "-T", "tflow-prefix", "C-f", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-command-menu"},
		{"bind-key", "-T", "tflow-prefix", "Escape", "switch-client", "-T", "root"},
		{"bind-key", "-T", "tflow-command", "Escape", "switch-client", "-T", "root"},
		{"bind-key", "-T", "tflow-command", "C-f", "switch-client", "-T", "tflow-prefix"},
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

// TestEnsureControlModeBindsCtrlFPrefixChord guards the C-f, f chord: C-f at
// the root table only switches the client into a dedicated wait-state table
// (tflow-prefix) -- mirroring how tmux's own prefix key works, auto-reverting
// after one keypress -- rather than opening the popup directly. Within that
// table, either "f" or a held "C-f" toggles it -- someone physically holding
// Ctrl through both presses never releases it, so the second key can arrive
// as C-f instead of f.
func TestEnsureControlModeBindsCtrlFPrefixChord(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	foundPrefixSwitch := false
	for _, call := range calls {
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-n" && call[2] == "C-f" {
			foundPrefixSwitch = true
			if len(call) < 6 || call[3] != "switch-client" || call[4] != "-T" || call[5] != "tflow-prefix" {
				t.Fatalf("root C-f binding = %#v, want a switch-client into tflow-prefix, not a direct toggle", call)
			}
		}
		// C-f must never be bound to open the popup directly at the root
		// table -- only the chord's second key, "f" inside tflow-prefix,
		// does that (checked separately below).
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-n" && call[2] == "C-f" && len(call) >= 5 && call[3] == "run-shell" {
			t.Fatalf("C-f must not directly open the popup: %#v", call)
		}
	}
	if !foundPrefixSwitch {
		t.Fatal("EnsureControlMode did not bind C-f to enter tflow-prefix")
	}

	foundToggle := false
	foundHeldToggle := false
	for _, call := range calls {
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-T" && call[2] == "tflow-prefix" && call[3] == "f" {
			foundToggle = true
		}
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-T" && call[2] == "tflow-prefix" && call[3] == "C-f" {
			foundHeldToggle = true
		}
	}
	if !foundToggle {
		t.Fatal("EnsureControlMode did not bind \"f\" inside tflow-prefix to toggle the popup")
	}
	if !foundHeldToggle {
		t.Fatal("EnsureControlMode did not bind a held \"C-f\" inside tflow-prefix to toggle the popup")
	}

	// commandTable (the popup-is-open table) has no fallback to root -n
	// bindings -- verified live, C-f does nothing there unless it's also
	// bound in this table -- so repeating Ctrl+F, f to close the popup
	// needs commandTable to also switch into tflow-prefix on C-f.
	foundCommandTablePrefixSwitch := false
	for _, call := range calls {
		if len(call) >= 4 && call[0] == "bind-key" && call[1] == "-T" && call[2] == "tflow-command" && call[3] == "C-f" {
			foundCommandTablePrefixSwitch = true
			if len(call) < 6 || call[4] != "switch-client" || call[5] != "-T" {
				t.Fatalf("commandTable C-f binding = %#v, want a switch-client into tflow-prefix", call)
			}
		}
	}
	if !foundCommandTablePrefixSwitch {
		t.Fatal("EnsureControlMode did not bind C-f inside tflow-command to re-enter tflow-prefix")
	}
}

// TestEnsureControlModeBindsQuickCommandModeActions guards item D: h/l/g
// must fire immediately from the tflow-prefix wait-state itself (not from
// commandTable, which is only entered once the popup is already open), each
// via run-shell invoking the tflow binary's own navigate-prev/navigate-next/
// jump-git subcommands -- the same standalone, no-popup shape
// toggle-command-menu already uses for "f".
func TestEnsureControlModeBindsQuickCommandModeActions(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := map[string]string{
		"h": "navigate-prev",
		"l": "navigate-next",
		"g": "jump-git",
	}
	for key, subcommand := range wants {
		found := false
		for _, call := range calls {
			if len(call) >= 6 && call[0] == "bind-key" && call[1] == "-T" && call[2] == prefixTable && call[3] == key {
				found = true
				if call[4] != "run-shell" {
					t.Fatalf("tflow-prefix %q binding = %#v, want run-shell, not a direct tmux command (must not open the popup)", key, call)
				}
				if !strings.Contains(call[5], "'/tmp/tflow' "+subcommand) {
					t.Fatalf("tflow-prefix %q run-shell script = %q, want it to invoke %q", key, call[5], subcommand)
				}
				if !strings.Contains(call[5], CurrentSessionEnv+"=") {
					t.Fatalf("tflow-prefix %q run-shell script = %q, want the current session env forwarded", key, call[5])
				}
				// Every action that ends up calling SwitchClient needs the
				// originating client forwarded too, or Manager.SwitchClient
				// falls back to a client-less switch-client that tmux may
				// route to an arbitrary attached client instead.
				if !strings.Contains(call[5], CurrentClientEnv+"=") {
					t.Fatalf("tflow-prefix %q run-shell script = %q, want the current client env forwarded so SwitchClient stays client-scoped", key, call[5])
				}
			}
		}
		if !found {
			t.Fatalf("EnsureControlMode did not bind %q inside tflow-prefix", key)
		}
	}

	// These three must never be bound inside commandTable (the popup-is-open
	// table) or at the root (-n) level -- only from the brief tflow-prefix
	// wait-state, before the popup ever opens.
	for _, call := range calls {
		if len(call) < 4 || call[0] != "bind-key" {
			continue
		}
		var key string
		switch {
		case call[1] == "-T" && call[2] == commandTable:
			key = call[3]
		case call[1] == "-n":
			key = call[2]
		default:
			continue
		}
		if key == "h" || key == "l" || key == "g" {
			t.Fatalf("quick command-mode action %q must only be bound inside tflow-prefix, found elsewhere: %#v", key, call)
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
		Green:    "#a6da95",
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
	if !strings.Contains(single, "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#a6adc8] #[fg=#a6da95]>_#[fg=#a6adc8] only #[bg=#181825,fg=#313244,nobold]\ue0b4") {
		t.Fatalf("single FormatTopBar should render the lone (active) session as a filled, green-icon pill: %q", single)
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
	if !strings.Contains(twoFirst, "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#a6adc8] #[fg=#a6da95]>_#[fg=#a6adc8] first #[bg=#181825,fg=#313244,nobold]\ue0b4") {
		t.Fatalf("first should be formatted as an active, filled, green-icon pill: %q", twoFirst)
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
	if !strings.Contains(twoSecond, "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#a6adc8] #[fg=#a6da95]>_#[fg=#a6adc8] second #[bg=#181825,fg=#313244,nobold]\ue0b4") {
		t.Fatalf("second should be formatted as an active, filled, green-icon pill: %q", twoSecond)
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
	if !strings.Contains(three, "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#a6adc8] #[fg=#a6da95]>_#[fg=#a6adc8] second #[bg=#181825,fg=#313244,nobold]\ue0b4") {
		t.Fatalf("second should be formatted as an active, filled, green-icon pill: %q", three)
	}

	// 4 sessions, end active
	four := p.FormatTopBar("demo", []string{"s1", "s2", "s3", "s4"}, nil, nil, 3)
	for _, s := range []string{"s1", "s2", "s3", "s4"} {
		if strings.Count(four, s) != 1 {
			t.Fatalf("session %q should appear exactly once in %q", s, four)
		}
	}
	if !strings.Contains(four, "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#a6adc8] #[fg=#a6da95]>_#[fg=#a6adc8] s4 #[bg=#181825,fg=#313244,nobold]\ue0b4") {
		t.Fatalf("s4 should be an active, filled, green-icon pill: %q", four)
	}
}

func TestFormatTopBarRendersProjectSectionBeforeSessions(t *testing.T) {
	p := topBarPalette()

	got := p.FormatTopBar("demo", []string{"code", "git"}, nil, nil, 0)
	want := "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#89b4fa,bold] demo #[bg=#181825,fg=#313244,nobold]\ue0b4  "
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
	want := "#[bg=#181825,fg=#313244]\ue0b6#[bg=#313244,fg=#89b4fa,bold]  #[bg=#181825,fg=#313244,nobold]\ue0b4  "
	if !strings.Contains(got, want) {
		t.Fatalf("FormatTopBar() = %q, want empty project section %q", got, want)
	}
}

func TestFormatTopBarRendersTypeIconsWithoutWordedChip(t *testing.T) {
	p := Palette{Surface0: "#313244", Subtext: "#a6adc8", Text: "#cdd6f4", Blue: "#89b4fa", Teal: "#94e2d5", Yellow: "#f9e2af", Green: "#a6da95", Mantle: "#181825"}

	// "code" sits at the active index, so it renders green regardless of
	// type -- the other three are inactive and keep their by-type colour.
	got := p.FormatTopBar("demo", []string{"code", "git", "agent", "code2"}, []string{"", "git", "agent", ""}, nil, 0)
	if !strings.Contains(got, "#[fg=#a6da95]>_#[fg=#a6adc8] code") {
		t.Fatalf("missing green active icon: %q", got)
	}
	if !strings.Contains(got, "#[fg=#89b4fa]>_#[fg=#a6adc8] code2") {
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
