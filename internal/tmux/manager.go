package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (m Manager) ListSessions() ([]Session, error) {
	out, err := m.runner()("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{"+tempMarker+"}")
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

	if _, err := m.runner()("has-session", "-t", name); err == nil {
		return Session{Name: name}, nil
	} else if !isNoSession(err) && !IsNoServer(err) {
		return Session{}, err
	}

	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	if command != "" {
		args = append(args, userShell(), "-lc", command)
	}
	if _, err := m.runner()(args...); err != nil {
		return Session{}, err
	}
	if _, err := m.runner()("rename-window", "-t", name+":1", name); err != nil && !strings.Contains(err.Error(), "can't find window") {
		return Session{}, err
	}
	return Session{Name: name, Windows: 1}, nil
}

func (m Manager) SetSessionTemporary(name string, temporary bool) error {
	marker := "0"
	if temporary {
		marker = "1"
	}
	if _, err := m.runner()("set-option", "-t", name, "destroy-unattached", "off"); err != nil {
		return err
	}
	if temporary {
		hook := "set-option -t " + ShellQuote(name) + " destroy-unattached on; set-hook -u -t " + ShellQuote(name) + " client-attached"
		if _, err := m.runner()("set-hook", "-t", name, "client-attached", hook); err != nil {
			return err
		}
	} else {
		if _, err := m.runner()("set-hook", "-u", "-t", name, "client-attached"); err != nil {
			return err
		}
	}
	_, err := m.runner()("set-option", "-t", name, tempMarker, marker)
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
	if err != nil && isNoSession(err) {
		return nil
	}
	return err
}

func (m Manager) RenameSession(oldName, newName string) error {
	oldName = SanitizeSessionName(oldName)
	newName = SanitizeSessionName(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("session name is empty")
	}
	if oldName == newName {
		return nil
	}
	_, err := m.runner()("rename-session", "-t", oldName, newName)
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

func (m Manager) SyncSessionProjects(sessionProjects map[string]string) error {
	for name, project := range sessionProjects {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		project = normalizeProjectName(project)
		if _, err := m.runner()("set-option", "-t", name, projectMarker, project); err != nil {
			if isNoSession(err) || IsNoServer(err) {
				continue
			}
			return err
		}
	}
	return nil
}
