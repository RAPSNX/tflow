package ui

import (
	"fmt"
	"hash/fnv"
	"strings"
)

func (m *model) saveState() error {
	desired := m.currentState()
	var unlock func() error
	if !m.stateLockHeld {
		var err error
		unlock, err = lockAppState(m.statePath)
		if err != nil {
			return err
		}
		defer func() { _ = unlock() }()
	}

	latest, err := loadAppState(m.statePath)
	if err != nil {
		return err
	}
	base := m.stateBase
	if m.stateBasePath != m.statePath {
		base = appState{}
	}
	state := mergeAppStates(latest, base, desired)
	if err := validateStateSessionLabels(state); err != nil {
		return err
	}
	if err := saveAppState(m.statePath, state); err != nil {
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
		project := storedProject{Name: name, Workdir: cfg.Workdir, Sessions: []persistentSession{}}
		for _, session := range m.projectSessions(name) {
			project.Sessions = append(project.Sessions, persistentSession{ID: session.Name, Label: m.sessionLabel(session.Name)})
		}
		state.Projects = append(state.Projects, project)
	}
	return normalizeAppState(state)
}

type stateSession struct {
	project string
	label   string
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
		if project.Workdir != baseProject.Workdir {
			ensureStateProject(&latest, project)
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
			session := stateSession{project: project.Name, label: desiredSession.Label}
			baseSession, existed := baseSessions[id]
			if existed && baseSession == session {
				continue
			}
			ensureStateProject(&latest, project)
			removeStateSession(&latest, id)
			for index := range latest.Projects {
				if latest.Projects[index].Name == session.project {
					latest.Projects[index].Sessions = append(latest.Projects[index].Sessions, persistentSession{ID: id, Label: session.label})
					break
				}
			}
		}
	}
	return normalizeAppState(latest)
}

func validateStateSessionLabels(state appState) error {
	for _, project := range state.Projects {
		labels := map[string]struct{}{}
		for _, session := range project.Sessions {
			if _, exists := labels[session.Label]; exists {
				return fmt.Errorf("session name already exists in this project")
			}
			labels[session.Label] = struct{}{}
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
			sessions[session.ID] = stateSession{project: project.Name, label: session.Label}
		}
	}
	return sessions
}

func ensureStateProject(state *appState, project storedProject) {
	for index := range state.Projects {
		if state.Projects[index].Name == project.Name {
			state.Projects[index].Workdir = project.Workdir
			return
		}
	}
	state.Projects = append(state.Projects, storedProject{Name: project.Name, Workdir: project.Workdir, Sessions: []persistentSession{}})
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

func (m model) syncTmuxSessionProjects() error {
	sessionProjects := make(map[string]string, len(m.sessions))
	sessionLabels := make(map[string]string, len(m.sessions))
	for _, s := range m.sessions {
		project := normalizeProjectName(m.sessionProjects[s.Name])
		sessionProjects[s.Name] = project
		sessionLabels[s.Name] = m.sessionLabel(s.Name)
	}
	return m.tmux.SyncSessionProjects(sessionProjects, sessionLabels)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
