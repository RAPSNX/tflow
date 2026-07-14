package ui

import (
	"hash/fnv"
	"strings"
	"unicode"
)

func (m model) saveState() error {
	state := appState{
		Projects:         append([]string(nil), m.projects...),
		SessionProjects:  map[string]string{},
		SessionTypes:     map[string]string{},
		ProjectDirs:      map[string]string{},
		ExpandedProjects: map[string]bool{},
	}
	for name, project := range m.sessionProjects {
		state.SessionProjects[name] = normalizeProjectName(project)
	}
	for name, sessionType := range m.sessionTypes {
		state.SessionTypes[name] = string(sessionType)
	}
	for project, cfg := range m.projectConfigs {
		project = normalizeProjectName(project)
		dir := strings.TrimSpace(cfg.Workdir)
		if project == "" || dir == "" {
			continue
		}
		state.ProjectDirs[project] = normalizeCWD(dir)
	}
	for project, expanded := range m.expandedProjects {
		state.ExpandedProjects[normalizeProjectName(project)] = expanded
	}
	state = normalizeAppState(state)
	if err := saveAppState(m.statePath, state); err != nil {
		return err
	}
	return saveProjectConfigs(m.statePath, m.projectConfigs)
}

func normalizeProjectList(projects []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(projects))
	add := func(name string) {
		name = normalizeProjectName(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	for _, project := range projects {
		add(project)
	}
	return result
}

func normalizeProjectName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
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

func containsString(values []string, want string) bool {
	return indexOfString(values, want) >= 0
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
