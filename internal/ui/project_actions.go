package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) beginProjectDelete() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	if project == "" {
		m.status = "No project selected."
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{project: project}
	m.status = ""
	return m, nil
}

func (m *model) beginProjectRename() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	if project == "" {
		m.status = "No project selected."
		return m, nil
	}
	m.mode = inputRename
	m.renameTarget = renameTarget{project: project}
	m.input.SetValue(project)
	m.input.CursorEnd()
	m.input.Prompt = ""
	m.input.Focus()
	m.status = ""
	return m, nil
}

func (m model) deleteProject(project string) (tea.Model, tea.Cmd) {
	project = normalizeProjectName(project)
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

	nextProject := m.nextProjectAfter(project, true)
	deletedSessions := m.projectSessions(project)
	// A blank m.currentSession means the client's active session is unknown
	// to this model instance; treat that as if the active session belonged
	// to the deleted project so a valid attachment still gets established,
	// matching prior behavior for that (test-only) case.
	activeProjectDeleted := m.currentSession == ""
	for _, s := range deletedSessions {
		if s.Name == m.currentSession {
			activeProjectDeleted = true
		}
	}
	for _, s := range deletedSessions {
		delete(m.sessionProjects, s.Name)
		delete(m.sessionLabels, s.Name)
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
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.syncSelection()
	// Every session in the deleted project was already killed above, and no
	// other project's sessions are affected by removing this one, so there
	// is nothing left to write.
	m.err = nil
	m.status = ""
	if !activeProjectDeleted {
		return m, nil
	}
	if nextProject != "" {
		return m.switchToProject(nextProject)
	}
	return m.createVolatileFallback()
}
