package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/store"
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

	// Source the tmux label marker from the label the mutation actually
	// observed on disk, not from this popup's pre-mutation model.
	// mutateAppState/mutateAppStateLocked reload the latest on-disk state
	// before applying store.MoveSession, and MoveSession is a pure project
	// reassignment that preserves whatever label the moved session already
	// has. If another tflow instance renamed this session between when this
	// popup's model was built and now, that rename landed in the reloaded
	// `state` first. Falling back to m.sessionLabel(sessionName) (the stale
	// model) would write the old label back into tmux and clobber the
	// concurrent rename until a future full sync.
	label := m.sessionLabel(sessionName)
	if moved, ok := stateSessions(state)[sessionName]; ok {
		label = moved.label
	}

	// Reflect the persisted result into in-memory bookkeeping only now that
	// the store mutation has succeeded. Use the same reloaded label the
	// tmux write below uses, not the pre-mutation model's label, so this
	// popup's own view doesn't stay stale after a concurrent rename.
	m.assignSessionProject(sessionName, targetProject)
	m.setSessionLabel(sessionName, label)
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
	m.persistentSessionOrder = make(map[string][]string, len(state.Projects))
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			m.persistentSessionOrder[project.Name] = append(m.persistentSessionOrder[project.Name], session.ID)
		}
	}
	// Anchor stateBase to this model's own tracked view (mirroring
	// saveState's base=desired invariant) rather than the raw state the
	// mutation observed. mutateAppState reloads the latest on-disk state
	// before applying the move, so `state` can include projects, sessions,
	// or label changes written concurrently by another tflow instance that
	// this model never folded into m.projects/m.sessionProjects/
	// m.sessionLabels (it only applies the moved session above). Recording
	// that broader `state` as stateBase would make a later saveState's
	// three-way merge see those concurrent entries as present in base but
	// absent from desired -- i.e. as if this sidebar intentionally deleted
	// or reverted them. Using m.currentState() keeps stateBase consistent
	// with what this model actually knows, so unrelated concurrent state is
	// neither base nor desired at the next save and passes through
	// untouched instead of being diffed away.
	m.stateBase = m.currentState()
	m.stateBasePath = m.statePath
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
	return m, m.closeMenuCmd()
}
