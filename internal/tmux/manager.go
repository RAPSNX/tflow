package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"tflow/internal/store"
)

func (m Manager) ListSessions() ([]Session, error) {
	out, err := m.runner()("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{"+tempMarker+"}\t#{"+instanceMarker+"}")
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
	if _, err := m.runner()("set-option", "-t", name, "destroy-unattached", "off"); err != nil {
		return err
	}
	// Volatile sessions are removed explicitly when their tflow instance exits.
	// Keep them alive while a client switches to another session, and clear the
	// legacy hook that previously re-enabled destroy-unattached after attach.
	if _, err := m.runner()("set-hook", "-u", "-t", name, "client-attached"); err != nil {
		return err
	}
	if _, err := m.runner()("set-option", "-t", name, tempMarker, marker); err != nil {
		return err
	}
	if _, err := m.runner()("set-option", "-t", name, instanceMarker, instanceID); err != nil {
		return err
	}
	if temporary {
		if label := VolatileSessionLabel(name, instanceID); label != "" {
			_, err := m.runner()("set-option", "-t", name, sessionLabelMarker, label)
			return err
		}
	}
	return nil
}

func (Manager) AttachCommand(name string) (*exec.Cmd, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("session name is empty")
	}
	return exec.Command("tmux", "-L", socketName, "attach-session", "-t", name), nil
}

func (m Manager) KillSession(name string) error {
	_, err := m.runner()("kill-session", "-t", name)
	if err != nil && isNoSession(err) {
		return nil
	}
	return err
}

func (m Manager) RenameSession(oldName, newName string) error {
	oldName = strings.TrimSpace(oldName)
	newName = SanitizeSessionName(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("session name is empty")
	}
	if oldName == newName {
		return nil
	}
	if _, err := m.runner()("rename-session", "-t", oldName, newName); err != nil {
		return err
	}
	if label := volatileSessionLabel(newName); label != "" {
		if _, err := m.runner()("set-option", "-t", newName, sessionLabelMarker, label); err != nil {
			if _, rollbackErr := m.runner()("rename-session", "-t", newName, oldName); rollbackErr != nil {
				return fmt.Errorf("update volatile session label: %w; rollback rename: %v", err, rollbackErr)
			}
			return fmt.Errorf("update volatile session label: %w", err)
		}
	}
	return nil
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
		if !session.Temporary || session.Instance != instanceID || session.Name == skipSession {
			continue
		}
		if err := m.KillSession(session.Name); err != nil {
			return err
		}
	}
	return nil
}
