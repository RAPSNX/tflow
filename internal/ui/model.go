package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputCreateSession
	inputCreateProject
	inputMoveProject
)

const defaultProjectName = "default"

type sessionsLoadedMsg struct {
	sessions []session
	err      error
}

type sessionCreatedMsg struct {
	session session
	err     error
}

type sessionKilledMsg struct {
	name string
	err  error
}

type projectCreatedMsg struct {
	name string
	err  error
}

type sessionMovedMsg struct {
	session string
	project string
	err     error
}

type appState struct {
	Projects         []string          `json:"projects"`
	SessionProjects  map[string]string `json:"session_projects"`
	ExpandedProjects map[string]bool   `json:"expanded_projects"`
}

type treeRowKind int

const (
	rowProject treeRowKind = iota
	rowSession
)

type treeRow struct {
	kind    treeRowKind
	project string
	session string
	depth   int
}

type model struct {
	manager sessionManager

	width  int
	height int

	mode inputMode

	sessions         []session
	projects         []string
	sessionProjects  map[string]string
	expandedProjects map[string]bool
	selectedProject  string
	selectedSession  string
	currentSession   string
	exitSession      string

	input     textinput.Model
	moveQuery string

	cwd       string
	statePath string

	status string
	err    error
}

func NewMenu() tea.Model {
	return newModel(newSessionManager(), "")
}

func Start() error {
	broker := newSessionBroker()
	if _, err := broker.CreateSession(defaultSessionName, defaultSessionDir()); err != nil {
		return err
	}
	return broker.runSession(defaultSessionName, defaultSessionDir())
}

func defaultSessionDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return cwd
	}
	return "."
}

func newModel(manager sessionManager, current string) tea.Model {
	cwd, _ := os.Getwd()

	input := textinput.New()
	input.CharLimit = 40
	input.Width = 28
	input.Blur()

	statePath := appStatePath()
	state, err := loadAppState(statePath)
	if err != nil {
		state = appState{
			Projects:         []string{defaultProjectName},
			SessionProjects:  map[string]string{},
			ExpandedProjects: map[string]bool{defaultProjectName: true},
		}
	}
	state = normalizeAppState(state)

	return model{
		manager:          manager,
		mode:             inputNone,
		projects:         state.Projects,
		sessionProjects:  state.SessionProjects,
		expandedProjects: state.ExpandedProjects,
		selectedProject:  defaultProjectName,
		currentSession:   current,
		input:            input,
		cwd:              cwd,
		statePath:        statePath,
		status:           "j/k move  h/l close/open project  enter open  n new session  p new project  m move  d delete project  x kill",
		err:              err,
	}
}

func runMenu(manager sessionManager, current string) (string, error) {
	final, err := tea.NewProgram(newModel(manager, current), tea.WithAltScreen()).Run()
	if err != nil {
		return "", err
	}
	if m, ok := final.(model); ok {
		return m.exitSession, nil
	}
	return "", nil
}

func RunMenu(current string) (string, error) {
	return runMenu(newSessionManager(), current)
}

