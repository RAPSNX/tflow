package tmux

import (
	"fmt"
	"os"
	"strings"
)

func (m Manager) EnsureControlMode(binaryPath string, palette Palette) error {
	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	runShell := "exec " + ShellQuote(binaryPath) + " toggle-menu"
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
		{"set-option", "-g", "default-shell", userShell()},
		{"set-option", "-g", "default-command", loginShellCommand()},
		{"bind-key", "-n", menuToggleKey, "run-shell", runShell},
	}
	for _, args := range commands {
		if _, err := m.runner()(args...); err != nil && !IsNoServer(err) {
			return err
		}
	}
	return nil
}

func (m Manager) ToggleMenu(binaryPath string) error {
	windowID, err := m.currentValue("#{window_id}")
	if err != nil {
		return err
	}

	if paneID, ok, err := m.menuPane(windowID); err != nil {
		return err
	} else if ok {
		return m.ClosePane(paneID)
	}

	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	currentSession, err := m.currentValue("#{session_name}")
	if err != nil {
		return err
	}
	currentClient, err := m.currentValue("#{client_id}")
	if err != nil {
		return err
	}

	menuCommand := fmt.Sprintf("%s=%s %s=%s exec %s menu", CurrentSessionEnv, ShellQuote(currentSession), CurrentClientEnv, ShellQuote(currentClient), ShellQuote(binaryPath))
	paneID, err := m.runner()("split-window", "-t", windowID, "-h", "-b", "-l", menuWidth, "-P", "-F", "#{pane_id}", menuCommand)
	if err != nil {
		return err
	}
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return fmt.Errorf("tmux did not return a menu pane id")
	}
	if _, err := m.runner()("set-option", "-p", "-t", paneID, menuMarker, "1"); err != nil {
		return err
	}
	return nil
}

func (m Manager) ClosePane(paneID string) error {
	if strings.TrimSpace(paneID) == "" {
		return nil
	}
	_, err := m.runner()("kill-pane", "-t", paneID)
	return err
}

func (m Manager) QuitAll(paneID string) error {
	script := []string{}
	if trimmed := strings.TrimSpace(paneID); trimmed != "" {
		script = append(script, "tmux -L "+ShellQuote(socketName)+" kill-pane -t "+ShellQuote(trimmed)+" >/dev/null 2>&1")
	}
	if clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv)); clientID != "" {
		script = append(script, "tmux -L "+ShellQuote(socketName)+" detach-client -t "+ShellQuote(clientID)+" >/dev/null 2>&1")
	} else {
		script = append(script, "tmux -L "+ShellQuote(socketName)+" detach-client >/dev/null 2>&1")
	}
	_, err := m.runner()("run-shell", strings.Join(script, "; "))
	return err
}

func (m Manager) currentValue(format string) (string, error) {
	out, err := m.runner()("display-message", "-p", format)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m Manager) menuPane(windowID string) (string, bool, error) {
	out, err := m.runner()("list-panes", "-t", windowID, "-F", "#{pane_id}\t#{"+menuMarker+"}")
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "1" {
			return strings.TrimSpace(parts[0]), true, nil
		}
	}
	return "", false, nil
}
