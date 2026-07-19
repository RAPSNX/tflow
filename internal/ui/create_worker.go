package ui

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
		_ = manager.DisplayMessage(err.Error())
	}
	return nil
}

func runCreateWorker(manager tmuxController, request createRequest) error {
	m, err := buildModel(manager, request.Current)
	if err != nil {
		return err
	}
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
		_ = m.tmux.KillSession(s.Name)
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
		return m.promoteVolatileSessions(request.Project, request.Current)
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
		_ = m.tmux.KillSession(s.Name)
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

func (m *model) promoteVolatileSessions(project, current string) error {
	m.addProject(project)
	m.setProjectConfig(projectConfig{Name: project, Workdir: m.cwd})
	active := ""
	for index := range m.sessions {
		s := m.sessions[index]
		if !s.Temporary || s.Instance != m.instanceID {
			continue
		}
		id, err := newSessionID()
		if err != nil {
			return err
		}
		name := persistentSessionName(id)
		if err := m.tmux.RenameSession(s.Name, name); err != nil {
			return err
		}
		label := m.sessionLabel(s.Name)
		delete(m.sessionProjects, s.Name)
		delete(m.sessionLabels, s.Name)
		m.sessions[index].Name, m.sessions[index].Temporary, m.sessions[index].Instance = name, false, ""
		m.assignSessionProject(name, project)
		m.setSessionLabel(name, label)
		if err := m.tmux.SetSessionTemporary(name, false, ""); err != nil {
			return err
		}
		if err := m.tmux.SetSessionProject(name, project); err != nil {
			return err
		}
		if err := m.tmux.SetSessionLabel(name, label); err != nil {
			return err
		}
		if s.Name == current {
			active = name
		}
	}
	if active == "" {
		return fmt.Errorf("active volatile session is missing")
	}
	if err := m.saveState(); err != nil {
		return err
	}
	return m.tmux.SwitchClient(active)
}