func (m model) Init() tea.Cmd {
	return m.loadSessionsCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(16, min(28, m.width-8))
		return m, nil
	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.err = msg.err
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		changed := m.ensureSessionProjects()
		m.syncSelection()
		if changed {
			if err := m.saveState(); err != nil {
				m.err = err
				m.status = err.Error()
			}
		}
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		project := m.contextProject()
		m.assignSessionProject(msg.session.Name, project)
		m.expandedProjects[project] = true
		m.selectedProject = project
		m.selectedSession = msg.session.Name
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Created session %s in %s.", msg.session.Name, project)
		return m, m.loadSessionsCmd()
	case sessionKilledMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		delete(m.sessionProjects, msg.name)
		if m.selectedSession == msg.name {
			m.selectedSession = ""
		}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Killed session %s.", msg.name)
		return m, m.loadSessionsCmd()
	case projectCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.addProject(msg.name)
		m.expandedProjects[msg.name] = true
		m.selectedProject = msg.name
		m.selectedSession = ""
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Created project %s.", msg.name)
		return m, nil
	case sessionMovedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.assignSessionProject(msg.session, msg.project)
		m.expandedProjects[msg.project] = true
		m.selectedProject = msg.project
		m.selectedSession = msg.session
		m.moveQuery = ""
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Moved session %s to %s.", msg.session, msg.project)
		return m, m.loadSessionsCmd()
	case tea.KeyMsg:
		if m.mode != inputNone {
			return m.updateModal(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		m.exitSession = ""
		return m, tea.Quit
	case tea.KeyCtrlF:
		m.exitSession = ""
		return m, tea.Quit
	case tea.KeyEsc:
		m.exitSession = ""
		return m, tea.Quit
	}

	switch msg.String() {
	case "q":
		m.exitSession = ""
		return m, tea.Quit
	case "j", "down":
		m.shiftRow(1)
		return m, nil
	case "k", "up":
		m.shiftRow(-1)
		return m, nil
	case "h", "left":
		m.collapseSelection()
		return m, nil
	case "l", "right":
		m.expandSelection()
		return m, nil
	case "enter":
		row, ok := m.selectedRow()
		if !ok {
			return m, nil
		}
		if row.kind == rowProject {
			m.toggleProject(row.project)
			return m, nil
		}
		m.exitSession = row.session
		return m, tea.Quit
	case "n":
		m.mode = inputCreateSession
		m.input.Prompt = "session: "
		m.input.SetValue("")
		m.input.Focus()
		m.status = fmt.Sprintf("Create a new session in %s.", m.contextProject())
		return m, nil
	case "p":
		m.mode = inputCreateProject
		m.input.Prompt = "project: "
		m.input.SetValue("")
		m.input.Focus()
		m.status = "Create a new project."
		return m, nil
	case "m":
		if _, ok := m.selectedSessionInfo(); !ok {
			m.status = "Select a session to move."
			return m, nil
		}
		m.mode = inputMoveProject
		m.moveQuery = ""
		m.status = "Type a project prefix to move the selected session."
		return m, nil
	case "d":
		return m.deleteSelectedProject()
	case "x":
		return m.killSelectedSession()
	case "r":
		return m, m.loadSessionsCmd()
	}

	return m, nil
}

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case inputCreateSession:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Session creation cancelled."
			return m, nil
		case tea.KeyEnter:
			name := strings.TrimSpace(m.input.Value())
			if name == "" {
				m.status = "Session name is empty."
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			return m, func() tea.Msg {
				s, err := m.manager.CreateSession(name, m.cwd)
				if err != nil {
					return sessionCreatedMsg{err: err}
				}
				return sessionCreatedMsg{session: s, err: nil}
			}
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputCreateProject:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Project creation cancelled."
			return m, nil
		case tea.KeyEnter:
			name := sanitizeProjectName(m.input.Value())
			if name == "" {
				m.status = "Project name is empty."
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			return m, func() tea.Msg {
				return projectCreatedMsg{name: name}
			}
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputMoveProject:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.moveQuery = ""
			m.status = "Move cancelled."
			return m, nil
		case tea.KeyBackspace, tea.KeyDelete:
			if m.moveQuery != "" {
				m.moveQuery = m.moveQuery[:len(m.moveQuery)-1]
			}
			return m.resolveMoveProject()
		case tea.KeyEnter:
			return m.commitMoveProject()
		}
		if len(msg.String()) == 1 {
			r := []rune(msg.String())[0]
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				m.moveQuery += strings.ToLower(string(r))
				return m.resolveMoveProject()
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case inputCreateSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Session"))
	case inputCreateProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Project"))
	case inputMoveProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMoveOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	lines := []string{titleStyle.Render("tflow")}
	rows := m.treeRows()
	if len(rows) == 0 {
		lines = append(lines, mutedStyle.Render("No projects or sessions"))
	} else {
		for index, row := range rows {
			lines = append(lines, m.renderRow(index, row))
		}
	}
	lines = append(lines, "", m.statusView())
	return sidebarStyle.Width(max(24, m.width-4)).Height(max(8, m.height-2)).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderRow(index int, row treeRow) string {
	label := m.rowLabel(row)
	if index == m.selectedRowIndex() {
		return selectedItemStyle.Render(label)
	}
	return itemStyle.Render(label)
}

func (m model) rowLabel(row treeRow) string {
	switch row.kind {
	case rowProject:
		prefix := "[-]"
		if !m.expandedProjects[row.project] {
			prefix = "[+]"
		}
		return fmt.Sprintf("%s %s", prefix, row.project)
	case rowSession:
		session, _ := m.findSession(row.session)
		marker := "  "
		if row.session == m.currentSession {
			marker = "* "
		}
		suffix := ""
		if session.Windows > 0 {
			suffix = fmt.Sprintf(" (%d)", session.Windows)
		}
		return strings.Repeat(" ", row.depth*2) + marker + row.session + suffix
	default:
		return ""
	}
}

func (m model) renderInputOverlay(title string) string {
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("project: " + m.contextProject()),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderMoveOverlay() string {
	lines := []string{
		titleStyle.Render("Move Session"),
		mutedStyle.Render("prefix: " + fallbackText(m.moveQuery, "type letters")),
		"",
	}
	for _, project := range m.matchingProjects(m.moveQuery) {
		lines = append(lines, itemStyle.Render("  "+project))
	}
	if len(m.matchingProjects(m.moveQuery)) == 0 {
		lines = append(lines, mutedStyle.Render("No matching project."))
	}
	lines = append(lines, "", hintStyle.Render("Type until one project matches. Enter confirms."))
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.manager.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) killSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return sessionKilledMsg{name: s.Name, err: m.manager.KillSession(s.Name)}
	}
}

func (m model) deleteSelectedProject() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok || row.kind != rowProject {
		m.status = "Select a project to delete."
		return m, nil
	}
	project := normalizeProjectName(row.project)
	if project == defaultProjectName {
		m.status = "The default project cannot be deleted."
		return m, nil
	}

	for _, s := range m.projectSessions(project) {
		m.assignSessionProject(s.Name, defaultProjectName)
	}
	m.projects = removeProject(m.projects, project)
	delete(m.expandedProjects, project)
	if m.selectedProject == project {
		m.selectedProject = defaultProjectName
		m.selectedSession = ""
	}
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.syncSelection()
	m.status = fmt.Sprintf("Deleted project %s.", project)
	return m, nil
}

func (m model) commitMoveProject() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.mode = inputNone
		m.moveQuery = ""
		m.status = "No session selected."
		return m, nil
	}
	project, ok := m.singleMatchingProject()
	if !ok {
		m.status = "No unique project match."
		return m, nil
	}
	m.mode = inputNone
	return m, func() tea.Msg {
		return sessionMovedMsg{session: s.Name, project: project}
	}
}

