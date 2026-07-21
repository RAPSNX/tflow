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
	toggleShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" toggle-menu"), " ")
	quitShell := strings.Join(append(parts, "exec "+ShellQuote(binaryPath)+" open-quit"), " ")
	cleanupClientShell := strings.Join(append(append([]string(nil), parts...), "exec "+ShellQuote(binaryPath)+" cleanup-client"), " ")
	commands := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", palette.statusStyle()},
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left-length", "120"},
		{"set-option", "-g", "status-right-length", "0"},
		{"set-option", "-g", "status-left", palette.statusLeft()},
		{"set-option", "-g", "status-right", ""},
		{"set-option", "-g", "window-status-separator", ""},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
		{"set-option", "-g", "detach-on-destroy", "off"},
		{"set-window-option", "-g", "remain-on-exit", "on"},
		{"set-option", "-g", "default-shell", userShell()},
		{"set-option", "-g", "default-command", loginShellCommand()},
		{"set-hook", "-g", "client-detached", "run-shell " + ShellQuote(cleanupClientShell)},
		{"bind-key", "-n", menuToggleKey, "run-shell", toggleShell},
		{"bind-key", "-n", quitKey, "run-shell", quitShell},
	}
	for _, args := range commands {
		if _, err := m.runner()(args...); err != nil && !IsNoServer(err) {
			return err
		}
	}
	return nil
}
