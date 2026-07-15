package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m *model) shiftSession(delta int) {
	sessions := m.contextSessions()
	if len(sessions) == 0 {
		return
	}

	index := m.selectedSessionIndex()
	if index < 0 {
		if delta < 0 {
			index = 0
		} else {
			index = len(sessions) - 1
		}
	}
	index = (index + delta + len(sessions)) % len(sessions)
	m.selectedSession = sessions[index].Name
}

func (m model) selectedSessionInfo() (session, bool) {
	if m.selectedSession == "" {
		return session{}, false
	}
	return m.findSession(m.selectedSession)
}

func (m model) currentSessionInfo() (session, bool) {
	if strings.TrimSpace(m.currentSession) == "" {
		return session{}, false
	}
	return m.findSession(m.currentSession)
}

func (m model) currentProject() string {
	current, ok := m.currentSessionInfo()
	if !ok || current.Temporary {
		return ""
	}
	return normalizeProjectName(m.sessionProjects[current.Name])
}

func (m model) findSession(name string) (session, bool) {
	for _, s := range m.sessions {
		if s.Name == name {
			return s, true
		}
	}
	return session{}, false
}

func (m model) contextProject() string {
	if m.selectedProject != "" {
		return m.selectedProject
	}
	if m.selectedSession != "" {
		return normalizeProjectName(m.sessionProjects[m.selectedSession])
	}
	return m.currentProject()
}

func (m *model) syncSelection() {
	m.projects = normalizeProjectList(m.projects)
	if !containsString(m.projects, m.selectedProject) {
		m.selectedProject = ""
	}
	if m.selectedProject == "" {
		if current, ok := m.currentSessionInfo(); ok && !current.Temporary {
			m.selectedProject = normalizeProjectName(m.sessionProjects[current.Name])
		}
	}
	if m.selectedProject == "" && len(m.projects) > 0 {
		m.selectedProject = m.projects[0]
	}

	sessions := m.contextSessions()
	if !containsSessionName(sessions, m.selectedSession) {
		m.selectedSession = ""
	}
	if m.selectedSession == "" {
		if current, ok := m.currentSessionInfo(); ok && !current.Temporary {
			currentProject := normalizeProjectName(m.sessionProjects[current.Name])
			if m.selectedProject == "" || currentProject == m.selectedProject {
				m.selectedSession = current.Name
				if currentProject != "" {
					m.selectedProject = currentProject
				}
			}
		}
	}
	if m.selectedSession == "" && len(sessions) > 0 {
		m.selectedSession = sessions[0].Name
	}
}

func (m *model) ensureSessionProjects() bool {
	changed := false
	if m.sessionProjects == nil {
		m.sessionProjects = map[string]string{}
		changed = true
	}
	if m.sessionTypes == nil {
		m.sessionTypes = map[string]sessionType{}
		changed = true
	}
	for _, s := range m.sessions {
		if s.Temporary {
			continue
		}
		project := normalizeProjectName(m.sessionProjects[s.Name])
		if m.sessionProjects[s.Name] != project {
			m.sessionProjects[s.Name] = project
			changed = true
		}
		if project != "" && !containsString(m.projects, project) {
			m.projects = append(m.projects, project)
			changed = true
		}
		if _, ok := m.sessionTypes[s.Name]; !ok {
			m.sessionTypes[s.Name] = sessionTypeTerminal
			changed = true
		}
	}
	m.projects = normalizeProjectList(m.projects)
	return changed
}

func (m *model) assignSessionProject(name, project string) {
	project = normalizeProjectName(project)
	if m.sessionProjects == nil {
		m.sessionProjects = map[string]string{}
	}
	m.sessionProjects[name] = project
	if project != "" {
		m.addProject(project)
		m.setProjectConfig(projectConfig{Name: project})
	}
}

func (m *model) addProject(name string) {
	name = normalizeProjectName(name)
	if name == "" {
		return
	}
	if !containsString(m.projects, name) {
		m.projects = append(m.projects, name)
		m.projects = normalizeProjectList(m.projects)
	}
}

func (m model) matchingProjects(query string) []string {
	query = normalizeProjectName(query)
	if query == "" {
		return append([]string(nil), m.projects...)
	}
	matches := make([]string, 0, len(m.projects))
	for _, project := range m.projects {
		if strings.HasPrefix(project, query) {
			matches = append(matches, project)
		}
	}
	return matches
}

func (m model) uniqueMatchingProject(query string) (string, bool) {
	matches := m.matchingProjects(query)
	if len(matches) != 1 {
		return "", false
	}
	return matches[0], true
}

func (m model) projectSessions(project string) []session {
	project = normalizeProjectName(project)
	result := make([]session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.Temporary {
			continue
		}
		if normalizeProjectName(m.sessionProjects[s.Name]) == project {
			result = append(result, s)
		}
	}
	return result
}

func (m model) contextSessions() []session {
	project := m.selectedProject
	if project == "" {
		result := make([]session, 0, len(m.sessions))
		for _, s := range m.sessions {
			if s.Temporary {
				continue
			}
			result = append(result, s)
		}
		return result
	}
	return m.projectSessions(project)
}

func (m model) selectedSessionIndex() int {
	sessions := m.contextSessions()
	for index, s := range sessions {
		if s.Name == m.selectedSession {
			return index
		}
	}
	return -1
}

func (m model) rowStyle(selected bool, projectName string) lipgloss.Style {
	if selected {
		return selectedSessionStyle
	}
	return sessionStyle.Copy().Foreground(lipgloss.Color(projectAccentColor(projectName)))
}

func (m model) visibleSessionCount() int {
	return len(m.contextSessions())
}

func containsSessionName(sessions []session, want string) bool {
	for _, s := range sessions {
		if s.Name == want {
			return true
		}
	}
	return false
}
