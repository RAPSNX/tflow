package tmux

import "strings"

func (m Manager) QuitAll() error {
	currentSession, err := m.contextValue(CurrentSessionEnv, "#{session_name}")
	if err != nil {
		return err
	}
	clientID, err := m.contextValue(CurrentClientEnv, "#{client_name}")
	if err != nil {
		return err
	}
	instanceID, err := m.resolveInstanceID(currentSession)
	if err != nil {
		return err
	}
	if err := m.cleanupVolatileSessions(instanceID, currentSession); err != nil {
		return err
	}
	script := []string{}
	if strings.TrimSpace(clientID) != "" {
		script = append(script, popupCloseScript(clientID))
		script = append(script, shellTmuxCommand("detach-client", "-t", clientID)+" >/dev/null 2>&1")
	} else {
		script = append(script, shellTmuxCommand("detach-client")+" >/dev/null 2>&1")
	}
	_, runErr := m.runner()("run-shell", strings.Join(script, "; "))
	return runErr
}
