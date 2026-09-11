package ui

import (
	"fmt"
	"hash/fnv"
	"strings"
)

func (m *model) saveState() error {
	desired := m.currentState()
	base := m.stateBase
	if m.stateBasePath != m.statePath {
		base = appState{}
	}
	mutate := func(latest appState) (appState, error) {
		if err := validateStateProjectNames(latest, base, desired); err != nil {
			return appState{}, err
		}
		state := mergeAppStates(latest, base, desired)
		if err := validateAppState(state); err != nil {
			return appState{}, err
		}
		return state, nil
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
		return err
	}
	m.stateBase = desired
	m.stateBasePath = m.statePath
	m.persistentSessionOrder = make(map[string][]string, len(state.Projects))
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			m.persistentSessionOrder[project.Name] = append(m.persistentSessionOrder[project.Name], session.ID)
		}
	}
	return nil
}

func (m *model) currentState() appState {
	state := appState{Projects: make([]storedProject, 0, len(m.projects))}
	for _, name := range m.projects {
		name = normalizeProjectName(name)
		if name == "" {
			continue
		}
		cfg := normalizeProjectConfig(m.projectConfig(name))
		project := storedProject{Name: name, Workdir: cfg.Workdir, AgentBinary: cfg.AgentBinary, Sessions: []persistentSession{}}
		for _, session := range m.projectSessions(name) {
			project.Sessions = append(project.Sessions, persistentSession{
				ID:      session.Name,
				Label:   m.sessionLabel(session.Name),
				Type:    strings.TrimSpace(m.sessionTypes[session.Name]),
				Command: m.sessionCommand(session.Name),
			})
		}
		state.Projects = append(state.Projects, project)
	}
	return normalizeAppState(state)
}

type stateSession struct {
	project     string
	label       string
	sessionType string
	command     string
}

func mergeAppStates(latest, base, desired appState) appState {
	latest = normalizeAppState(latest)
	base = normalizeAppState(base)
	desired = normalizeAppState(desired)
	baseProjects := stateProjects(base)
	desiredProjects := stateProjects(desired)

	for name := range baseProjects {
		if _, exists := desiredProjects[name]; !exists {
			latest.Projects = removeStateProject(latest.Projects, name)
		}
	}
	for _, project := range desired.Projects {
		baseProject, existed := baseProjects[project.Name]
		if !existed {
			ensureStateProject(&latest, project)
			continue
		}
		if project.Workdir != baseProject.Workdir || project.AgentBinary != baseProject.AgentBinary {
			mergeStateProjectFields(&latest, project, baseProject)
		}
	}

	baseSessions := stateSessions(base)
	desiredSessions := stateSessions(desired)
	for id := range baseSessions {
		if _, exists := desiredSessions[id]; !exists {
			removeStateSession(&latest, id)
		}
	}
	for _, project := range desired.Projects {
		for _, desiredSession := range project.Sessions {
			id := desiredSession.ID
			session := stateSession{project: project.Name, label: desiredSession.Label, sessionType: desiredSession.Type, command: desiredSession.Command}
			baseSession, existed := baseSessions[id]
			if existed && baseSession == session {
				continue
			}
			ensureStateProjectExists(&latest, project)
			removeStateSession(&latest, id)
			for index := range latest.Projects {
				if latest.Projects[index].Name == session.project {
					latest.Projects[index].Sessions = append(latest.Projects[index].Sessions, persistentSession{ID: id, Label: session.label, Type: session.sessionType, Command: session.command})
					break
				}
			}
		}
	}
	return normalizeAppState(latest)
}

func validateStateProjectNames(latest, base, desired appState) error {
	latestProjects := stateProjects(latest)
	baseProjects := stateProjects(base)
	for name := range stateProjects(desired) {
		if _, existed := baseProjects[name]; existed {
			continue
		}
		if _, exists := latestProjects[name]; exists {
			return fmt.Errorf("project already exists")
		}
	}
	return nil
}

func stateProjects(state appState) map[string]storedProject {
	projects := make(map[string]storedProject, len(state.Projects))
	for _, project := range state.Projects {
		projects[project.Name] = project
	}
	return projects
}

func stateSessions(state appState) map[string]stateSession {
	sessions := map[string]stateSession{}
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			sessions[session.ID] = stateSession{project: project.Name, label: session.Label, sessionType: session.Type, command: session.Command}
		}
	}
	return sessions
}