func (m model) resolveMoveProject() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	project, ok := m.singleMatchingProject()
	if !ok {
		if m.moveQuery == "" {
			m.status = "Type a project prefix."
			return m, nil
		}
		m.status = fmt.Sprintf("Matching projects: %s", strings.Join(m.matchingProjects(m.moveQuery), ", "))
		return m, nil
	}
	m.mode = inputNone
	return m, func() tea.Msg {
		return sessionMovedMsg{session: s.Name, project: project}
	}
}

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
	return defaultProjectName
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
		m.selectedProject = defaultProjectName
	}
	if m.selectedSession != "" {
		if _, ok := m.findSession(m.selectedSession); !ok {
			m.selectedSession = ""
		}
	}
	if m.selectedSession == "" && m.currentSession != "" {
		if _, ok := m.findSession(m.currentSession); ok {
			m.selectedSession = m.currentSession
			m.selectedProject = normalizeProjectName(m.sessionProjects[m.currentSession])
		}
	}
	if m.selectedProject == "" {
		m.selectedProject = defaultProjectName
	}
	if !m.expandedProjects[m.selectedProject] {
		m.expandedProjects[m.selectedProject] = true
	}
}

func (m *model) ensureSessionProjects() bool {
	changed := false
	if m.sessionProjects == nil {
		m.sessionProjects = map[string]string{}
		changed = true
	}
	if m.expandedProjects == nil {
		m.expandedProjects = map[string]bool{}
		changed = true
	}
	for _, s := range m.sessions {
		project := normalizeProjectName(m.sessionProjects[s.Name])
		if project == "" {
			project = defaultProjectName
		}
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
	}
	m.projects = normalizeProjectList(m.projects)
	return changed
}

