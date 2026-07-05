package ui

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
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
	inputNew
	inputCreateSession
	inputCreateProject
	inputMoveProject
	inputSetProjectDir
	inputConfirmDelete
	inputRename
	inputCommand
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
	config projectConfig
	err    error
}

type sessionMovedMsg struct {
	session string
	project string
	err     error
}

type sessionRenamedMsg struct {
	oldName string
	newName string
	err     error
}

type projectRenamedMsg struct {
	oldName string
	newName string
	err     error
}

type projectEditedMsg struct {
	oldName string
	config  projectConfig
	err     error
}

type menuActionMsg struct {
	err error
}

type appState struct {
	Projects         []string          `json:"projects"`
	SessionProjects  map[string]string `json:"session_projects"`
	ProjectDirs      map[string]string `json:"project_dirs"`
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
	tmux tmuxController

	width  int
	height int

	mode inputMode

	sessions         []session
	projects         []string
	sessionProjects  map[string]string
	projectConfigs   map[string]projectConfig
	expandedProjects map[string]bool
	selectedProject  string
	selectedSession  string
	currentSession   string
	paneID           string

	input             textinput.Model
	moveQuery         string
	renameRow         treeRow
	deleteRow         treeRow
	createSessionKind sessionKind

	cwd       string
	statePath string

	status string
	err    error
}

type sessionKind int

const (
	sessionKindTerminal sessionKind = iota
	sessionKindK9s
	sessionKindCodex
)

func NewMenu() tea.Model {
	return newModel(newSessionManager(), os.Getenv(menuCurrentEnv), os.Getenv("TMUX_PANE"))
}

