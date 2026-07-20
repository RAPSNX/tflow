package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"tflow/internal/store"
)

func (m Manager) ListSessions() ([]Session, error) {
	out, err := m.runner()("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{"+tempMarker+"}\t#{"+instanceMarker+"}\t#{"+sessionLabelMarker+"}")
	if err != nil {
		if IsNoServer(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	sessions := make([]Session, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		session := Session{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &session.Windows)
		}
		if len(parts) > 2 {
			session.Attached = strings.TrimSpace(parts[2]) == "1"
		}
		if len(parts) > 3 {
			session.Temporary = strings.TrimSpace(parts[3]) == "1"
		}
		if len(parts) > 4 {
			session.Instance = strings.TrimSpace(parts[4])
		}
		if len(parts) > 5 {
			session.Label = strings.TrimSpace(parts[5])
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (m Manager) CreateSession(name, cwd, command string) (Session, error) {
	name = SanitizeSessionName(name)
	if name == "" {
		return Session{}, fmt.Errorf("session name is empty")
	}
	cwd = NormalizeCWD(cwd)
	command = strings.TrimSpace(command)

	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	if command != "" {
		args = append(args, userShell(), "-lc", command)
	}
	if _, err := m.runner()(args...); err != nil {
		return Session{}, err
	}
	// This rename-window fallback string was captured against tmux 3.7b.
	if _, err := m.runner()("rename-window", "-t", name+":1", name); err != nil && !strings.Contains(err.Error(), "can't find window") {
		return Session{}, err
	}
	return Session{Name: name, Windows: 1}, nil
}

func (m Manager) SetSessionTemporary(name string, temporary bool, instanceID string) error {
	instanceID = strings.TrimSpace(instanceID)
	if temporary && instanceID == "" {
		return fmt.Errorf("instance id is empty")
	}
	marker := "0"
	if temporary {
		marker = "1"
	} else {
		instanceID = ""
	}
	if _, err := m.runner()("set-option", "-t", name, tempMarker, marker); err != nil {
		return err
	}
	if _, err := m.runner()("set-option", "-t", name, instanceMarker, instanceID); err != nil {
		return err
	}
	if _, err := m.runner()("set-option", "-t", name, "destroy-unattached", "off"); err != nil {
		return err
	}
	// Volatile sessions are removed explicitly when their tflow instance exits.
	// Keep them alive while a client switches to another session, and clear the
	// legacy hook that previously re-enabled destroy-unattached after attach.
	if _, err := m.runner()("set-hook", "-u", "-t", name, "client-attached"); err != nil {
		return err
	}
	return nil
}

func (m Manager) SetSessionLabel(name, label string) error {
	name = strings.TrimSpace(name)
	label = SanitizeSessionName(label)
	if name == "" || label == "" {
		return fmt.Errorf("session label is empty")
	}
	_, err := m.runner()("set-option", "-t", name, sessionLabelMarker, label)
	return err
}

func (Manager) AttachCommand(name string) (*exec.Cmd, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("session name is empty")
	}
	return exec.Command("tmux", "-L", socketName, "attach-session", "-t", name), nil
}

func (m Manager) KillSession(name string) error {
	_, err := m.runner()("kill-session", "-t", name)
	if err != nil && (isNoSession(err) || IsNoServer(err)) {
		return nil
	}
	return err
}

func (m Manager) SwitchClient(name string) error {
	args := []string{"switch-client"}
	if clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv)); clientID != "" {
		args = append(args, "-c", clientID)
	}
	args = append(args, "-t", name)
	_, err := m.runner()(args...)
	return err
}

func (m Manager) SyncSessionProjects(sessionProjects, sessionLabels map[string]string) error {
	for name, project := range sessionProjects {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		project = store.NormalizeProjectName(project)
		if _, err := m.runner()("set-option", "-t", name, projectMarker, project); err != nil {
			if isNoSession(err) || IsNoServer(err) {
				continue
			}
			return err
		}
		label := strings.TrimSpace(sessionLabels[name])
		if label == "" {
			label = name
		}
		if _, err := m.runner()("set-option", "-t", name, sessionLabelMarker, label); err != nil {
			if isNoSession(err) || IsNoServer(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (m Manager) CleanupVolatileSessions(instanceID string) error {
	return m.cleanupVolatileSessions(instanceID, "")
}

func (m Manager) cleanupVolatileSessions(instanceID, skipSession string) error {
	instanceID = strings.TrimSpace(instanceID)
	if instanceID == "" {
		return nil
	}

	sessions, err := m.ListSessions()
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if !session.Temporary || session.Instance != instanceID || session.Name == skipSession || strings.HasPrefix(session.Name, persistentSessionPrefix) {
			continue
		}
		if err := m.KillSession(session.Name); err != nil {
			return err
		}
	}
	return nil
}

func (m Manager) RenameSession(oldName, newName string) error {
	_, err := m.runner()("rename-session", "-t", oldName, newName)
	return err
}
func (m Manager) SetSessionProject(name, project string) error {
	_, err := m.runner()("set-option", "-t", name, projectMarker, store.NormalizeProjectName(project))
	return err
}
func (m Manager) RunBackground(command string) error {
	_, err := m.runner()("run-shell", "-b", command)
	return err
}
func (m Manager) DisplayMessage(message string) error {
	args := []string{"display-message"}
	if clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv)); clientID != "" {
		args = append(args, "-c", clientID)
	}
	args = append(args, message)
	_, err := m.runner()(args...)
	return err
}

func (m Manager) CurrentPaneDir() (string, error) {
	args := []string{"display-message", "-p"}
	clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv))
	hasClient := clientID != ""
	if hasClient {
		args = append(args, "-c", clientID)
	}
	args = append(args, "#{pane_current_path}")
	out, err := m.runner()(args...)
	if err == nil {
		if dir := strings.TrimSpace(out); dir != "" {
			return dir, nil
		}
	} else if !hasClient {
		return "", err
	}
	// A popup can retain a stale client identifier after the tmux client that opened it has been recreated.
	// Retry without the stale target when tmux fails to resolve that client or returns an empty path.
	if hasClient {
		out, err = m.runner()("display-message", "-p", "#{pane_current_path}")
		if err != nil {
			return "", err
		}
		if dir := strings.TrimSpace(out); dir != "" {
			return dir, nil
		}
	}
	return "", fmt.Errorf("active pane directory is empty")
}
