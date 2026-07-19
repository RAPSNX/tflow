package ui

import (
	"hash/fnv"
	"strings"
)

func (m model) saveState() error {
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
	return saveAppState(m.statePath, normalizeAppState(state))
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