func Start() error {
	cwd := defaultSessionDir()
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	manager := newSessionManager()
	if err := prepareStartup(manager, exe, cwd); err != nil {
		return err
	}

	cmd, err := manager.AttachCommand(defaultSessionName)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func OpenMenu() error {
	_, err := tea.NewProgram(NewMenu(), tea.WithAltScreen()).Run()
	return err
}

func prepareStartup(manager tmuxController, binaryPath, cwd string) error {
	if _, err := manager.CreateSession(defaultSessionName, cwd, ""); err != nil {
		return err
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return err
	}
	return ensureDefaultStartupState()
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

func newModel(manager tmuxController, current, paneID string) tea.Model {
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
			ProjectDirs:      map[string]string{},
			ExpandedProjects: map[string]bool{defaultProjectName: true},
		}
	}
	state = normalizeAppState(state)
	projectConfigs, configErr := loadProjectConfigs(statePath, state)
	if configErr != nil {
		projectConfigs = map[string]projectConfig{
			defaultProjectName: defaultProjectConfig(),
		}
		if err == nil {
			err = configErr
		}
	}
	for name := range projectConfigs {
		if !containsString(state.Projects, name) {
			state.Projects = append(state.Projects, name)
		}
	}
	state = normalizeAppState(state)

	return model{
		tmux:             manager,
		mode:             inputNone,
		projects:         state.Projects,
		sessionProjects:  state.SessionProjects,
		projectConfigs:   projectConfigs,
		expandedProjects: state.ExpandedProjects,
		selectedProject:  defaultProjectName,
		currentSession:   current,
		paneID:           paneID,
		input:            input,
		cwd:              cwd,
		statePath:        statePath,
		status:           "",
		err:              err,
	}
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
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
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
		m.err = nil
		m.status = ""
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
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case projectCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		msg.config = normalizeProjectConfig(msg.config)
		m.addProject(msg.config.Name)
		m.setProjectConfig(msg.config)
		m.expandedProjects[msg.config.Name] = true
		m.selectedProject = msg.config.Name
		m.selectedSession = ""
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
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
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		project := normalizeProjectName(m.sessionProjects[msg.oldName])
		if project == "" {
			project = defaultProjectName
		}
		delete(m.sessionProjects, msg.oldName)
		m.sessionProjects[msg.newName] = project
		if m.selectedSession == msg.oldName {
			m.selectedSession = msg.newName
		}
		if m.currentSession == msg.oldName {
			m.currentSession = msg.newName
		}
		m.mode = inputNone
		m.renameRow = treeRow{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case projectRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		if msg.oldName == defaultProjectName {
			m.err = fmt.Errorf("cannot rename default project")
			m.status = "The default project cannot be renamed."
			return m, nil
		}
		for name, project := range m.sessionProjects {
			if normalizeProjectName(project) == msg.oldName {
				m.sessionProjects[name] = msg.newName
			}
		}
		cfg := m.projectConfig(msg.oldName)
		delete(m.projectConfigs, msg.oldName)
		cfg.Name = msg.newName
		m.setProjectConfig(cfg)
		m.projects = replaceProject(m.projects, msg.oldName, msg.newName)
		expanded := m.expandedProjects[msg.oldName]
		delete(m.expandedProjects, msg.oldName)
		m.expandedProjects[msg.newName] = expanded
		if err := removeProjectConfigFile(m.statePath, msg.oldName); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		if m.selectedProject == msg.oldName {
			m.selectedProject = msg.newName
		}
		m.mode = inputNone
		m.renameRow = treeRow{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.syncSelection()
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, nil
	case projectEditedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		return m.applyProjectEdit(msg.oldName, msg.config)
	case menuActionMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, tea.Quit
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
		return m, m.closeMenuCmd()
	case tea.KeyEsc:
		return m, m.closeMenuCmd()
	}

	switch msg.String() {
	case "q":
		return m, m.closeMenuCmd()
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
	case ":":
		m.mode = inputCommand
		m.input.Prompt = ":"
		m.input.SetValue("")
		m.input.Focus()
		m.err = nil
		m.status = ""
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
		return m.switchSelectedSession()
	case "n":
		m.mode = inputNew
		m.status = "New: np project, nt terminal, nk k9s, nc codex."
		return m, nil
	case "c":
		m.mode = inputSetProjectDir
		m.input.Prompt = "dir: "
		m.input.SetValue(m.projectDir(m.contextProject()))
		m.input.Focus()
		m.status = fmt.Sprintf("Set the default directory for %s.", m.contextProject())
		return m, nil
	case "m":
		if _, ok := m.selectedSessionInfo(); !ok {
			m.status = "Select a session to move."
			return m, nil
		}
		m.mode = inputMoveProject
		m.moveQuery = ""
		m.status = "Move: type project prefix."
		return m, nil
	case "d":
		return m.beginDelete()
	case "r":
		return m.beginRename()
	case "e":
		return m.editProject()
	}

	return m, nil
}

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case inputNew:
		return m.updateNew(msg)
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
			command, err := m.sessionStartupCommand(m.createSessionKind)
			if err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			return m, func() tea.Msg {
				s, err := m.tmux.CreateSession(name, m.createSessionDir(), command)
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
			if containsString(m.projects, name) {
				m.status = "Project already exists."
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			return m, func() tea.Msg {
				return projectCreatedMsg{config: projectConfig{Name: name}}
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
			m.updateMoveStatus()
			return m, nil
		case tea.KeyEnter:
			return m.commitMoveProject()
		}
		if len(msg.String()) == 1 {
			r := []rune(msg.String())[0]
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				m.moveQuery += strings.ToLower(string(r))
				if project, ok := m.singleMatchingProject(); ok {
					return m, func() tea.Msg {
						s, _ := m.selectedSessionInfo()
						return sessionMovedMsg{session: s.Name, project: project}
					}
				}
				m.updateMoveStatus()
				return m, nil
			}
		}
	case inputSetProjectDir:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Directory update cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitProjectDir()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputConfirmDelete:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.deleteRow = treeRow{}
			m.status = "Delete cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmDelete()
		}
		switch msg.String() {
		case "d", "y":
			return m.confirmDelete()
		}
	case inputCommand:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.input.SetValue("")
			m.err = nil
			m.status = ""
			return m, nil
		case tea.KeyEnter:
			command := strings.TrimSpace(strings.ToLower(m.input.Value()))
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.input.SetValue("")
			return m.executeCommand(command)
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputRename:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.renameRow = treeRow{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Rename cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitRename()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	}
	return m, nil
}

