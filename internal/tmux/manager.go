package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
	"github.com/rapsnx/tflow/internal/store"
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
			var attachedCount int
			fmt.Sscanf(parts[2], "%d", &attachedCount)
			session.Attached = attachedCount > 0
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
		// Preserve the setup error; deleting the newly created session is best effort.
		if killErr := m.KillSession(name); killErr != nil {
			diag.Warnf("kill orphaned session %q after post-creation setup failure: %v", name, killErr)
		}
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
	label = NormalizeSessionLabel(label)
	if name == "" || label == "" {
		return fmt.Errorf("session label is empty")
	}
	_, err := m.runner()("set-option", "-t", name, sessionLabelMarker, label)
	return err
}

func (Manager) AttachCommand(ctx context.Context, name string) (*exec.Cmd, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("session name is empty")
	}
	args := append(socketArgs(), "attach-session", "-t", name)
	return exec.CommandContext(ctx, "tmux", args...), nil
}

func (m Manager) KillSession(name string) error {
	_, err := m.runner()("kill-session", "-t", name)
	if err != nil && (IsNoSession(err) || IsNoServer(err)) {
		return nil
	}
	return err
}

func (m Manager) SwitchClient(name string) error {
	clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv))
	if clientID == "" {
		_, err := m.runner()("switch-client", "-t", name)
		return err
	}
	if _, err := m.runner()("switch-client", "-c", clientID, "-t", name); err != nil {
		if !isMissingClientError(err) {
			return err
		}
		// TFLOW_CURRENT_CLIENT can outlive the tmux client it named, e.g. a create-worker inherits it from a
		// popup whose client was already recreated. Retry only once a missing-client error is positively
		// identified, and only against a replacement client resolved from the current session -- never an
		// arbitrary client tmux itself might pick without -c.
		replacement, resolveErr := m.resolveReplacementClient()
		if resolveErr != nil {
			return err
		}
		_, err = m.runner()("switch-client", "-c", replacement, "-t", name)
		return err
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
	clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv))
	if clientID == "" {
		_, err := m.runner()("display-message", message)
		return err
	}
	if _, err := m.runner()("display-message", "-c", clientID, message); err != nil {
		if !isMissingClientError(err) {
			return err
		}
		// Same stale-client fallback as SwitchClient: don't let a gone client swallow the error report, but
		// only retry against a same-session replacement client, never an arbitrary one.
		replacement, resolveErr := m.resolveReplacementClient()
		if resolveErr != nil {
			return err
		}
		_, err = m.runner()("display-message", "-c", replacement, message)
		return err
	}
	return nil
}

func (m Manager) CurrentPaneDir() (string, error) {
	clientID := strings.TrimSpace(os.Getenv(CurrentClientEnv))
	if clientID == "" {
		out, err := m.runner()("display-message", "-p", "#{pane_current_path}")
		if err != nil {
			return "", err
		}
		return nonEmptyPaneDir(out)
	}
	out, err := m.runner()("display-message", "-p", "-c", clientID, "#{pane_current_path}")
	if err != nil {
		if !isMissingClientError(err) {
			return "", err
		}
		// A popup can retain a stale client identifier after the tmux client that opened it has been recreated.
		// Retry only once a missing-client error is positively identified, and only against a replacement
		// client resolved from the current session. A resolved client whose active pane path is legitimately
		// empty must not fall back to a different client's active pane.
		replacement, resolveErr := m.resolveReplacementClient()
		if resolveErr != nil {
			return "", err
		}
		out, err = m.runner()("display-message", "-p", "-c", replacement, "#{pane_current_path}")
		if err != nil {
			return "", err
		}
	}
	return nonEmptyPaneDir(out)
}

func nonEmptyPaneDir(out string) (string, error) {
	if dir := strings.TrimSpace(out); dir != "" {
		return dir, nil
	}
	return "", fmt.Errorf("active pane directory is empty")
}

// isMissingClientError reports whether err is tmux positively reporting that
// a -c target-client could not be resolved, as opposed to some other,
// unrelated failure that happens to occur while a stale client is in use.
// Only this specific error should trigger a same-session client retry.
func isMissingClientError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "can't find client")
}

// resolveReplacementClient finds the client currently attached to the
// session recorded in CurrentSessionEnv. That session -- not the possibly
// gone client name a caller already has -- is the reliable anchor for "the
// same tflow instance": a client that was recreated (for example after a
// popup respawned) is still attached to the same session, so resolving by
// session finds its live successor instead of falling back to whatever
// client tmux itself would pick without -c, which could belong to a
// different instance entirely on a multi-client server.
func (m Manager) resolveReplacementClient() (string, error) {
	sessionName := strings.TrimSpace(os.Getenv(CurrentSessionEnv))
	if sessionName == "" {
		return "", fmt.Errorf("no current session to resolve a replacement client")
	}
	out, err := m.runner()("list-clients", "-F", "#{client_name}\t#{client_session}")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.TrimSpace(parts[1]) == sessionName {
			if client := strings.TrimSpace(parts[0]); client != "" {
				return client, nil
			}
		}
	}
	return "", fmt.Errorf("no client attached to session %q", sessionName)
}
