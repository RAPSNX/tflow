package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) shiftRow(delta int) {
	rows := m.treeRows()
	if len(rows) == 0 {
		return
	}
	index := m.selectedRowIndex()
	if index < 0 {
		index = 0
	}
	index = (index + delta + len(rows)) % len(rows)
	m.selectRow(rows[index])
}

func (m *model) collapseSelection() {
	row, ok := m.selectedRow()
	if !ok {
		return
	}
	switch row.kind {
	case rowSession:
		m.selectedProject = row.project
		m.selectedSession = ""
	case rowProject:
		if m.expandedProjects[row.project] {
			m.expandedProjects[row.project] = false
			m.selectedProject = row.project
			m.selectedSession = ""
			_ = m.saveState()
		}
	}
}

func (m *model) expandSelection() {
	row, ok := m.selectedRow()
	if !ok {
		return
	}
	switch row.kind {
	case rowProject:
		if !m.expandedProjects[row.project] {
			m.expandedProjects[row.project] = true
			_ = m.saveState()
		}
	case rowSession:
		m.selectedProject = row.project
		m.selectedSession = row.session
	}
}

func (m *model) toggleProject(project string) {
	m.expandedProjects[project] = !m.expandedProjects[project]
	if !m.expandedProjects[project] {
		m.selectedProject = project
		m.selectedSession = ""
	}
	_ = m.saveState()
}

func (m model) renameValue(row treeRow) string {
	if row.kind == rowProject {
		return row.project
	}
	return row.session
}

func (m model) treeRows() []treeRow {
	rows := make([]treeRow, 0, len(m.projects))
	for _, project := range m.projects {
		rows = append(rows, treeRow{kind: rowProject, project: project})
		if !m.expandedProjects[project] {
			continue
		}
		for _, s := range m.projectSessions(project) {
			rows = append(rows, treeRow{
				kind:    rowSession,
				project: project,
				session: s.Name,
				depth:   1,
			})
		}
	}
	return rows
}

func (m model) selectedRow() (treeRow, bool) {
	rows := m.treeRows()
	index := m.selectedRowIndex()
	if index < 0 || index >= len(rows) {
		return treeRow{}, false
	}
	return rows[index], true
}

func (m model) selectedRowIndex() int {
	rows := m.treeRows()
	if len(rows) == 0 {
		return -1
	}
	for index, row := range rows {
		if row.kind == rowSession && row.session == m.selectedSession && row.project == m.selectedProject {
			return index
		}
		if row.kind == rowProject && row.project == m.selectedProject && m.selectedSession == "" {
			return index
		}
	}
	for index, row := range rows {
		if row.kind == rowSession && row.session == m.currentSession {
			return index
		}
	}
	return 0
}

func (m *model) selectRow(row treeRow) {
	m.selectedProject = row.project
	if row.kind == rowSession {
		m.selectedSession = row.session
		return
	}
	m.selectedSession = ""
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

func (m model) findSession(name string) (session, bool) {
	for _, s := range m.sessions {
		if s.Name == name {
			return s, true
		}
	}
	return session{}, false
}

func (m model) addCurrentTempSession() (tea.Model, tea.Cmd) {
	current, ok := m.currentSessionInfo()
	if !ok {
		m.status = "No current session to add."
		return m, nil
	}
	if !current.Temporary {
		m.status = "Current session is not temporary."
		return m, nil
	}
	project := m.contextProject()
	return m, func() tea.Msg {
		err := m.tmux.SetSessionTemporary(current.Name, false)
		return sessionMovedMsg{session: current.Name, project: project, err: err}
	}
}

func (m model) contextProject() string {
	if m.selectedProject != "" {
		return m.selectedProject
	}
	if m.selectedSession != "" {
		return normalizeProjectName(m.sessionProjects[m.selectedSession])
	}
	return ""
}

func (m *model) syncSelection() {
	m.projects = normalizeProjectList(m.projects)
	if m.expandedProjects == nil {
		m.expandedProjects = map[string]bool{}
	}
	for _, project := range m.projects {
		if _, ok := m.expandedProjects[project]; !ok {
			m.expandedProjects[project] = true
		}
	}
	if !containsString(m.projects, m.selectedProject) {
		m.selectedProject = ""
	}
	if m.selectedSession != "" {
		if _, ok := m.findSession(m.selectedSession); !ok {
			m.selectedSession = ""
		}
	}
	if m.selectedSession == "" && m.currentSession != "" {
		if current, ok := m.findSession(m.currentSession); ok && !current.Temporary {
			m.selectedSession = m.currentSession
			m.selectedProject = normalizeProjectName(m.sessionProjects[m.currentSession])
		}
	}
	if m.selectedProject == "" && len(m.projects) > 0 {
		m.selectedProject = m.projects[0]
	}
	if m.selectedProject != "" && !m.expandedProjects[m.selectedProject] {
		m.expandedProjects[m.selectedProject] = true
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
	if m.expandedProjects == nil {
		m.expandedProjects = map[string]bool{}
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
		if !containsString(m.projects, project) {
			m.projects = append(m.projects, project)
			changed = true
		}
		if _, ok := m.expandedProjects[project]; !ok {
			m.expandedProjects[project] = true
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
	}
	if project != "" {
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
	if m.expandedProjects == nil {
		m.expandedProjects = map[string]bool{}
	}
	if _, ok := m.expandedProjects[name]; !ok {
		m.expandedProjects[name] = true
	}
}

func (m model) projectSessions(project string) []session {
	project = normalizeProjectName(project)
	result := make([]session, 0, len(m.sessions))
	for _, s := range m.sessions {
		if normalizeProjectName(m.sessionProjects[s.Name]) == project {
			result = append(result, s)
		}
	}
	return result
}

func (m model) matchingProjects(prefix string) []string {
	prefix = normalizeProjectName(prefix)
	if prefix == "" {
		return append([]string(nil), m.projects...)
	}
	matches := make([]string, 0, len(m.projects))
	for _, project := range m.projects {
		if strings.HasPrefix(strings.ToLower(project), prefix) {
			matches = append(matches, project)
		}
	}
	return matches
}

func (m model) rowStyle(selected, project bool, projectName string) lipgloss.Style {
	switch {
	case project && selected:
		return selectedProjectStyle
	case project:
		return projectStyle.Copy().Foreground(lipgloss.Color(projectAccentColor(projectName)))
	case selected:
		return selectedSessionStyle
	default:
		return sessionStyle.Copy().Foreground(lipgloss.Color(projectAccentColor(projectName)))
	}
}

func (m model) visibleSessionCount() int {
	count := 0
	for _, session := range m.sessions {
		if session.Temporary {
			continue
		}
		count++
	}
	return count
}

func (m model) singleMatchingProject() (string, bool) {
	matches := m.matchingProjects(m.moveQuery)
	if len(matches) == 1 {
		return matches[0], true
	}
	return "", false
}