func (m model) updateNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = inputNone
		m.status = "New cancelled."
		return m, nil
	}

	switch msg.String() {
	case "p":
		m.mode = inputCreateProject
		m.input.Prompt = "project: "
		m.input.SetValue("")
		m.input.Focus()
		m.status = "Create a new project."
		return m, nil
	case "t":
		return m.startSessionCreate(sessionKindTerminal)
	case "k":
		return m.startSessionCreate(sessionKindK9s)
	case "c":
		return m.startSessionCreate(sessionKindCodex)
	default:
		m.status = "New: use np, nt, nk, or nc."
		return m, nil
	}
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case inputNew:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	case inputCreateSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Session"))
	case inputCreateProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Project"))
	case inputMoveProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	case inputSetProjectDir:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderProjectDirOverlay())
	case inputConfirmDelete:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderDeleteOverlay())
	case inputRename:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderRenameOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	innerWidth := max(28, m.width-4)
	header := m.renderHeader(innerWidth)
	tree := m.renderTreePanel(innerWidth)
	footer := m.renderFooter(innerWidth)
	return lipgloss.JoinVertical(lipgloss.Left, header, tree, footer)
}

func (m model) renderRow(index int, row treeRow) string {
	selected := index == m.selectedRowIndex()
	switch row.kind {
	case rowProject:
		return m.renderProjectRow(row, selected)
	case rowSession:
		return m.renderSessionRow(row, selected)
	default:
		return ""
	}
}

func (m model) rowLabel(row treeRow) string {
	switch row.kind {
	case rowProject:
		prefix := "v"
		if !m.expandedProjects[row.project] {
			prefix = ">"
		}
		return fmt.Sprintf("%s %s", prefix, row.project)
	case rowSession:
		marker := "  "
		if row.session == m.currentSession {
			marker = "* "
		}
		return strings.Repeat(" ", row.depth*2) + marker + row.session
	default:
		return ""
	}
}

