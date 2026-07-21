package tmux

import (
	"os"
	"strings"
)

// resolveInstanceID resolves the tflow instance owning the current popup
// invocation. The current session's own marker is authoritative when
// present (volatile sessions always carry one). Persistent sessions never
// carry an instance marker, since instance ownership is a client property,
// not a session property -- so when the session has none, fall back to the
// client-scoped instance slot rememberClientInstance keeps up to date,
// letting a client that switched into a persistent session still resolve
// its own owning instance instead of losing it. This is a deliberately
// keyed, explicitly-queried global-environment entry (the same pattern
// already used for the popup-visibility marker), not the ambient/inherited
// process-environment fallback the architecture forbids for instance IDs.
func (m Manager) resolveInstanceID(currentSession, clientID string) (string, error) {
	id, err := m.sessionInstanceID(currentSession)
	if err != nil {
		return "", err
	}
	if id != "" {
		return id, nil
	}
	return m.clientInstanceID(clientID)
}

// rememberClientInstance keeps the client-scoped instance slot in sync
// whenever a non-empty instance ID is known for clientID, regardless of
// whether it was just resolved from the current session's own marker or
// recovered from this same slot moments earlier. It is a no-op once either
// value is empty.
func (m Manager) rememberClientInstance(clientID, instanceID string) error {
	clientID = strings.TrimSpace(clientID)
	instanceID = strings.TrimSpace(instanceID)
	if clientID == "" || instanceID == "" {
		return nil
	}
	_, err := m.runner()("set-environment", "-gh", clientInstanceEnvKey(clientID), instanceID)
	return err
}

// clientInstanceID reads the client-scoped instance slot rememberClientInstance
// writes, mirroring menuPopupVisible's list-and-filter approach against
// `show-environment -gh` rather than querying a single variable name
// directly, since that approach is already proven for this exact style of
// client-scoped global-environment entry.
func (m Manager) clientInstanceID(clientID string) (string, error) {
	clientID = strings.TrimSpace(clientID)
	if clientID == "" {
		return "", nil
	}
	out, err := m.runner()("show-environment", "-gh")
	if err != nil {
		return "", err
	}
	key := clientInstanceEnvKey(clientID) + "="
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, key) {
			return strings.TrimSpace(strings.TrimPrefix(line, key)), nil
		}
	}
	return "", nil
}

func clientInstanceEnvKey(clientID string) string {
	return clientScopedEnvKey(menuInstanceEnvPrefix, clientID)
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
