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

	runShell := fmt.Sprintf("%s=%s %s=%s exec %s toggle-menu",
		CurrentSessionEnv, ShellQuote("#{session_name}"),
		CurrentClientEnv, ShellQuote("#{client_name}"),
		ShellQuote(binaryPath),
	)
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
	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	currentSession, err := m.contextValue(CurrentSessionEnv, "#{session_name}")
	if err != nil {
		return err
	}
	currentClient, err := m.contextValue(CurrentClientEnv, "#{client_name}")
	if err != nil {
		return err
	}

	visible, err := m.menuPopupVisible(currentClient)
	if err != nil {
		return err
	}
	if visible {
		return m.closeMenuPopup(currentClient)
	}
	if err := m.markMenuPopup(currentClient); err != nil {
		return err
	}

	_, err = m.runner()(
		"display-popup",
		"-c", currentClient,
		"-E",
		"-w", menuWidth,
		"-h", menuHeight,
		"-x", "#{popup_pane_left}",
		"-y", "C",
		"-e", fmt.Sprintf("%s=%s", CurrentSessionEnv, currentSession),
		"-e", fmt.Sprintf("%s=%s", CurrentClientEnv, currentClient),
		popupShellCommand(binaryPath, currentClient),
	)
	if err != nil {
		_ = m.unmarkMenuPopup(currentClient)
		return err
	}
	return nil
}

func (m Manager) CloseMenu() error {
	clientID, err := m.contextValue(CurrentClientEnv, "#{client_name}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return m.closeMenuPopup(clientID)
}

func (m Manager) QuitAll() error {
	clientID, err := m.contextValue(CurrentClientEnv, "#{client_name}")
	if err != nil {
		return err
	}
	script := []string{}
	if strings.TrimSpace(clientID) != "" {
		script = append(script, popupCloseScript(clientID))
		script = append(script, "tmux detach-client -t "+ShellQuote(clientID)+" >/dev/null 2>&1")
	} else {
		script = append(script, "tmux -L "+ShellQuote(socketName)+" detach-client >/dev/null 2>&1")
	}
	_, runErr := m.runner()("run-shell", strings.Join(script, "; "))
	return runErr
}

func (m Manager) currentValue(format string) (string, error) {
	out, err := m.runner()("display-message", "-p", format)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m Manager) contextValue(envName, format string) (string, error) {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value, nil
	}
	return m.currentValue(format)
}

func (m Manager) menuPopupVisible(clientID string) (bool, error) {
	if strings.TrimSpace(clientID) == "" {
		return false, nil
	}
	out, err := m.runner()("show-environment", "-gh")
	if err != nil {
		return false, err
	}
	key := popupEnvKey(clientID) + "="
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), key) {
			return true, nil
		}
	}
	return false, nil
}

func (m Manager) markMenuPopup(clientID string) error {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	_, err := m.runner()("set-environment", "-gh", popupEnvKey(clientID), "1")
	return err
}

func (m Manager) unmarkMenuPopup(clientID string) error {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	_, err := m.runner()("set-environment", "-gu", popupEnvKey(clientID))
	return err
}

func (m Manager) closeMenuPopup(clientID string) error {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	_, closeErr := m.runner()("display-popup", "-C", "-c", clientID)
	unmarkErr := m.unmarkMenuPopup(clientID)
	if closeErr != nil {
		if isBenignPopupCloseError(closeErr) {
			return unmarkErr
		}
		if unmarkErr != nil {
			return fmt.Errorf("close popup: %w; cleanup popup marker: %v", closeErr, unmarkErr)
		}
		return closeErr
	}
	return unmarkErr
}

func popupShellCommand(binaryPath, clientID string) string {
	script := strings.Join([]string{
		"cleanup() { " + popupUnsetScript(clientID) + "; }",
		"trap cleanup EXIT HUP INT TERM",
		ShellQuote(binaryPath) + " menu",
	}, "; ")
	return "sh -lc " + ShellQuote(script)
}

func popupUnsetScript(clientID string) string {
	return "tmux set-environment -gu " + popupEnvKey(clientID) + " >/dev/null 2>&1"
}

func popupCloseScript(clientID string) string {
	return "tmux display-popup -C -c " + ShellQuote(clientID) + " >/dev/null 2>&1; " + popupUnsetScript(clientID)
}

func popupEnvKey(clientID string) string {
	var key strings.Builder
	key.WriteString(menuPopupEnvPrefix)
	for _, r := range clientID {
		switch {
		case r >= 'a' && r <= 'z':
			key.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			key.WriteRune(r)
		case r >= '0' && r <= '9':
			key.WriteRune(r)
		default:
			key.WriteByte('_')
		}
	}
	return key.String()
}

func isBenignPopupCloseError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.TrimSpace(err.Error())
	return msg == "exit status 1" ||
		strings.Contains(msg, "no popup") ||
		strings.Contains(msg, "can't find client")
}