func (m model) renderHeader(width int) string {
	left := brandBadgeStyle.Render("TFLOW")
	right := mutedStyle.Render(fmt.Sprintf("%d projects  %d sections", len(m.projects), len(m.sessions)))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return headerStyle.Width(width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) renderTreePanel(width int) string {
	lines := []string{
		sectionTitleStyle.Render("Projects"),
	}

	rows := m.treeRows()
	if len(rows) == 0 {
		lines = append(lines, "", mutedStyle.Render("No projects or sessions"))
	} else {
		lines = append(lines, "")
		for index, row := range rows {
			lines = append(lines, m.renderRow(index, row))
		}
	}

	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderProjectRow(row treeRow, selected bool) string {
	toggle := "[-]"
	if !m.expandedProjects[row.project] {
		toggle = "[+]"
	}
	return m.renderLabeledRow(fmt.Sprintf("%s %s", toggle, m.projectLabel(row.project)), "", selected, true, row.project)
}

func (m model) renderSessionRow(row treeRow, selected bool) string {
	content := "  " + row.session
	if row.session == m.currentSession {
		badge := "[live]"
		if !selected {
			badge = currentBadgeStyle.Render(badge)
		}
		content = "  " + badge + " " + row.session
	}
	return m.renderInlineRow(content, selected, false, row.project)
}

func (m model) renderLabeledRow(left, right string, selected, project bool, projectName string) string {
	style := m.rowStyle(selected, project, projectName)

	contentWidth := max(16, m.width-12)
	gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap)
	if right != "" {
		content += right
	}
	return style.Width(contentWidth).Render(content)
}

func (m model) renderInlineRow(content string, selected, project bool, projectName string) string {
	style := m.rowStyle(selected, project, projectName)
	return style.Width(max(16, m.width-12)).Render(content)
}

func (m model) renderFooter(width int) string {
	lines := []string{}
	if m.mode == inputNew {
		lines = append(lines, hintStyle.Render("new: [p] project  [t] terminal  [k] k9s  [c] codex  [esc] cancel"))
	}
	if m.mode == inputMoveProject {
		lines = append(lines, hintStyle.Render("move: type a project prefix and the matching initials stay highlighted"))
	}
	if m.mode == inputCommand {
		lines = append(lines, inputStyle.Render(m.input.View()), hintStyle.Render("Enter runs. Esc cancels."))
	}
	if status := m.statusView(); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}
	if len(lines) == 0 {
		return ""
	}
	return footerStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
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

func (m model) renderDeleteOverlay() string {
	target := "selection"
	switch m.deleteRow.kind {
	case rowProject:
		target = "project " + m.deleteRow.project
	case rowSession:
		target = "section " + m.deleteRow.session
	}
	lines := []string{
		titleStyle.Render("Confirm Delete"),
		mutedStyle.Render("Delete " + target + "?"),
		"",
		hintStyle.Render("Enter, d, or y confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(42, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderMoveOverlay() string {
	lines := []string{
		titleStyle.Render("Move Session"),
		mutedStyle.Render("prefix: " + fallbackText(m.moveQuery, "type letters")),
		"",
	}
	for _, project := range m.matchingProjects(m.moveQuery) {
		lines = append(lines, sessionStyle.Render("  "+project))
	}
	if len(m.matchingProjects(m.moveQuery)) == 0 {
		lines = append(lines, mutedStyle.Render("No matching project."))
	}
	lines = append(lines, "", hintStyle.Render("Type until one project matches. Enter confirms."))
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderProjectDirOverlay() string {
	lines := []string{
		titleStyle.Render("Project Directory"),
		mutedStyle.Render("project: " + m.contextProject()),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(44, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderRenameOverlay() string {
	title := "Rename"
	current := ""
	switch m.renameRow.kind {
	case rowProject:
		title = "Rename Project"
		current = m.renameRow.project
	case rowSession:
		title = "Rename Section"
		current = m.renameRow.session
	}
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("current: " + fallbackText(current, "none")),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.tmux.SwitchClient(s.Name)
		if err == nil {
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
	}
}

func (m model) killSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return sessionKilledMsg{name: s.Name, err: m.tmux.KillSession(s.Name)}
	}
}

func (m *model) beginDelete() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		m.status = "Select a project or section to delete."
		return m, nil
	}
	if row.kind == rowProject && m.projectConfig(row.project).Protect {
		project := row.project
		m.status = fmt.Sprintf("Project %s is protected.", project)
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteRow = row
	switch row.kind {
	case rowProject:
		m.status = fmt.Sprintf("Confirm delete for project %s.", row.project)
	case rowSession:
		m.status = fmt.Sprintf("Confirm delete for section %s.", row.session)
	}
	return m, nil
}

func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	row := m.deleteRow
	m.mode = inputNone
	m.deleteRow = treeRow{}
	if row.kind == rowSession {
		return m.killSelectedSession()
	}
	return m.deleteSelectedProject()
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.ClosePane(m.paneID)}
	}
}

func (m model) quitAllCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.QuitAll(m.paneID)}
	}
}

func (m *model) beginRename() (tea.Model, tea.Cmd) {
	row, ok := m.selectedRow()
	if !ok {
		m.status = "Select a project or section to rename."
		return m, nil
	}
	if row.kind == rowProject && normalizeProjectName(row.project) == defaultProjectName {
		m.status = "The default project cannot be renamed."
		return m, nil
	}
	m.mode = inputRename
	m.renameRow = row
	m.input.SetValue(m.renameValue(row))
	m.input.CursorEnd()
	if row.kind == rowProject {
		m.input.Prompt = "project: "
		m.status = "Rename the selected project."
	} else {
		m.input.Prompt = "section: "
		m.status = "Rename the selected section."
	}
	m.input.Focus()
	return m, nil
}

