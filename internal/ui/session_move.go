package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"tflow/internal/store"
)

// beginSessionMove starts the target-project picker for moving the selected
// persistent session into another project. It mirrors beginProjectSwitch's
// picker UX, scoped to projects other than the session's current one.
func (m *model) beginSessionMove() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	if s.Temporary {
		m.status = "Volatile sessions cannot be moved."
		return m, nil
	}
	if len(m.matchingMoveProjectsFor(s.Name, "")) == 0 {
		m.status = "No other project to move into."
		return m, nil
	}
	m.mode = inputMoveSession
	m.moveTarget = moveTarget{session: s.Name}
	m.moveProjectIndex = 0
	m.input.Prompt = ""
	m.input.SetValue("")
	m.input.Focus()
	m.status = ""
	return m, nil
}

// matchingMoveProjectsFor lists projects matching query, excluding the
// session's current project.
func (m model) matchingMoveProjectsFor(sessionName, query string) []string {
	source := normalizeProjectName(m.sessionProjects[sessionName])
	all := m.matchingProjects(query)
	matches := make([]string, 0, len(all))
	for _, project := range all {
		if project != source {
			matches = append(matches, project)
		}
	}
	return matches
}

func (m model) matchingMoveProjects(query string) []string {
	return m.matchingMoveProjectsFor(m.moveTarget.session, query)
}

func (m *model) shiftMoveProjectSwitch(delta int) {
	matches := m.matchingMoveProjects(m.input.Value())
	if len(matches) == 0 {
		m.moveProjectIndex = 0
		return
	}
	m.moveProjectIndex = (m.moveProjectIndex + delta + len(matches)) % len(matches)
}

func (m *model) selectedMoveProject() (string, bool) {
	matches := m.matchingMoveProjects(m.input.Value())
	if len(matches) == 0 {
		return "", false
	}
	if m.moveProjectIndex < 0 || m.moveProjectIndex >= len(matches) {
		m.moveProjectIndex = 0
	}
	return matches[m.moveProjectIndex], true
}

func (m *model) commitSessionMove() (tea.Model, tea.Cmd) {
	target, ok := m.selectedMoveProject()
	sessionName := m.moveTarget.session
	m.mode = inputNone
	m.moveTarget = moveTarget{}
	m.moveProjectIndex = 0
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	if !ok {
		m.status = "No matching project selected."
		return m, nil
	}
	return m.applySessionMove(sessionName, target)
}

// applySessionMove persists the move through the store's single advisory-lock
// mutation path and, only on success, reflects the change into in-memory
// state and writes tmux markers for the moved session alone. It never
// switches the attached client: a move preserves the session's tmux
// identity, so the sidebar and status indicators simply reflect the new
// project on their next read of sessionProjects.
func (m model) applySessionMove(sessionName, targetProject string) (tea.Model, tea.Cmd) {
	targetProject = normalizeProjectName(targetProject)
	selected, found := m.findSession(sessionName)
	if !found || selected.Temporary {
		m.status = "Session no longer exists."
		return m, nil
	}
	sourceProject := normalizeProjectName(m.sessionProjects[sessionName])
	if targetProject == "" {
		m.status = "No target project selected."
		return m, nil
	}
	if sourceProject == targetProject {
		m.status = "Session already belongs to this project."
		return m, nil
	}

	mutate := func(latest appState) (appState, error) {
		return store.MoveSession(latest, sessionName, targetProject)
	}
	var (
		state appState
		err   error
	)
	if m.stateLockHeld {
		state, err = mutateAppStateLocked(m.statePath, mutate)
	} else {
		state, err = mutateAppState(m.statePath, mutate)
	}
	if err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}

	label := m.sessionLabel(sessionName)

	// Reflect the persisted result into in-memory bookkeeping only now that
	// the store mutation has succeeded.
	m.assignSessionProject(sessionName, targetProject)
	sourceRemains := false
	for _, project := range state.Projects {
		if project.Name == sourceProject {
			sourceRemains = true
			break
		}
	}
	if !sourceRemains {
		m.projects = removeProject(m.projects, sourceProject)
		delete(m.projectConfigs, sourceProject)
	}
	m.stateBase = state
	m.stateBasePath = m.statePath
	m.persistentSessionOrder = make(map[string][]string, len(state.Projects))
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			m.persistentSessionOrder[project.Name] = append(m.persistentSessionOrder[project.Name], session.ID)
		}
	}
	m.selectedProject = targetProject
	m.selectedSession = sessionName
	m.syncSelection()

	// Update tmux markers only for the session that moved; do not resync
	// unrelated sessions.
	if err := m.tmux.SetSessionProject(sessionName, targetProject); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if err := m.tmux.SetSessionLabel(sessionName, label); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = fmt.Sprintf("Moved %s to %s.", label, targetProject)
	return m, nil
}
