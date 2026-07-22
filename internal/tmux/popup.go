package tmux

import (
	"fmt"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
)

func (m Manager) ToggleMenu(binaryPath string) error {
	return m.openMenu(binaryPath, "")
}

func (m Manager) OpenQuit(binaryPath string) error {
	return m.openMenu(binaryPath, MenuModeQuit)
}

func (m Manager) openMenu(binaryPath, mode string) error {
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
		if mode == "" {
			return m.closeMenuPopup(currentClient)
		}
		if err := m.closeMenuPopup(currentClient); err != nil {
			return err
		}
	}

	instanceID, err := m.resolveInstanceID(currentSession, currentClient)
	if err != nil {
		return err
	}
	// Remember the instance before marking the popup visible: a failure here
	// must return before the popup marker is set, so there is nothing to
	// unmark on this path -- markMenuPopup's own failure needs no cleanup
	// either, and the later display-popup failure path already unmarks on
	// its own.
	if err := m.rememberClientInstance(currentClient, instanceID); err != nil {
		return err
	}
	if err := m.markMenuPopup(currentClient); err != nil {
		return err
	}

	args := []string{
		"display-popup",
		"-c", currentClient,
		"-E",
		"-w", menuWidth,
		"-h", menuHeight,
		"-x", "0",
		"-y", "C",
		"-e", fmt.Sprintf("%s=%s", CurrentSessionEnv, currentSession),
		"-e", fmt.Sprintf("%s=%s", CurrentClientEnv, currentClient),
	}
	args = append(args, popupInstanceEnvArgs(instanceID)...)
	if mode != "" {
		args = append(args, "-e", fmt.Sprintf("%s=%s", MenuModeEnv, mode))
	}
	args = append(args, popupShellCommand(binaryPath, currentClient))
	_, err = m.runner()(args...)
	if err != nil {
		if unmarkErr := m.unmarkMenuPopup(currentClient); unmarkErr != nil {
			diag.Warnf("cleanup popup marker after failed display-popup: %v", unmarkErr)
		}
		return err
	}
	return nil
}

func (m Manager) CloseMenu() error {
	clientID, err := m.contextValue(CurrentClientEnv, "#{client_name}")
	if err != nil {
		if isBenignPopupCloseError(err) {
			return nil
		}
		return err
	}
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return m.closeMenuPopup(clientID)
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
	if isBenignEnvCleanupError(err) {
		return nil
	}
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
			diag.Warnf("clear popup marker for client %q after close failure: %v", clientID, unmarkErr)
		}
		// Preserve the close error; popup-marker cleanup is deliberately best effort.
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

// popupInstanceEnvArgs always emits an explicit -e for the instance ID, even
// when it resolved to empty, rather than omitting the flag. Omitting it would
// let the popup process fall through to whatever TFLOW_INSTANCE_ID happens to
// already be sitting in the tmux server's own inherited environment, instead
// of the value this specific popup actually resolved.
func popupInstanceEnvArgs(instanceID string) []string {
	instanceID = strings.TrimSpace(instanceID)
	return []string{"-e", fmt.Sprintf("%s=%s", CurrentInstanceEnv, instanceID)}
}

func popupUnsetScript(clientID string) string {
	return shellTmuxCommand("set-environment", "-gu", popupEnvKey(clientID)) + " >/dev/null 2>&1"
}

func popupCloseScript(clientID string) string {
	return shellTmuxCommand("display-popup", "-C", "-c", clientID) + " >/dev/null 2>&1; " + popupUnsetScript(clientID)
}

func popupEnvKey(clientID string) string {
	return clientScopedEnvKey(menuPopupEnvPrefix, clientID)
}

func clientScopedEnvKey(prefix, clientID string) string {
	var key strings.Builder
	key.WriteString(prefix)
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

func shellTmuxCommand(args ...string) string {
	socket := socketArgs()
	quoted := make([]string, 0, len(args)+3)
	quoted = append(quoted, "tmux", socket[0], ShellQuote(socket[1]))
	for _, arg := range args {
		quoted = append(quoted, ShellQuote(arg))
	}
	return strings.Join(quoted, " ")
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

func isBenignEnvCleanupError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.TrimSpace(err.Error())
	return msg == "exit status 1" ||
		strings.Contains(msg, "unknown variable") ||
		strings.Contains(msg, "not found")
}
