package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/diag"
)

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	name := strings.TrimSpace(m.selectedSession)
	if name == "" {
		m.status = "No session selected."
		return m, nil
	}
	if _, ok := m.findSession(name); !ok {
		return m.materializePersistentSession(name)
	}
	return m, func() tea.Msg {
		return menuActionMsg{switchSession: name}
	}
}

func (m model) materializePersistentSession(name string) (tea.Model, tea.Cmd) {
	project := normalizeProjectName(m.sessionProjects[name])
	if project == "" {
		m.status = "No session selected."
		return m, nil
	}

	unlock, err := lockAppState(m.statePath)
	if err != nil {
		m.err, m.status = err, err.Error()
		return m, nil
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release state lock %q after lazy materialization: %v", m.statePath, unlockErr)
		}
	}()

	state, err := loadAppState(m.statePath)
	if err != nil {
		m.err, m.status = err, err.Error()
		return m, nil
	}
	label, workdir, found := "", "", false
	for _, storedProject := range state.Projects {
		for _, storedSession := range storedProject.Sessions {
			if storedSession.ID != name {
				continue
			}
			if storedProject.Name != project {
				m.status = "Session is no longer in the selected project."
				return m, nil
			}
			label, workdir, found = storedSession.Label, storedProject.Workdir, true
			break
		}
		if found {
			break
		}
	}
	if !found {
		m.status = "Session no longer exists."
		return m, nil
	}

	created, err := m.tmux.CreateSession(name, workdir, "")
	if err != nil {
		m.err = fmt.Errorf("create saved session %q: %w", name, err)
		m.status = m.err.Error()
		return m, nil
	}
	cleanup := func(operation string, err error) (tea.Model, tea.Cmd) {
		if killErr := m.tmux.KillSession(name); killErr != nil {
			diag.Warnf("kill lazily materialized session %q after %s failure: %v", name, operation, killErr)
		}
		m.err = fmt.Errorf("%s saved session %q: %w", operation, name, err)
		m.status = m.err.Error()
		return m, nil
	}
	if err := m.tmux.SetSessionProject(name, project); err != nil {
		return cleanup("set project marker for", err)
	}
	if err := m.tmux.SetSessionLabel(name, label); err != nil {
		return cleanup("set label marker for", err)
	}
	if err := m.tmux.SetSessionTemporary(name, false, ""); err != nil {
		return cleanup("clear volatile markers for", err)
	}
	created.Name = name
	created.Label = label
	created.Temporary = false
	created.Instance = ""
	m.setSessionLabel(name, label)
	m.sessions = append(m.sessions, created)
	return m.switchSelectedSession()
}

func (m *model) beginProjectSwitch() (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		m.status = "No projects available."
		return m, nil
	}
	m.mode = inputSwitchProject
	m.switchProjectTarget = ""
	m.projectSwitchIndex = 0
	m.input.Prompt = ""
	m.input.SetValue("")
	m.input.Focus()
	m.status = ""
	return m, nil
}

func (m *model) beginProjectCreate() (tea.Model, tea.Cmd) {
	m.mode = inputCreateProject
	m.input.Prompt = ""
	m.input.SetValue("")
	m.input.Focus()
	m.status = ""
	return m, nil
}

func (m *model) commitProjectCreate() (tea.Model, tea.Cmd) {
	name := sanitizeProjectName(m.input.Value())
	if name == "" {
		m.status = "Project name is empty."
		return m, nil
	}
	if containsString(m.projects, name) {
		m.status = "Project already exists."
		return m, nil
	}

	workdir, err := m.tmux.CurrentPaneDir()
	if err != nil {
		m.err, m.status = err, err.Error()
		return m, nil
	}
	if err := m.submitCreate(createRequest{Kind: "project", Project: name, Label: randomAnimalName(), Workdir: workdir, Current: m.currentSession, Instance: m.instanceID}); err != nil {
		m.err, m.status = err, err.Error()
		return m, nil
	}
	m.mode = inputNone
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	return m, m.closeMenuCmd()
}

func (m *model) commitProjectSwitch() (tea.Model, tea.Cmd) {
	project, ok := m.selectedMatchingProject()
	if !ok {
		m.status = "No matching project selected."
		return m, nil
	}
	if current, ok := m.currentSessionInfo(); ok && current.Temporary {
		m.mode = inputConfirmProjectSwitch
		m.switchProjectTarget = project
		m.input.Blur()
		m.input.Prompt = ""
		m.input.SetValue("")
		m.status = ""
		return m, nil
	}
	return m.switchToProject(project)
}

func (m *model) confirmProjectSwitch() (tea.Model, tea.Cmd) {
	project := m.switchProjectTarget
	m.switchProjectTarget = ""
	return m.switchToProject(project)
}

