package ui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
)

type createRequest struct{ Kind, Project, Label, Workdir, Current, Instance string }

func (m *model) submitCreate(request createRequest) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	command := strings.Join([]string{menuCurrentEnv + "=" + shellQuote(request.Current), menuClientEnv + "=" + shellQuote(os.Getenv(menuClientEnv)), menuInstanceEnv + "=" + shellQuote(request.Instance), "exec " + shellQuote(binary) + " create-worker " + shellQuote(base64.RawURLEncoding.EncodeToString(payload))}, " ")
	return m.tmux.RunBackground(command)
}

func RunCreateWorker(encoded string) error {
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return err
	}
	var request createRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		return err
	}
	manager := newSessionManager()
	if err := runCreateWorker(manager, request); err != nil {
		if displayErr := manager.DisplayMessage(err.Error()); displayErr != nil {
			diag.Warnf("display create-worker error message: %v", displayErr)
		}
	}
	return nil
}

func runCreateWorker(manager tmuxController, request createRequest) error {
	path := appStatePath()
	unlock, err := lockAppState(path)
	if err != nil {
		return fmt.Errorf("lock state %q: %w", path, err)
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release app-state lock %q after create-worker: %v", path, unlockErr)
		}
	}()

	m, err := buildModel(manager, request.Current)
	if err != nil {
		return err
	}
	m.stateLockHeld = true
	m.instanceID = request.Instance
	m.sessions, err = manager.ListSessions()
	if err != nil {
		return err
	}
	request.Project, request.Label = normalizeProjectName(request.Project), sanitizeSessionName(request.Label)
	if request.Kind == "session" {
		return m.createSessionRequest(request)
	}
	if request.Kind == "project" {
		return m.createProjectRequest(request)
	}
	return fmt.Errorf("unknown create request %q", request.Kind)
}

func (m *model) createSessionRequest(request createRequest) error {
	if request.Label == "" {
		return fmt.Errorf("session name is empty")
	}
	if request.Project == "" {
		if m.hasVolatileSessionLabel(request.Label, "") {
			return fmt.Errorf("session name already exists")
		}
		s, err := m.createVolatileSession(request.Workdir, "", request.Label)
		if err != nil {
			return err
		}
		return m.tmux.SwitchClient(s.Name)
	}
	if m.hasSessionLabel(request.Project, request.Label, "") {
		return fmt.Errorf("session name already exists in this project")
	}
	s, err := m.createPersistentSession(request.Workdir, "")
	if err != nil {
		return err
	}
	m.sessions = append(m.sessions, s)
	m.assignSessionProject(s.Name, request.Project)
	m.setSessionLabel(s.Name, request.Label)
	if err := m.saveState(); err != nil {
		if killErr := m.tmux.KillSession(s.Name); killErr != nil {
			diag.Warnf("kill unpersisted session %q: %v", s.Name, killErr)
		}
		return err
	}
	if err := m.tmux.SetSessionProject(s.Name, request.Project); err != nil {
		return err
	}
	if err := m.tmux.SetSessionLabel(s.Name, request.Label); err != nil {
		return err
	}
	return m.tmux.SwitchClient(s.Name)
}

func (m *model) createProjectRequest(request createRequest) error {
	if request.Project == "" {
		return fmt.Errorf("project name is empty")
	}
	if containsString(m.projects, request.Project) {
		return fmt.Errorf("project already exists")
	}
	current, ok := m.findSession(request.Current)
	if ok && current.Temporary && current.Instance == request.Instance {
		return m.promoteVolatileSessions(request.Project, request.Workdir, request.Current)
	}
	s, err := m.createPersistentSession(request.Workdir, "")
	if err != nil {
		return err
	}
	m.sessions = append(m.sessions, s)
	m.addProject(request.Project)
	m.setProjectConfig(projectConfig{Name: request.Project, Workdir: request.Workdir})
	m.assignSessionProject(s.Name, request.Project)
	m.setSessionLabel(s.Name, request.Label)
	if err := m.saveState(); err != nil {
		if killErr := m.tmux.KillSession(s.Name); killErr != nil {
			diag.Warnf("kill unpersisted session %q: %v", s.Name, killErr)
		}
		return err
	}
	if err := m.tmux.SetSessionProject(s.Name, request.Project); err != nil {
		return err
	}
	if err := m.tmux.SetSessionLabel(s.Name, request.Label); err != nil {
		return err
	}
	return m.tmux.SwitchClient(s.Name)
}

// promotedSession records a volatile-to-persistent rename performed during
// promotion, so it can be restored to its original volatile identity if a
// later step in the promotion fails.
type promotedSession struct{ oldName, newName string }

// restorePromotedSessions walks already-renamed sessions in reverse and
// renames each back to its original volatile name. If a restore rename
// itself fails, it performs direct best-effort cleanup of that one affected
// session instead of leaving it stranded under its new persistent-style
// name. Failures here are diagnostics only: they never replace the original
// promotion error that triggered the restore.
func (m *model) restorePromotedSessions(promoted []promotedSession) {
	for index := len(promoted) - 1; index >= 0; index-- {
		p := promoted[index]
		if err := m.tmux.RenameSession(p.newName, p.oldName); err != nil {
			diag.Warnf("restore promoted session %q to volatile name %q: %v", p.newName, p.oldName, err)
			if killErr := m.tmux.KillSession(p.newName); killErr != nil {
				diag.Warnf("clean up stranded promoted session %q after restore failure: %v", p.newName, killErr)
			}
		}
	}
}

func (m *model) promoteVolatileSessions(project, workdir, current string) error {
	m.addProject(project)
	m.setProjectConfig(projectConfig{Name: project, Workdir: workdir})
	active := ""
	promoted := make([]promotedSession, 0)
	for index := range m.sessions {
		s := m.sessions[index]
		if !s.Temporary || s.Instance != m.instanceID {
			continue
		}
		id, err := newSessionID()
		if err != nil {
			m.restorePromotedSessions(promoted)
			return err
		}
		name := persistentSessionName(id)
		if err := m.tmux.RenameSession(s.Name, name); err != nil {
			m.restorePromotedSessions(promoted)
			return err
		}
		label := m.sessionLabel(s.Name)
		delete(m.sessionProjects, s.Name)
		delete(m.sessionLabels, s.Name)
		m.sessions[index].Name, m.sessions[index].Temporary, m.sessions[index].Instance = name, false, ""
		m.assignSessionProject(name, project)
		m.setSessionLabel(name, label)
		promoted = append(promoted, promotedSession{oldName: s.Name, newName: name})
		if s.Name == current {
			active = name
		}
	}
	if active == "" {
		m.restorePromotedSessions(promoted)
		return fmt.Errorf("active volatile session is missing")
	}
	if err := m.saveState(); err != nil {
		m.restorePromotedSessions(promoted)
		return err
	}
	for _, session := range promoted {
		if err := m.tmux.SetSessionTemporary(session.newName, false, ""); err != nil {
			return err
		}
		if err := m.tmux.SetSessionProject(session.newName, project); err != nil {
			return err
		}
		if err := m.tmux.SetSessionLabel(session.newName, m.sessionLabel(session.newName)); err != nil {
			return err
		}
	}
	return m.tmux.SwitchClient(active)
}
