package ui

import (
	"hash/fnv"
	"strings"
)

func (m model) saveState() error {
	state := appState{
		Projects:        append([]string(nil), m.projects...),
		SessionProjects: map[string]string{},
		SessionTypes:    map[string]string{},
		ProjectConfigs:  map[string]projectConfig{},
	}
	for name, project := range m.sessionProjects {
		state.SessionProjects[name] = normalizeProjectName(project)
	}
	for name, sessionType := range m.sessionTypes {
		state.SessionTypes[name] = string(sessionType)
	}
	for project, cfg := range m.projectConfigs {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		cfg = normalizeProjectConfig(cfg)
		cfg.Name = project
		state.ProjectConfigs[project] = cfg
	}
	return saveAppState(m.statePath, normalizeAppState(state))
}

func sanitizeProjectName(name string) string {
	return normalizeProjectName(name)
}

func projectSessionName(project, name string) string {
	return sanitizeSessionName(project) + "--" + sanitizeSessionName(name)
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
	style := statusStyle
	if m.err != nil {
		style = errorStatusStyle
	}
	return style.Width(max(20, m.width-6)).Render(m.status)
}

func (m model) syncTmuxSessionProjects() error {
	sessionProjects := make(map[string]string, len(m.sessions))
	for _, s := range m.sessions {
		project := normalizeProjectName(m.sessionProjects[s.Name])
		sessionProjects[s.Name] = project
	}
	return m.tmux.SyncSessionProjects(sessionProjects)
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