// mergeStateProjectFields applies only the project fields this editor
// actually changed (desired vs. its own base snapshot) onto latest,
// leaving every other field as latest already has it -- a concurrent
// instance may have changed a different field on the same project, and
// copying the whole desired project over would silently revert that
// unrelated, already-saved edit.
func mergeStateProjectFields(state *appState, desired, base storedProject) {
	for index := range state.Projects {
		if state.Projects[index].Name != desired.Name {
			continue
		}
		if desired.Workdir != base.Workdir {
			state.Projects[index].Workdir = desired.Workdir
		}
		if desired.AgentBinary != base.AgentBinary {
			state.Projects[index].AgentBinary = desired.AgentBinary
		}
		return
	}
	// The project is no longer in latest (e.g. removed by a concurrent
	// save); there is nothing to merge fields into, so reinsert it whole.
	ensureStateProject(state, desired)
}

func ensureStateProject(state *appState, project storedProject) {
	for index := range state.Projects {
		if state.Projects[index].Name == project.Name {
			state.Projects[index].Workdir = project.Workdir
			state.Projects[index].AgentBinary = project.AgentBinary
			return
		}
	}
	state.Projects = append(state.Projects, storedProject{Name: project.Name, Workdir: project.Workdir, AgentBinary: project.AgentBinary, Sessions: []persistentSession{}})
}

func ensureStateProjectExists(state *appState, project storedProject) {
	for _, existing := range state.Projects {
		if existing.Name == project.Name {
			return
		}
	}
	state.Projects = append(state.Projects, storedProject{Name: project.Name, Workdir: project.Workdir, AgentBinary: project.AgentBinary, Sessions: []persistentSession{}})
}

func removeStateProject(projects []storedProject, name string) []storedProject {
	result := projects[:0]
	for _, project := range projects {
		if project.Name != name {
			result = append(result, project)
		}
	}
	return result
}

func removeStateSession(state *appState, id string) {
	for projectIndex := range state.Projects {
		sessions := state.Projects[projectIndex].Sessions[:0]
		for _, session := range state.Projects[projectIndex].Sessions {
			if session.ID != id {
				sessions = append(sessions, session)
			}
		}
		state.Projects[projectIndex].Sessions = sessions
	}
}

func sanitizeProjectName(name string) string {
	return normalizeProjectName(name)
}

func projectAccentColor(project string) string {
	palette := []string{
		"#89b4fa",
		"#94e2d5",
		"#f9e2af",
		"#f38ba8",
		"#cba6f7",
		"#f5c2e7",
		"#fab387",
		"#74c7ec",
		"#a6e3a1",
	}
	project = normalizeProjectName(project)
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(project))
	return palette[hasher.Sum32()%uint32(len(palette))]
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func indexOfString(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}

func removeProject(projects []string, target string) []string {
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		if project == target {
			continue
		}
		result = append(result, project)
	}
	return normalizeProjectList(result)
}

func filterSessions(sessions []session, keep func(session) bool) []session {
	result := make([]session, 0, len(sessions))
	for _, s := range sessions {
		if keep(s) {
			result = append(result, s)
		}
	}
	return result
}

func replaceProject(projects []string, oldName, newName string) []string {
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		if project == oldName {
			result = append(result, newName)
			continue
		}
		result = append(result, project)
	}
	return normalizeProjectList(result)
}

func (m model) statusView() string {
	if strings.TrimSpace(m.status) == "" {
		return ""
	}
	style := warningStatusStyle
	if m.err != nil && m.status == m.err.Error() {
		style = errorStatusStyle
	}
	return style.Width(max(20, m.width-6)).Render(m.status)
}

// syncSessionMarkers writes the tmux project and label markers for a single
// session, using the model's current in-memory metadata for it. Callers must
// scope this to the session(s) a mutation directly affects; it must never be
// used to resync every session in the fleet.
func (m model) syncSessionMarkers(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	project := normalizeProjectName(m.sessionProjects[name])
	if err := m.tmux.SetSessionProject(name, project); err != nil {
		return err
	}
	label := strings.TrimSpace(m.sessionLabel(name))
	if label == "" {
		label = name
	}
	return m.tmux.SetSessionLabel(name, label)
}