func (m *model) switchToProject(project string) (tea.Model, tea.Cmd) {
	project = normalizeProjectName(project)
	m.mode = inputNone
	m.switchProjectTarget = ""
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	if project == "" {
		m.status = "No project selected."
		return m, nil
	}
	sessions := m.projectSessions(project)
	if len(sessions) == 0 {
		m.status = fmt.Sprintf("Project %s has no sessions.", project)
		return m, nil
	}
	m.selectedProject = project
	m.selectedSession = sessions[0].Name
	m.status = ""
	return m.switchSelectedSession()
}

func (m model) killSession(name string) (tea.Model, tea.Cmd) {
	name = strings.TrimSpace(name)
	if name == "" {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return sessionKilledMsg{name: name, project: normalizeProjectName(m.sessionProjects[name]), err: m.tmux.KillSession(name)}
	}
}

func (m *model) beginDelete() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: s.Name}
	m.status = ""
	return m, nil
}

func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	target := m.deleteTarget
	m.mode = inputNone
	m.deleteTarget = deleteTarget{}
	if target.session != "" {
		if project := normalizeProjectName(m.sessionProjects[target.session]); project != "" && len(m.projectSessions(project)) == 1 {
			return m.deleteProject(project)
		}
		return m.killSession(target.session)
	}
	return m.deleteProject(target.project)
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{}
	}
}

func (m *model) beginQuit() (tea.Model, tea.Cmd) {
	m.mode = inputConfirmQuit
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	m.status = ""
	return m, nil
}

func (m model) confirmQuit() (tea.Model, tea.Cmd) {
	m.mode = inputNone
	return m, func() tea.Msg {
		return menuActionMsg{quit: true}
	}
}

func (m *model) beginRename() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	m.mode = inputRename
	m.renameTarget = renameTarget{session: s.Name}
	m.input.SetValue(m.sessionLabel(s.Name))
	m.input.CursorEnd()
	m.input.Prompt = ""
	m.input.Focus()
	m.status = ""
	return m, nil
}

func (m *model) commitRename() (tea.Model, tea.Cmd) {
	target := m.renameTarget
	switch {
	case target.project != "":
		name := sanitizeProjectName(m.input.Value())
		if name == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if name == target.project {
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if containsString(m.projects, name) {
			m.status = "Project already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return projectRenamedMsg{oldName: target.project, newName: name}
		}
	case target.session != "":
		label := sanitizeSessionName(m.input.Value())
		if label == "" {
			m.status = "Session name is empty."
			return m, nil
		}
		if label == m.sessionLabel(target.session) {
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		selected, found := m.findSession(target.session)
		if !found {
			m.status = "Session no longer exists."
			return m, nil
		}
		project := ""
		if !selected.Temporary {
			project = normalizeProjectName(m.sessionProjects[target.session])
		}
		if project != "" && m.hasSessionLabel(project, label, target.session) {
			m.status = "Session name already exists in this project."
			return m, nil
		}
		if project == "" && m.hasVolatileSessionLabel(label, target.session) {
			m.status = "Session name already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return sessionRenamedMsg{name: target.session, label: label, volatile: selected.Temporary, err: m.tmux.SetSessionLabel(target.session, label)}
		}
	default:
		m.status = "Select a session or project to rename."
		return m, nil
	}
}

func (m *model) shiftProjectSwitch(delta int) {
	matches := m.matchingProjects(m.input.Value())
	if len(matches) == 0 {
		m.projectSwitchIndex = 0
		return
	}
	m.projectSwitchIndex = (m.projectSwitchIndex + delta + len(matches)) % len(matches)
}

func (m *model) selectedMatchingProject() (string, bool) {
	matches := m.matchingProjects(m.input.Value())
	if len(matches) == 0 {
		return "", false
	}
	if m.projectSwitchIndex < 0 || m.projectSwitchIndex >= len(matches) {
		m.projectSwitchIndex = 0
	}
	return matches[m.projectSwitchIndex], true
}

func (m model) nextProjectAfter(project string, deletingProject bool) string {
	if len(m.projects) == 0 {
		return ""
	}
	index := indexOfString(m.projects, project)
	if index < 0 {
		return m.projects[0]
	}
	if deletingProject && len(m.projects) == 1 {
		return ""
	}
	return m.projects[(index+1)%len(m.projects)]
}

func (m model) createVolatileFallback() (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.instanceID) == "" {
		m.err = fmt.Errorf("tflow instance id is empty")
		m.status = m.err.Error()
		return m, nil
	}
	label := nextTempSessionNameForInstance(m.sessions, m.instanceID)
	return m, func() tea.Msg {
		s, err := m.createVolatileSession(m.cwd, "", label)
		if err != nil {
			return sessionCreatedMsg{err: fmt.Errorf("create volatile fallback: %w", err)}
		}
		return sessionCreatedMsg{session: s, volatile: true, label: label}
	}
}
