package tmux

import (
	"fmt"
	"strings"
)

func (m Manager) EnsureControlMode(binaryPath string, palette Palette) error {
	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	parts := []string{
		fmt.Sprintf("%s=%s", CurrentSessionEnv, ShellQuote("#{session_name}")),
		fmt.Sprintf("%s=%s", CurrentClientEnv, ShellQuote("#{client_name}")),
	}
	toggleCommandShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" toggle-command-menu"), " ")
	quitShell := strings.Join(append(parts, "exec "+ShellQuote(binaryPath)+" open-quit"), " ")
	cleanupClientShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" cleanup-client"), " ")
	sessionOnlyPart := fmt.Sprintf("%s=%s", CurrentSessionEnv, ShellQuote("#{session_name}"))
	// navigate-prev/navigate-next/jump-git are already fully standalone --
	// they resolve the current session from this same env var, lazily
	// materialize their target if needed, and switch the client directly,
	// with no popup involved -- so binding them straight at the
	// prefixTable wait-state level (h/l/g fire immediately after Ctrl+F,
	// before f would open the popup) is a small, low-risk addition to a
	// path that already runs this exact shape for toggle-command-menu.
	// They also need CurrentClientEnv forwarded, like every other action
	// here that ends up calling SwitchClient: without it, Manager.SwitchClient
	// falls back to `switch-client -t` with no `-c`, and tmux may pick an
	// arbitrary attached client rather than the one that actually pressed
	// the key, when more than one client is attached.
	navigatePrevShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" navigate-prev"), " ")
	navigateNextShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" navigate-next"), " ")
	jumpGitShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" jump-git"), " ")
	sessionActivityShell := sessionOnlyPart + " exec " + ShellQuote(binaryPath) + " session-activity"
	sessionVisitedShell := sessionOnlyPart + " " +
		fmt.Sprintf("%s=%s", LastVisitedSessionEnv, ShellQuote("#{client_last_session}")) +
		" exec " + ShellQuote(binaryPath) + " session-visited"
	// attention-scan below is the mechanism the attention feature actually
	// depends on: it rides tmux's own status-interval timer -- a bounded,
	// tmux-native redraw tick, not a custom daemon -- to periodically set
	// the marker for any unvisited session with fresh output and refresh
	// this client's own visible top bar, so a sibling session's attention
	// reaches it without waiting for an unrelated switch/rename/etc. Its
	// output is discarded by the caller (status-right only substitutes it,
	// never displays it) via #(). The alert-activity hook above is kept as
	// a best-effort supplement on tmux builds where it fires.
	attentionScanShell := sessionOnlyPart + " exec " + ShellQuote(binaryPath) + " attention-scan"
	commands := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-interval", "2"},
		{"set-option", "-g", "status-style", palette.statusStyle()},
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left-length", "200"},
		{"set-option", "-g", "status-right-length", "30"},
		{"set-option", "-g", "status-left", palette.statusLeft()},
		{"set-option", "-g", "status-right", palette.statusRight() + "#(" + attentionScanShell + ")"},
		{"set-option", "-g", "window-status-separator", ""},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
		{"set-option", "-g", "detach-on-destroy", "off"},
		{"set-window-option", "-g", "remain-on-exit", "on"},
		{"set-option", "-g", "default-shell", userShell()},
		{"set-option", "-g", "default-command", loginShellCommand()},
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
		{"set-hook", "-g", "client-detached", "run-shell " + ShellQuote(cleanupClientShell)},
		{"set-window-option", "-g", "monitor-activity", "on"},
		{"set-hook", "-g", "alert-activity", "run-shell " + ShellQuote(sessionActivityShell)},
		{"set-hook", "-g", "client-session-changed", "run-shell " + ShellQuote(sessionVisitedShell)},
		{"bind-key", "-n", prefixKey, "switch-client", "-T", prefixTable},
		// Both "f" and a held "C-f" finish the chord -- someone tapping the
		// same key twice while physically holding Ctrl down never releases
		// it between presses, so the second key arrives as C-f, not f.
		{"bind-key", "-T", prefixTable, "f", "run-shell", toggleCommandShell},
		{"bind-key", "-T", prefixTable, prefixKey, "run-shell", toggleCommandShell},
		// h/l/g act immediately from the prefixTable wait-state, without
		// ever opening the popup -- see navigatePrevShell/navigateNextShell/
		// jumpGitShell's comment above.
		{"bind-key", "-T", prefixTable, "h", "run-shell", navigatePrevShell},
		{"bind-key", "-T", prefixTable, "l", "run-shell", navigateNextShell},
		{"bind-key", "-T", prefixTable, "g", "run-shell", jumpGitShell},
		{"bind-key", "-T", prefixTable, "Escape", "switch-client", "-T", "root"},
		{"bind-key", "-T", commandTable, "Escape", "switch-client", "-T", "root"},
		// commandTable (the popup-is-open table) has no fallback to root -n
		// bindings, so C-f alone does nothing while the popup is open unless
		// it's bound here too: repeating Ctrl+F, f to close the popup needs
		// this table to also enter prefixTable for the chord's second key.
		{"bind-key", "-T", commandTable, prefixKey, "switch-client", "-T", prefixTable},
		{"bind-key", "-n", quitKey, "run-shell", quitShell},
	}
	for _, args := range commands {
		if _, err := m.runner()(args...); err != nil && !IsNoServer(err) {
			return err
		}
	}
	return nil
}

func (m Manager) SetSessionTopBar(name, content string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("session name is empty")
	}
	_, err := m.runner()("set-option", "-t", name, "status-left", content)
	return err
}

// SetSessionAttention sets or clears the runtime-only session attention
// marker. It is never written to JSON and is not expected to survive a
// tmux restart.
func (m Manager) SetSessionAttention(name string, attention bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("session name is empty")
	}
	value := "0"
	if attention {
		value = "1"
	}
	_, err := m.runner()("set-option", "-t", name, attentionMarker, value)
	return err
}
