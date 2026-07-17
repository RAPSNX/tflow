package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.tmux.SwitchClient(s.Name)
		if err == nil {
			err = m.tmux.CloseMenu()
		}
		return menuActionMsg{err: err}
	}
}

func (m *model) beginProjectSwitch() (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		m.status = "No projects available."
		return m, nil
	}
	m.mode = inputSwitchProject
	m.switchProjectTarget = ""
	m.input.Prompt = "project: "
	m.input.SetValue("")
	m.input.Focus()
	m.status = "Type a project prefix to switch."
	return m, nil
}

func (m *model) commitProjectSwitch() (tea.Model, tea.Cmd) {
	project, ok := m.uniqueMatchingProject(m.input.Value())
	if !ok {
		m.status = "No unique project match."
		return m, nil
	}
	if current, ok := m.currentSessionInfo(); ok && current.Temporary {
		m.mode = inputConfirmProjectSwitch
		m.switchProjectTarget = project
		m.input.Blur()
		m.input.Prompt = ""
		m.input.SetValue("")
		m.status = fmt.Sprintf("Confirm switch from volatile session to project %s.", project)
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

func (m model) killSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return sessionKilledMsg{name: s.Name, err: m.tmux.KillSession(s.Name)}
	}
}

func (m *model) beginDelete() (tea.Model, tea.Cmd) {
	if s, ok := m.selectedSessionInfo(); ok {
		m.mode = inputConfirmDelete
		m.deleteTarget = deleteTarget{session: s.Name}
		m.status = fmt.Sprintf("Confirm delete for session %s.", s.Name)
		return m, nil
	}

	project := m.contextProject()
	if project == "" {
		m.status = "Select a session or project to delete."
		return m, nil
	}
	if m.projectConfig(project).Protect {
		m.status = fmt.Sprintf("Project %s is protected.", project)
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{project: project}
	m.status = fmt.Sprintf("Confirm delete for project %s.", project)
	return m, nil
}

func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	target := m.deleteTarget
	m.mode = inputNone
	m.deleteTarget = deleteTarget{}
	if target.session != "" {
		return m.killSelectedSession()
	}
	return m.deleteSelectedProject()
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.CloseMenu()}
	}
}

func (m model) quitAllCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.QuitAll()}
	}
}

func (m *model) beginRename() (tea.Model, tea.Cmd) {
	if s, ok := m.selectedSessionInfo(); ok {
		m.mode = inputRename
		m.renameTarget = renameTarget{session: s.Name}
		m.input.SetValue(s.Name)
		m.input.CursorEnd()
		m.input.Prompt = "session: "
		m.input.Focus()
		m.status = "Rename the selected session."
		return m, nil
	}

	project := m.contextProject()
	if project == "" {
		m.status = "Select a session or project to rename."
		return m, nil
	}
	m.mode = inputRename
	m.renameTarget = renameTarget{project: project}
	m.input.SetValue(project)
	m.input.CursorEnd()
	m.input.Prompt = "project: "
	m.input.Focus()
	m.status = "Rename the current project."
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
		name := sanitizeSessionName(m.input.Value())
		if name == "" {
			m.status = "Session name is empty."
			return m, nil
		}
		if name == target.session {
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if _, ok := m.findSession(name); ok {
			m.status = "Session already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return sessionRenamedMsg{oldName: target.session, newName: name, err: m.tmux.RenameSession(target.session, name)}
		}
	default:
		m.status = "Select a session or project to rename."
		return m, nil
	}
}

func (m model) deleteSelectedProject() (tea.Model, tea.Cmd) {
	project := normalizeProjectName(m.contextProject())
	if project == "" {
		m.status = "Select a project to delete."
		return m, nil
	}
	sessions := m.projectSessions(project)
	return m, func() tea.Msg {
		for _, s := range sessions {
			if err := m.tmux.KillSession(s.Name); err != nil {
				return projectDeletedMsg{project: project, err: err}
			}
		}
		return projectDeletedMsg{project: project}
	}
}

func (m model) applyProjectDeletion(project string) (tea.Model, tea.Cmd) {
	project = normalizeProjectName(project)
	if project == "" {
		m.err = fmt.Errorf("project name is empty")
		m.status = "Project name is empty."
		return m, nil
	}

	deletedSessions := m.projectSessions(project)
	for _, s := range deletedSessions {
		delete(m.sessionProjects, s.Name)
		delete(m.sessionTypes, s.Name)
		if m.selectedSession == s.Name {
			m.selectedSession = ""
		}
		if m.currentSession == s.Name {
			m.currentSession = ""
		}
	}
	m.sessions = filterSessions(m.sessions, func(s session) bool {
		for _, deleted := range deletedSessions {
			if deleted.Name == s.Name {
				return false
			}
		}
		return true
	})
	m.projects = removeProject(m.projects, project)
	delete(m.projectConfigs, project)
	if m.selectedProject == project {
		m.selectedProject = ""
	}
	if err := removeProjectConfigFile(m.statePath, project); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.syncSelection()
	if err := m.syncTmuxSessionProjects(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
	return m, nil
}
