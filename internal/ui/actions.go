package ui

import (
	"fmt"
	"strings"

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
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
	}
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
	row, ok := m.selectedRow()
	if !ok {
		m.status = "Select a project or session to delete."
		return m, nil
	}
	if row.kind == rowProject && m.projectConfig(row.project).Protect {
		project := row.project
		m.status = fmt.Sprintf("Project %s is protected.", project)
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteRow = row
	switch row.kind {
	case rowProject:
		m.status = fmt.Sprintf("Confirm delete for project %s.", row.project)
	case rowSession:
		m.status = fmt.Sprintf("Confirm delete for session %s.", row.session)
	}
	return m, nil
}

func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	row := m.deleteRow
	m.mode = inputNone
	m.deleteRow = treeRow{}
	if row.kind == rowSession {
		return m.killSelectedSession()
	}
	return m.deleteSelectedProject()
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.ClosePane(m.paneID)}
	}
}

func (m model) quitAllCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.QuitAll(m.paneID)}
	}
}

func (m *model) beginRename() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		m.status = "Select a project or session to rename."
		return m, nil
	}
	m.mode = inputRename
	m.renameRow = row
	m.input.SetValue(m.renameValue(row))
	m.input.CursorEnd()
	if row.kind == rowProject {
		m.input.Prompt = "project: "
		m.status = "Rename the selected project."
	} else {
		m.input.Prompt = "session: "
		m.status = "Rename the selected session."
	}
	m.input.Focus()
	return m, nil
}

func (m *model) commitRename() (tea.Model, tea.Cmd) {
	row := m.renameRow
	switch row.kind {
	case rowProject:
		name := sanitizeProjectName(m.input.Value())
		if name == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if name == row.project {
			m.mode = inputNone
			m.renameRow = treeRow{}
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
		m.renameRow = treeRow{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return projectRenamedMsg{oldName: row.project, newName: name}
		}
	case rowSession:
		name := sanitizeSessionName(m.input.Value())
		if name == "" {
			m.status = "Section name is empty."
			return m, nil
		}
		if name == row.session {
			m.mode = inputNone
			m.renameRow = treeRow{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if _, ok := m.findSession(name); ok {
			m.status = "Section already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameRow = treeRow{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return sessionRenamedMsg{oldName: row.session, newName: name, err: m.tmux.RenameSession(row.session, name)}
		}
	default:
		m.status = "Select a project or session to rename."
		return m, nil
	}
}

func (m model) deleteSelectedProject() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok || row.kind != rowProject {
		m.status = "Select a project to delete."
		return m, nil
	}
	project := normalizeProjectName(row.project)
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
	delete(m.expandedProjects, project)
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

func (m *model) commitProjectDir() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	dir := strings.TrimSpace(m.input.Value())
	cfg := m.projectConfig(project)
	m.mode = inputNone
	m.input.Blur()
	m.input.Prompt = ""
	if dir == "" {
		cfg.Workdir = ""
	} else {
		cfg.Workdir = normalizeCWD(dir)
	}
	m.setProjectConfig(cfg)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
	return m, nil
}

func (m model) commitMoveProject() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.mode = inputNone
		m.moveQuery = ""
		m.status = "No session selected."
		return m, nil
	}
	project, ok := m.singleMatchingProject()
	if !ok {
		m.status = "No unique project match."
		return m, nil
	}
	m.mode = inputNone
	return m, func() tea.Msg {
		return sessionMovedMsg{session: s.Name, project: project}
	}
}