func (m *model) assignSessionProject(name, project string) {
	project = normalizeProjectName(project)
	if project == "" {
		project = defaultProjectName
	}
	if m.sessionProjects == nil {
		m.sessionProjects = map[string]string{}
	}
	m.sessionProjects[name] = project
	m.addProject(project)
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
	if project == "" {
		project = defaultProjectName
	}
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

func (m model) singleMatchingProject() (string, bool) {
	matches := m.matchingProjects(m.moveQuery)
	if len(matches) == 1 {
		return matches[0], true
	}
	return "", false
}

func (m model) saveState() error {
	state := appState{
		Projects:         append([]string(nil), m.projects...),
		SessionProjects:  map[string]string{},
		ExpandedProjects: map[string]bool{},
	}
	for name, project := range m.sessionProjects {
		state.SessionProjects[name] = normalizeProjectName(project)
	}
	for project, expanded := range m.expandedProjects {
		state.ExpandedProjects[normalizeProjectName(project)] = expanded
	}
	state = normalizeAppState(state)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(m.statePath, data, 0o644)
}

func ensureDefaultStartupState() error {
	path := appStatePath()
	state, err := loadAppState(path)
	if err != nil {
		return err
	}
	state = normalizeAppState(state)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.ExpandedProjects == nil {
		state.ExpandedProjects = map[string]bool{}
	}
	state.SessionProjects[defaultSessionName] = defaultProjectName
	state.ExpandedProjects[defaultProjectName] = true

	data, err := json.MarshalIndent(normalizeAppState(state), "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func loadAppState(path string) (appState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return appState{
				Projects:         []string{defaultProjectName},
				SessionProjects:  map[string]string{},
				ExpandedProjects: map[string]bool{defaultProjectName: true},
			}, nil
		}
		return appState{}, err
	}
	var state appState
	if err := json.Unmarshal(data, &state); err != nil {
		return appState{}, err
	}
	return normalizeAppState(state), nil
}

func normalizeAppState(state appState) appState {
	state.Projects = normalizeProjectList(state.Projects)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.ExpandedProjects == nil {
		state.ExpandedProjects = map[string]bool{}
	}
	for _, project := range state.Projects {
		if _, ok := state.ExpandedProjects[project]; !ok {
			state.ExpandedProjects[project] = true
		}
	}
	for name, project := range state.SessionProjects {
		normalized := normalizeProjectName(project)
		if normalized == "" {
			normalized = defaultProjectName
		}
		state.SessionProjects[name] = normalized
		if !containsString(state.Projects, normalized) {
			state.Projects = append(state.Projects, normalized)
		}
		if _, ok := state.ExpandedProjects[normalized]; !ok {
			state.ExpandedProjects[normalized] = true
		}
	}
	state.Projects = normalizeProjectList(state.Projects)
	return state
}

func normalizeProjectList(projects []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(projects)+1)
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
	add(defaultProjectName)
	for _, project := range projects {
		add(project)
	}
	if len(result) > 1 {
		sort.Strings(result[1:])
	}
	return result
}

func normalizeProjectName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func sanitizeProjectName(name string) string {
	return normalizeProjectName(name)
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

func appStatePath() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "tflow", "state.json")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".config", "tflow", "state.json")
	}
	return filepath.Join(".", ".tflow", "state.json")
}

func (m model) statusView() string {
	style := statusStyle
	if m.err != nil {
		style = errorStatusStyle
	}
	return style.Width(max(20, m.width-6)).Render(m.status)
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
