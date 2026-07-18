package tmux

import (
	"os"
	"strings"
)

func (m Manager) resolveInstanceID(currentSession, currentClient string) (string, error) {
	instanceID, err := m.sessionInstanceID(currentSession)
	if err != nil {
		return "", err
	}
	if instanceID != "" {
		return instanceID, nil
	}
	instanceID, err = m.clientInstanceID(currentClient)
	if err != nil {
		return "", err
	}
	if instanceID != "" {
		return instanceID, nil
	}
	return "", nil
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
		if isNoSession(err) || IsNoServer(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m Manager) clientInstanceID(clientID string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", nil
	}
	out, err := m.runner()("show-environment", "-gh")
	if err != nil {
		return "", err
	}
	key := instanceEnvKey(clientID) + "="
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			return strings.TrimPrefix(line, key), nil
		}
	}
	return "", nil
}

func (m Manager) rememberClientInstance(clientID, instanceID string) error {
	clientID = strings.TrimSpace(clientID)
	instanceID = strings.TrimSpace(instanceID)
	if clientID == "" || instanceID == "" {
		return nil
	}
	_, err := m.runner()("set-environment", "-gh", instanceEnvKey(clientID), instanceID)
	return err
}