func (m *model) commitRename() (tea.Model, tea.Cmd) {
	row := m.renameRow
	switch row.kind {
	case rowProject:
		name := sanitizeProjectName(m.input.Value())
		if name == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if name == row.project {
			m.mode = inputNone
			m.renameRow = treeRow{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if containsString(m.projects, name) {
			m.status = "Project already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameRow = treeRow{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return projectRenamedMsg{oldName: row.project, newName: name}
		}
	case rowSession:
		name := sanitizeSessionName(m.input.Value())
		if name == "" {
			m.status = "Section name is empty."
			return m, nil
		}
		if name == row.session {
			m.mode = inputNone
			m.renameRow = treeRow{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if _, ok := m.findSession(name); ok {
			m.status = "Section already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameRow = treeRow{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return sessionRenamedMsg{oldName: row.session, newName: name, err: m.tmux.RenameSession(row.session, name)}
		}
	default:
		m.status = "Select a project or section to rename."
		return m, nil
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
	delete(m.projectConfigs, project)
	delete(m.expandedProjects, project)
	if m.selectedProject == project {
		m.selectedProject = defaultProjectName
		m.selectedSession = ""
	}
	if err := removeProjectConfigFile(m.statePath, project); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.syncSelection()
	if err := m.syncTmuxSessionProjects(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
	return m, nil
}

func (m *model) commitProjectDir() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	dir := strings.TrimSpace(m.input.Value())
	cfg := m.projectConfig(project)
	m.mode = inputNone
	m.input.Blur()
	m.input.Prompt = ""
	if dir == "" {
		cfg.Workdir = ""
	} else {
		cfg.Workdir = normalizeCWD(dir)
	}
	m.setProjectConfig(cfg)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
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
	if _, ok := m.projectConfigs[project]; !ok {
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
		ProjectDirs:      map[string]string{},
		ExpandedProjects: map[string]bool{},
	}
	for name, project := range m.sessionProjects {
		state.SessionProjects[name] = normalizeProjectName(project)
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
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(m.statePath, data, 0o644); err != nil {
		return err
	}
	return saveProjectConfigs(m.statePath, m.projectConfigs)
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
				ProjectDirs:      map[string]string{},
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
	if state.ProjectDirs == nil {
		state.ProjectDirs = map[string]string{}
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
	for project, dir := range state.ProjectDirs {
		normalized := normalizeProjectName(project)
		if normalized == "" || strings.TrimSpace(dir) == "" {
			delete(state.ProjectDirs, project)
			continue
		}
		delete(state.ProjectDirs, project)
		state.ProjectDirs[normalized] = normalizeCWD(dir)
		if !containsString(state.Projects, normalized) {
			state.Projects = append(state.Projects, normalized)
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
	if project == "" {
		project = defaultProjectName
	}
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
		if project == "" {
			project = defaultProjectName
		}
		sessionProjects[s.Name] = project
	}
	return m.tmux.SyncSessionProjects(sessionProjects)
}

func (m model) projectConfig(project string) projectConfig {
	project = normalizeProjectName(project)
	if project == "" {
		project = defaultProjectName
	}
	if cfg, ok := m.projectConfigs[project]; ok {
		return normalizeProjectConfig(cfg)
	}
	return projectConfig{Name: project}
}

func (m *model) setProjectConfig(cfg projectConfig) {
	cfg = normalizeProjectConfig(cfg)
	if cfg.Name == "" {
		return
	}
	if m.projectConfigs == nil {
		m.projectConfigs = map[string]projectConfig{}
	}
	m.projectConfigs[cfg.Name] = cfg
}

func (m model) projectDir(project string) string {
	return strings.TrimSpace(m.projectConfig(project).Workdir)
}

func (m model) createSessionDir() string {
	if dir := m.projectDir(m.contextProject()); dir != "" {
		return dir
	}
	return m.cwd
}

func (m *model) startSessionCreate(kind sessionKind) (tea.Model, tea.Cmd) {
	m.mode = inputCreateSession
	m.createSessionKind = kind
	m.input.Prompt = "section: "
	switch kind {
	case sessionKindK9s:
		m.input.SetValue("k9s")
		m.status = fmt.Sprintf("Create a new k9s section in %s.", m.contextProject())
	case sessionKindCodex:
		m.input.SetValue("codex")
		m.status = fmt.Sprintf("Create a new codex section in %s.", m.contextProject())
	default:
		m.input.SetValue("")
		m.status = fmt.Sprintf("Create a new terminal section in %s.", m.contextProject())
	}
	m.input.Focus()
	m.input.CursorEnd()
	return m, nil
}

func (m model) sessionStartupCommand(kind sessionKind) (string, error) {
	cfg := m.projectConfig(m.contextProject())
	switch kind {
	case sessionKindTerminal:
		return "", nil
	case sessionKindK9s:
		if cfg.Cluster.Path != "" {
			return "export KUBECONFIG=" + shellQuote(cfg.Cluster.Path) + "; exec k9s", nil
		}
		if cfg.Cluster.ConnectionCmd != "" {
			return cfg.Cluster.ConnectionCmd + " && exec k9s", nil
		}
		return "", fmt.Errorf("project %s has no cluster configured", cfg.Name)
	case sessionKindCodex:
		return "exec codex", nil
	default:
		return "", nil
	}
}

func (m *model) updateMoveStatus() {
	matches := m.matchingProjects(m.moveQuery)
	switch {
	case m.moveQuery == "":
		m.status = "Move: type project prefix."
	case len(matches) == 0:
		m.status = "Move: no matching project."
	default:
		m.status = "Move matches: " + strings.Join(matches, ", ")
	}
}

func (m model) projectLabel(project string) string {
	project = normalizeProjectName(project)
	if m.mode != inputMoveProject {
		return project
	}
	if m.moveQuery == "" {
		return project
	}
	if !strings.HasPrefix(project, m.moveQuery) {
		return project
	}
	prefix := lipgloss.NewStyle().Bold(true).Foreground(yellowColor).Render(project[:len(m.moveQuery)])
	return prefix + project[len(m.moveQuery):]
}

func (m model) editProject() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	cfg := m.projectConfig(project)
	path, err := writeProjectEditFile(cfg)
	if err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	cmd, err := projectEditorCommand(path)
	if err != nil {
		_ = os.Remove(path)
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(path)
		if err != nil {
			return projectEditedMsg{oldName: project, err: err}
		}
		edited, readErr := os.ReadFile(path)
		if readErr != nil {
			return projectEditedMsg{oldName: project, err: readErr}
		}
		parsed, parseErr := parseProjectConfig(edited)
		if parseErr != nil {
			return projectEditedMsg{oldName: project, err: parseErr}
		}
		return projectEditedMsg{oldName: project, config: parsed}
	})
}

func writeProjectEditFile(cfg projectConfig) (string, error) {
	file, err := os.CreateTemp("", "tflow-project-*.yaml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.Write(marshalProjectConfig(cfg)); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func projectEditorCommand(path string) (*exec.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	args := strings.Fields(editor)
	if len(args) == 0 {
		return nil, fmt.Errorf("EDITOR is empty")
	}
	args = append(args, path)
	return exec.Command(args[0], args[1:]...), nil
}

func (m model) applyProjectEdit(oldName string, cfg projectConfig) (tea.Model, tea.Cmd) {
	cfg = normalizeProjectConfig(cfg)
	if cfg.Name == "" {
		m.err = fmt.Errorf("project name is empty")
		m.status = "Project name is empty."
		return m, nil
	}
	if oldName == defaultProjectName && cfg.Name != defaultProjectName {
		m.err = fmt.Errorf("cannot rename default project")
		m.status = "The default project cannot be renamed."
		return m, nil
	}
	if oldName != cfg.Name && containsString(m.projects, cfg.Name) {
		m.err = fmt.Errorf("project already exists")
		m.status = "Project already exists."
		return m, nil
	}
	if oldName != cfg.Name {
		for name, project := range m.sessionProjects {
			if normalizeProjectName(project) == oldName {
				m.sessionProjects[name] = cfg.Name
			}
		}
		m.projects = replaceProject(m.projects, oldName, cfg.Name)
		delete(m.projectConfigs, oldName)
		delete(m.expandedProjects, oldName)
		m.expandedProjects[cfg.Name] = true
		if m.selectedProject == oldName {
			m.selectedProject = cfg.Name
		}
		if err := removeProjectConfigFile(m.statePath, oldName); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
	}
	m.setProjectConfig(cfg)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if err := m.syncTmuxSessionProjects(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
	return m, nil
}

func (m model) executeCommand(command string) (tea.Model, tea.Cmd) {
	switch command {
	case "", ":":
		m.err = nil
		m.status = ""
		return m, nil
	case "q", "q!":
		return m, m.closeMenuCmd()
	case "qa", "qa!":
		return m, m.quitAllCmd()
	default:
		m.err = fmt.Errorf("unknown command")
		m.status = fmt.Sprintf("Unknown command: %s", command)
		return m, nil
	}
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
