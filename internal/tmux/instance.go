package tmux

import (
	"os"
	"strings"
)

func (m Manager) resolveInstanceID(currentSession string) (string, error) {
	return m.sessionInstanceID(currentSession)
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

func (m Manager) sessionInstanceID(sessionName string) (string, error) {
	sessionName = strings.TrimSpace(sessionName)
	if sessionName == "" {
		return "", nil
	}
	out, err := m.runner()("show-options", "-qv", "-t", sessionName, instanceMarker)
	if err != nil {
		if IsNoSession(err) || IsNoServer(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m Manager) CleanupDetachedClient() error {
	session, err := m.contextValue(CurrentSessionEnv, "#{session_name}")
	if err != nil {
		return err
	}
	instanceID, err := m.sessionInstanceID(session)
	if err != nil {
		return err
	}
	if instanceID == "" {
		return nil
	}
	return m.CleanupVolatileSessions(instanceID)
}

func CleanupDetachedClient() error {
	return New().CleanupDetachedClient()
}
