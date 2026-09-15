package tmux

import (
	"fmt"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
)

func (m Manager) ToggleMenu(binaryPath string) error {
	return m.openMenu(binaryPath, "")
}

// ToggleCommandMenu opens the sidebar in command mode, or closes it when the
// same client already has a command sidebar open.
func (m Manager) ToggleCommandMenu(binaryPath string) error {
	return m.openMenu(binaryPath, MenuModeCommand)
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
		if mode == MenuModeCommand {
			return m.closeCommandMenuPopup(currentClient)
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
	displayArgs := []string{
		"display-popup",
		"-c", currentClient,
		"-E",
		"-w", menuWidth,
		"-h", menuHeight,
		// "C" centers the popup horizontally under the top bar rather than
		// pinning it to the left edge; "S" for -y keeps it anchored directly
		// below the status line, which status-position top puts at the top.
		"-x", "C",
		"-y", "S",
		"-e", fmt.Sprintf("%s=%s", CurrentSessionEnv, currentSession),
		"-e", fmt.Sprintf("%s=%s", CurrentClientEnv, currentClient),
	}
	displayArgs = append(displayArgs, popupInstanceEnvArgs(instanceID)...)
	if mode != "" {
		displayArgs = append(displayArgs, "-e", fmt.Sprintf("%s=%s", MenuModeEnv, mode))
	}
	displayArgs = append(displayArgs, popupShellCommand(binaryPath, currentClient, mode))

	// markMenuPopup, the command-mode key-table switch, and display-popup
	// are three unconditional writes that always ran as three separate tmux
	// subprocess round-trips -- batched into one exec.Command via runBatch
	// instead. tmux runs a batch's groups in order and stops at the first
	// one that fails, so any error here still leaves the same ambiguity a
	// sequence of separate calls already had (an earlier group may have
	// already applied) -- the cleanup below is unconditional for exactly
	// that reason, matching the old per-step cleanup's net effect.
	groups := [][]string{markMenuPopupArgs(currentClient)}
	if mode == MenuModeCommand {
		groups = append(groups, setClientKeyTableArgs(currentClient, commandTable))
	}
	groups = append(groups, displayArgs)
	if _, err := m.runBatch(groups...); err != nil {
		if unmarkErr := m.unmarkMenuPopup(currentClient); unmarkErr != nil {
			diag.Warnf("cleanup popup marker after failed popup open: %v", unmarkErr)
		}
		if mode == MenuModeCommand {
			if resetErr := m.setClientKeyTable(currentClient, "root"); resetErr != nil {
				diag.Warnf("reset command key table after failed popup open: %v", resetErr)
			}
		}
		return err
	}
	return nil
}

func (m Manager) closeCommandMenuPopup(clientID string) error {
	closeErr := m.closeMenuPopup(clientID)
	resetErr := m.setClientKeyTable(clientID, "root")
	if closeErr != nil {
		if resetErr != nil {
			diag.Warnf("reset command key table after failed popup close: %v", resetErr)
		}
		return closeErr
	}
	return resetErr
}

func (m Manager) setClientKeyTable(clientID, table string) error {
	args := setClientKeyTableArgs(clientID, table)
	if args == nil {
		return nil
	}
	_, err := m.runner()(args...)
	return err
}

func setClientKeyTableArgs(clientID, table string) []string {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return []string{"switch-client", "-c", clientID, "-T", table}
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
	args := markMenuPopupArgs(clientID)
	if args == nil {
		return nil
	}
	_, err := m.runner()(args...)
	return err
}

func markMenuPopupArgs(clientID string) []string {
	if strings.TrimSpace(clientID) == "" {
		return nil
	}
	return []string{"set-environment", "-gh", popupEnvKey(clientID), "1"}
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

func popupShellCommand(binaryPath, clientID, mode string) string {
	cleanup := popupUnsetScript(clientID)
	if mode == MenuModeCommand {
		cleanup += "; " + shellTmuxCommand("switch-client", "-c", clientID, "-T", "root") + " >/dev/null 2>&1"
	}
	script := strings.Join([]string{
		"cleanup() { " + cleanup + "; }",
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
