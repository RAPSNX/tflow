package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultSessionName = "code"
)

var bootstrapProjectAnimals = []string{
	"otter",
	"fox",
	"lynx",
	"koala",
	"badger",
	"panda",
	"falcon",
	"gecko",
	"orca",
	"wombat",
}

type inputMode int

const (
	inputNone inputMode = iota
	inputCreateSession
	inputCreateProject
	inputRenameProject
	inputRenameSession
	inputConfirmTerminate
	inputProjectOverlay
	inputMoveSession
)

type overlayAction int

const (
	overlaySwitchProject overlayAction = iota
	overlayMoveSession
)

type sessionType string

const (
	sessionTypeTerminal sessionType = "terminal"
	sessionTypeAgent    sessionType = "agent"
)

type appState struct {
	CurrentProject string         `json:"current_project"`
	Projects       []projectState `json:"projects"`
}

type projectState struct {
	Name     string         `json:"name"`
	Sessions []sessionState `json:"sessions"`
}

type sessionState struct {
	Name     string      `json:"name"`
	TmuxName string      `json:"tmux_name"`
	Type     sessionType `json:"type"`
}

type legacyAppState struct {
	Projects         []string          `json:"projects"`
	SessionProjects  map[string]string `json:"session_projects"`
	SessionTypes     map[string]string `json:"session_types"`
	ProjectDirs      map[string]string `json:"project_dirs"`
	ExpandedProjects map[string]bool   `json:"expanded_projects"`
}

type sessionsLoadedMsg struct {
	sessions []session
	err      error
}

type sessionCreatedMsg struct {
	session sessionState
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

type projectCreatedMsg struct {
	name string
	err  error
}

type sessionMovedMsg struct {
	targetProject string
	err           error
}

type projectTerminatedMsg struct {
	project string
	err     error
}

type menuActionMsg struct {
	err error
}

type overlayTarget struct {
	Project string
	Hint    string
}

type overlayState struct {
	Action  overlayAction
	Targets []overlayTarget
	Query   string
}

type model struct {
	tmux tmuxController

	width  int
	height int
	paneID string
	cwd    string

	state      appState
	statePath  string
	projectCfg map[string]projectConfig

	currentTmuxSession string
	selectedSession    string
	tmuxSessions       map[string]session

	mode    inputMode
	overlay overlayState
	input   textinput.Model

	createSessionType sessionType
	status            string
	err               error
}

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
	sessionName, err := prepareStartup(manager, exe, cwd)
	if err != nil {
		return err
	}

	cmd, err := manager.AttachCommand(sessionName)
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

func prepareStartup(manager tmuxController, binaryPath, cwd string) (string, error) {
	if err := saveDefaultAppConfig(); err != nil {
		return "", err
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return "", err
	}

	statePath := appStatePath()
	state, err := loadAppState(statePath)
	if err != nil {
		return "", err
	}
	cfgs, err := loadProjectConfigs(statePath, state)
	if err != nil {
		return "", err
	}

	if len(state.Projects) == 0 {
		project := nextBootstrapProjectName(state)
		cfg := normalizeProjectConfig(projectConfig{Name: project})
		cfgs[project] = cfg
		state.CurrentProject = project
		state.Projects = []projectState{{
			Name: project,
			Sessions: []sessionState{{
				Name:     defaultSessionName,
				TmuxName: tmuxSessionName(project, defaultSessionName),
				Type:     sessionTypeTerminal,
			}},
		}}
	}

	state = normalizeAppState(state)
	targetProject := findProjectState(state, state.CurrentProject)
	if targetProject == nil && len(state.Projects) > 0 {
		targetProject = &state.Projects[0]
		state.CurrentProject = targetProject.Name
	}
	if targetProject == nil || len(targetProject.Sessions) == 0 {
		if targetProject == nil {
			return "", fmt.Errorf("no project available")
		}
		targetProject.Sessions = append(targetProject.Sessions, sessionState{
			Name:     defaultSessionName,
			TmuxName: tmuxSessionName(targetProject.Name, defaultSessionName),
			Type:     sessionTypeTerminal,
		})
	}

	startupSession := targetProject.Sessions[0]
	projectCfg := cfgs[targetProject.Name]
	projectCfg.Name = targetProject.Name
	cfgs[targetProject.Name] = normalizeProjectConfig(projectCfg)
	command, err := startupCommandForSession(projectCfg, startupSession.Type)
	if err != nil {
		return "", err
	}
	if _, err := manager.CreateSession(startupSession.TmuxName, projectWorkdir(projectCfg, cwd), command); err != nil {
		return "", err
	}
	if err := saveAppState(statePath, state, cfgs); err != nil {
		return "", err
	}
	if err := syncTmuxState(manager, state); err != nil {
		return "", err
	}
	return startupSession.TmuxName, nil
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

func newModel(manager tmuxController, currentTmuxSession, paneID string) tea.Model {
	cwd, _ := os.Getwd()

	input := textinput.New()
	input.CharLimit = 40
	input.Width = 28
	input.Blur()

	statePath := appStatePath()
	cfg, cfgErr := loadAppConfigForStatePath(statePath)
	if cfgErr == nil {
		applyTheme(themeFromConfig(cfg))
	}
	state, err := loadAppState(statePath)
	if err != nil {
		state = appState{}
	}
	state = normalizeAppState(state)
	projectCfg, cfgErr2 := loadProjectConfigs(statePath, state)
	if cfgErr2 != nil {
		projectCfg = map[string]projectConfig{}
		if err == nil {
			err = cfgErr2
		}
	}
	if err == nil {
		err = cfgErr
	}

	m := model{
		tmux:               manager,
		paneID:             paneID,
		cwd:                cwd,
		state:              state,
		statePath:          statePath,
		projectCfg:         projectCfg,
		currentTmuxSession: strings.TrimSpace(currentTmuxSession),
		tmuxSessions:       map[string]session{},
		input:              input,
		err:                err,
	}
	m.syncCurrentProjectFromSession()
	m.syncSelection()
	return m
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
		m.err = msg.err
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.tmuxSessions = map[string]session{}
		for _, item := range msg.sessions {
			m.tmuxSessions[item.Name] = item
		}
		changed := m.pruneMissingSessions()
		m.syncCurrentProjectFromSession()
		m.syncSelection()
		if changed {
			if err := m.saveState(); err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
		}
		if err := m.syncTmuxState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = ""
		m.err = nil
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		project := m.currentProjectName()
		m.addSession(project, msg.session)
		m.selectedSession = msg.session.Name
		m.status = ""
		m.err = nil
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.loadSessionsCmd()
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.renameSelectedSessionState(msg.oldName, msg.newName)
		m.selectedSession = msg.newName
		if m.currentStateSession() != nil && m.currentStateSession().Name == msg.oldName {
			m.currentTmuxSession = tmuxSessionName(m.currentProjectName(), msg.newName)
		}
		m.mode = inputNone
		m.input.Blur()
		m.input.Prompt = ""
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.loadSessionsCmd()
	case projectCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.addProject(msg.name)
		m.state.CurrentProject = msg.name
		m.selectedSession = ""
		m.mode = inputNone
		m.input.Blur()
		m.input.Prompt = ""
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, nil
	case projectRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.applyProjectRename(msg.oldName, msg.newName)
		m.mode = inputNone
		m.input.Blur()
		m.input.Prompt = ""
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.loadSessionsCmd()
	case sessionMovedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.applyMoveSelectedSession(msg.targetProject)
		m.mode = inputNone
		m.overlay = overlayState{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.loadSessionsCmd()
	case projectTerminatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.removeProject(msg.project)
		m.mode = inputNone
		m.overlay = overlayState{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.loadSessionsCmd()
	case menuActionMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
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
	case tea.KeyCtrlQ:
		if m.currentProjectName() == "" {
			m.status = "No project selected."
			return m, nil
		}
		m.mode = inputConfirmTerminate
		m.status = fmt.Sprintf("Confirm termination for project %s.", m.currentProjectName())
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		m.shiftSelection(1)
		return m, nil
	case "k", "up":
		m.shiftSelection(-1)
		return m, nil
	case "enter":
		return m.switchSelectedSession()
	case "t":
		return m.startSessionCreate(sessionTypeTerminal)
	case "a":
		return m.startSessionCreate(sessionTypeAgent)
	case "r":
		if m.selectedSessionState() == nil {
			m.status = "Select a session to rename."
			return m, nil
		}
		m.mode = inputRenameSession
		m.input.Prompt = "session: "
		m.input.SetValue(m.selectedSession)
		m.input.Focus()
		m.input.CursorEnd()
		m.status = "Rename the selected session."
		return m, nil
	case "R":
		if m.currentProjectName() == "" {
			m.status = "No project selected."
			return m, nil
		}
		m.mode = inputRenameProject
		m.input.Prompt = "project: "
		m.input.SetValue(m.currentProjectName())
		m.input.Focus()
		m.input.CursorEnd()
		m.status = "Rename the current project."
		return m, nil
	case "P":
		m.openProjectOverlay(overlaySwitchProject)
		return m, nil
	case "m":
		if m.selectedSessionState() == nil {
			m.status = "Select a session to move."
			return m, nil
		}
		m.openProjectOverlay(overlayMoveSession)
		return m, nil
	}

	return m, nil
}

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case inputCreateSession:
		return m.updateCreateSession(msg)
	case inputCreateProject:
		return m.updateCreateProject(msg)
	case inputRenameProject:
		return m.updateRenameProject(msg)
	case inputRenameSession:
		return m.updateRenameSession(msg)
	case inputConfirmTerminate:
		return m.updateConfirmTerminate(msg)
	case inputProjectOverlay, inputMoveSession:
		return m.updateOverlay(msg)
	default:
		return m, nil
	}
}

func (m model) updateCreateSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetInput("Session creation cancelled.")
		return m, nil
	case tea.KeyEnter:
		project := m.currentProjectName()
		if project == "" {
			m.status = "Create a project first."
			return m, nil
		}
		name := sanitizeSessionName(m.input.Value())
		if name == "" {
			m.status = "Session name is empty."
			return m, nil
		}
		if m.findSession(project, name) != nil {
			m.status = "Session already exists in this project."
			return m, nil
		}
		cfg := m.projectConfig(project)
		command, err := startupCommandForSession(cfg, m.createSessionType)
		if err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		tmuxName := tmuxSessionName(project, name)
		cwd := projectWorkdir(cfg, m.cwd)
		m.resetInput("")
		return m, func() tea.Msg {
			_, err := m.tmux.CreateSession(tmuxName, cwd, command)
			if err != nil {
				return sessionCreatedMsg{err: err}
			}
			return sessionCreatedMsg{session: sessionState{Name: name, TmuxName: tmuxName, Type: m.createSessionType}}
		}
	}
	next, cmd := m.input.Update(msg)
	m.input = next
	return m, cmd
}

func (m model) updateCreateProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetInput("Project creation cancelled.")
		m.restoreOverlayMode()
		return m, nil
	case tea.KeyEnter:
		name := sanitizeProjectName(m.input.Value())
		if name == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if m.findProject(name) != nil {
			m.status = "Project already exists."
			return m, nil
		}
		m.resetInput("")
		return m, func() tea.Msg {
			return projectCreatedMsg{name: name}
		}
	}
	next, cmd := m.input.Update(msg)
	m.input = next
	return m, cmd
}

func (m model) updateRenameProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetInput("Rename cancelled.")
		return m, nil
	case tea.KeyEnter:
		oldName := m.currentProjectName()
		newName := sanitizeProjectName(m.input.Value())
		if oldName == "" {
			m.status = "No project selected."
			return m, nil
		}
		if newName == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if oldName == newName {
			m.resetInput("")
			return m, nil
		}
		if m.findProject(newName) != nil {
			m.status = "Project already exists."
			return m, nil
		}
		project := m.findProject(oldName)
		if project == nil {
			m.status = "Project not found."
			return m, nil
		}
		renames := make([][2]string, 0, len(project.Sessions))
		for _, session := range project.Sessions {
			renames = append(renames, [2]string{session.TmuxName, tmuxSessionName(newName, session.Name)})
		}
		m.resetInput("")
		return m, func() tea.Msg {
			for _, rename := range renames {
				if err := m.tmux.RenameSession(rename[0], rename[1]); err != nil {
					return projectRenamedMsg{oldName: oldName, newName: newName, err: err}
				}
			}
			return projectRenamedMsg{oldName: oldName, newName: newName}
		}
	}
	next, cmd := m.input.Update(msg)
	m.input = next
	return m, cmd
}

func (m model) updateRenameSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetInput("Rename cancelled.")
		return m, nil
	case tea.KeyEnter:
		current := m.selectedSessionState()
		if current == nil {
			m.status = "Select a session to rename."
			return m, nil
		}
		newName := sanitizeSessionName(m.input.Value())
		if newName == "" {
			m.status = "Session name is empty."
			return m, nil
		}
		if current.Name == newName {
			m.resetInput("")
			return m, nil
		}
		if m.findSession(m.currentProjectName(), newName) != nil {
			m.status = "Session already exists in this project."
			return m, nil
		}
		newTmuxName := tmuxSessionName(m.currentProjectName(), newName)
		oldName := current.Name
		oldTmuxName := current.TmuxName
		m.resetInput("")
		return m, func() tea.Msg {
			return sessionRenamedMsg{oldName: oldName, newName: newName, err: m.tmux.RenameSession(oldTmuxName, newTmuxName)}
		}
	}
	next, cmd := m.input.Update(msg)
	m.input = next
	return m, cmd
}

func (m model) updateConfirmTerminate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = inputNone
		m.status = "Termination cancelled."
		return m, nil
	case tea.KeyEnter:
		return m.terminateCurrentProject()
	}
	if msg.String() == "y" {
		return m.terminateCurrentProject()
	}
	return m, nil
}

func (m model) updateOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = inputNone
		m.overlay = overlayState{}
		m.status = "Overlay closed."
		return m, nil
	case tea.KeyBackspace, tea.KeyDelete:
		if m.overlay.Query != "" {
			m.overlay.Query = m.overlay.Query[:len(m.overlay.Query)-1]
		}
		m.updateOverlayStatus()
		return m, nil
	}

	switch msg.String() {
	case "n":
		if m.mode == inputProjectOverlay && m.overlay.Action == overlaySwitchProject {
			m.mode = inputCreateProject
			m.input.Prompt = "project: "
			m.input.SetValue("")
			m.input.Focus()
			m.status = "Create a new project."
		}
		return m, nil
	case "enter":
		return m.commitOverlaySelection()
	}

	if len(msg.String()) == 1 {
		r := []rune(msg.String())[0]
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			m.overlay.Query += strings.ToLower(string(r))
			if target, ok := m.exactOverlayMatch(); ok {
				return m.applyOverlaySelection(target.Project)
			}
			m.updateOverlayStatus()
		}
	}
	return m, nil
}

func (m *model) restoreOverlayMode() {
	if m.overlay.Action == overlaySwitchProject {
		m.mode = inputProjectOverlay
		return
	}
	m.mode = inputMoveSession
}

func (m *model) resetInput(status string) {
	m.mode = inputNone
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	m.status = status
}

func (m *model) openProjectOverlay(action overlayAction) {
	m.overlay = overlayState{
		Action:  action,
		Targets: buildOverlayTargets(m.overlayProjects(action)),
	}
	if action == overlayMoveSession {
		m.mode = inputMoveSession
		m.status = "Move session: type a project hint."
		return
	}
	m.mode = inputProjectOverlay
	if len(m.overlay.Targets) == 0 {
		m.status = "No other projects. Press n to create one."
	} else {
		m.status = "Project overlay: type a project hint or press n to create one."
	}
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	session := m.selectedSessionState()
	if session == nil {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		err := m.tmux.SwitchClient(session.TmuxName)
		if err == nil {
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
	}
}

func (m model) terminateCurrentProject() (tea.Model, tea.Cmd) {
	project := m.currentProjectName()
	if project == "" {
		m.status = "No project selected."
		return m, nil
	}
	sessions := append([]sessionState(nil), m.projectSessions(project)...)
	return m, func() tea.Msg {
		for _, session := range sessions {
			if err := m.tmux.KillSession(session.TmuxName); err != nil {
				return projectTerminatedMsg{project: project, err: err}
			}
		}
		return projectTerminatedMsg{project: project}
	}
}

func (m model) applyOverlaySelection(project string) (tea.Model, tea.Cmd) {
	if m.overlay.Action == overlayMoveSession {
		return m.moveSelectedSession(project)
	}
	return m.switchProject(project)
}

func (m model) commitOverlaySelection() (tea.Model, tea.Cmd) {
	target, ok := m.exactOverlayMatch()
	if !ok {
		m.status = "No matching hint."
		return m, nil
	}
	return m.applyOverlaySelection(target.Project)
}

func (m model) exactOverlayMatch() (overlayTarget, bool) {
	for _, target := range m.overlay.Targets {
		if target.Hint == m.overlay.Query {
			return target, true
		}
	}
	return overlayTarget{}, false
}

func (m model) switchProject(project string) (tea.Model, tea.Cmd) {
	projectState := m.findProject(project)
	if projectState == nil {
		m.status = "Project not found."
		return m, nil
	}
	m.state.CurrentProject = project
	m.selectedSession = ""
	m.syncSelection()
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if len(projectState.Sessions) == 0 {
		m.mode = inputNone
		m.overlay = overlayState{}
		m.status = "Switched to empty project."
		return m, nil
	}
	target := projectState.Sessions[0].TmuxName
	return m, func() tea.Msg {
		err := m.tmux.SwitchClient(target)
		if err == nil {
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
	}
}

func (m model) moveSelectedSession(targetProject string) (tea.Model, tea.Cmd) {
	session := m.selectedSessionState()
	if session == nil {
		m.status = "No session selected."
		return m, nil
	}
	if m.findSession(targetProject, session.Name) != nil {
		m.status = "Target project already has a session with that name."
		return m, nil
	}
	newTmuxName := tmuxSessionName(targetProject, session.Name)
	oldTmuxName := session.TmuxName
	return m, func() tea.Msg {
		return sessionMovedMsg{targetProject: targetProject, err: m.tmux.RenameSession(oldTmuxName, newTmuxName)}
	}
}

func (m *model) applyMoveSelectedSession(targetProject string) {
	sourceProject := m.currentProjectName()
	session := m.selectedSessionState()
	if session == nil {
		return
	}
	moved := *session
	moved.TmuxName = tmuxSessionName(targetProject, session.Name)
	m.removeSession(sourceProject, session.Name)
	m.addSession(targetProject, moved)
	m.state.CurrentProject = targetProject
	m.selectedSession = moved.Name
	if strings.TrimSpace(m.currentTmuxSession) == strings.TrimSpace(session.TmuxName) {
		m.currentTmuxSession = moved.TmuxName
	}
}

func (m *model) applyProjectRename(oldName, newName string) {
	project := m.findProject(oldName)
	if project == nil {
		return
	}
	for i := range project.Sessions {
		if strings.TrimSpace(m.currentTmuxSession) == strings.TrimSpace(project.Sessions[i].TmuxName) {
			m.currentTmuxSession = tmuxSessionName(newName, project.Sessions[i].Name)
		}
		project.Sessions[i].TmuxName = tmuxSessionName(newName, project.Sessions[i].Name)
	}
	project.Name = newName
	for i := range m.state.Projects {
		if m.state.Projects[i].Name == oldName {
			m.state.Projects[i] = *project
			break
		}
	}
	if cfg, ok := m.projectCfg[oldName]; ok {
		delete(m.projectCfg, oldName)
		cfg.Name = newName
		m.projectCfg[newName] = normalizeProjectConfig(cfg)
		_ = removeProjectConfigFile(m.statePath, oldName)
	}
	if m.state.CurrentProject == oldName {
		m.state.CurrentProject = newName
	}
	m.syncSelection()
}

func (m *model) renameSelectedSessionState(oldName, newName string) {
	project := m.findProject(m.currentProjectName())
	if project == nil {
		return
	}
	for i := range project.Sessions {
		if project.Sessions[i].Name == oldName {
			project.Sessions[i].Name = newName
			project.Sessions[i].TmuxName = tmuxSessionName(project.Name, newName)
			break
		}
	}
}

func (m *model) addProject(name string) {
	name = normalizeProjectName(name)
	if name == "" || m.findProject(name) != nil {
		return
	}
	m.state.Projects = append(m.state.Projects, projectState{Name: name})
	m.state = normalizeAppState(m.state)
	if _, ok := m.projectCfg[name]; !ok {
		m.projectCfg[name] = normalizeProjectConfig(projectConfig{Name: name})
	}
}

func (m *model) removeProject(name string) {
	name = normalizeProjectName(name)
	projects := make([]projectState, 0, len(m.state.Projects))
	for _, project := range m.state.Projects {
		if project.Name == name {
			continue
		}
		projects = append(projects, project)
	}
	m.state.Projects = projects
	delete(m.projectCfg, name)
	_ = removeProjectConfigFile(m.statePath, name)
	if m.state.CurrentProject == name {
		m.state.CurrentProject = ""
	}
	m.state = normalizeAppState(m.state)
	m.syncSelection()
}

func (m *model) addSession(project string, session sessionState) {
	project = normalizeProjectName(project)
	session = normalizeSessionState(session)
	if project == "" || session.Name == "" {
		return
	}
	if m.findProject(project) == nil {
		m.addProject(project)
	}
	for i := range m.state.Projects {
		if m.state.Projects[i].Name != project {
			continue
		}
		m.state.Projects[i].Sessions = append(m.state.Projects[i].Sessions, session)
		m.state.Projects[i].Sessions = normalizeProjectSessions(m.state.Projects[i].Sessions)
		break
	}
	if m.state.CurrentProject == "" {
		m.state.CurrentProject = project
	}
}

func (m *model) removeSession(project, name string) {
	projectState := m.findProject(project)
	if projectState == nil {
		return
	}
	sessions := make([]sessionState, 0, len(projectState.Sessions))
	for _, session := range projectState.Sessions {
		if session.Name == name {
			continue
		}
		sessions = append(sessions, session)
	}
	projectState.Sessions = sessions
	m.syncSelection()
}

func (m *model) shiftSelection(delta int) {
	sessions := m.projectSessions(m.currentProjectName())
	if len(sessions) == 0 {
		m.selectedSession = ""
		return
	}
	index := 0
	for i, session := range sessions {
		if session.Name == m.selectedSession {
			index = i
			break
		}
	}
	index = (index + delta + len(sessions)) % len(sessions)
	m.selectedSession = sessions[index].Name
}

func (m *model) syncSelection() {
	m.state = normalizeAppState(m.state)
	current := m.currentProjectName()
	if current == "" {
		m.selectedSession = ""
		return
	}
	sessions := m.projectSessions(current)
	if len(sessions) == 0 {
		m.selectedSession = ""
		return
	}
	if m.selectedSession != "" {
		for _, session := range sessions {
			if session.Name == m.selectedSession {
				return
			}
		}
	}
	if live := m.currentStateSession(); live != nil && liveProjectName(m.state, m.currentTmuxSession) == current {
		m.selectedSession = live.Name
		return
	}
	m.selectedSession = sessions[0].Name
}

func (m *model) syncCurrentProjectFromSession() {
	if project := liveProjectName(m.state, m.currentTmuxSession); project != "" {
		m.state.CurrentProject = project
		return
	}
	m.state = normalizeAppState(m.state)
}

func liveProjectName(state appState, tmuxName string) string {
	tmuxName = strings.TrimSpace(tmuxName)
	if tmuxName == "" {
		return normalizeProjectName(state.CurrentProject)
	}
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			if session.TmuxName == tmuxName {
				return project.Name
			}
		}
	}
	return normalizeProjectName(state.CurrentProject)
}

func (m model) selectedSessionState() *sessionState {
	if m.selectedSession == "" {
		return nil
	}
	return m.findSession(m.currentProjectName(), m.selectedSession)
}

func (m model) currentStateSession() *sessionState {
	for i := range m.state.Projects {
		for j := range m.state.Projects[i].Sessions {
			if m.state.Projects[i].Sessions[j].TmuxName == m.currentTmuxSession {
				return &m.state.Projects[i].Sessions[j]
			}
		}
	}
	return nil
}

func (m model) findProject(name string) *projectState {
	name = normalizeProjectName(name)
	for i := range m.state.Projects {
		if m.state.Projects[i].Name == name {
			return &m.state.Projects[i]
		}
	}
	return nil
}

func (m model) findSession(project, sessionName string) *sessionState {
	projectState := m.findProject(project)
	if projectState == nil {
		return nil
	}
	for i := range projectState.Sessions {
		if projectState.Sessions[i].Name == sessionName {
			return &projectState.Sessions[i]
		}
	}
	return nil
}

func (m model) currentProjectName() string {
	return normalizeProjectName(m.state.CurrentProject)
}

func (m model) projectNames() []string {
	result := make([]string, 0, len(m.state.Projects))
	for _, project := range m.state.Projects {
		result = append(result, project.Name)
	}
	return result
}

func (m model) projectSessions(project string) []sessionState {
	projectState := m.findProject(project)
	if projectState == nil {
		return nil
	}
	return append([]sessionState(nil), projectState.Sessions...)
}

func (m *model) pruneMissingSessions() bool {
	changed := false
	for i := range m.state.Projects {
		filtered := m.state.Projects[i].Sessions[:0]
		for _, session := range m.state.Projects[i].Sessions {
			if _, ok := m.tmuxSessions[session.TmuxName]; ok {
				filtered = append(filtered, session)
				continue
			}
			changed = true
		}
		m.state.Projects[i].Sessions = append([]sessionState(nil), filtered...)
	}
	return changed
}

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.ClosePane(m.paneID)}
	}
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	base := m.renderSidebar()
	switch m.mode {
	case inputCreateSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay(base, titleForSessionType(m.createSessionType)))
	case inputCreateProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay(base, "New Project"))
	case inputRenameProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay(base, "Rename Project"))
	case inputRenameSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay(base, "Rename Session"))
	case inputConfirmTerminate:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderConfirmTerminate(base))
	case inputProjectOverlay, inputMoveSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderProjectOverlay(base))
	default:
		return appStyle.Width(m.width).Height(m.height).Render(base)
	}
}

func (m model) renderSidebar() string {
	innerWidth := max(28, m.width-4)
	header := m.renderHeader(innerWidth)
	body := m.renderBody(innerWidth)
	footer := m.renderFooter(innerWidth)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m model) renderHeader(width int) string {
	project := fallbackText(m.currentProjectName(), "none")
	left := brandBadgeStyle.Render("TFLOW") + " " + titleStyle.Render(project)
	right := mutedStyle.Render(fmt.Sprintf("%d projects  %d sessions", len(m.state.Projects), len(m.projectSessions(m.currentProjectName()))))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return headerStyle.Width(width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) renderBody(width int) string {
	lines := []string{sectionTitleStyle.Render("Sessions"), ""}
	sessions := m.projectSessions(m.currentProjectName())
	if m.currentProjectName() == "" {
		lines = append(lines, mutedStyle.Render("No project selected."))
	} else if len(sessions) == 0 {
		lines = append(lines, mutedStyle.Render("No sessions in this project."))
	} else {
		for _, session := range sessions {
			lines = append(lines, m.renderSessionRow(width, session))
		}
	}
	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderSessionRow(width int, session sessionState) string {
	selected := session.Name == m.selectedSession
	parts := []string{}
	if session.Type == sessionTypeAgent {
		parts = append(parts, countBadgeStyle.Render("[agent]"))
	}
	if session.TmuxName == m.currentTmuxSession {
		parts = append(parts, currentBadgeStyle.Render("[live]"))
	}
	parts = append(parts, session.Name)
	content := strings.Join(parts, " ")
	style := sessionStyle
	if selected {
		style = selectedSessionStyle
	}
	return style.Width(max(16, width-6)).Render(content)
}

func (m model) renderFooter(width int) string {
	lines := []string{hintStyle.Render("j/k move  Enter open  t terminal  a agent  r rename session  R rename project  P projects  m move  Esc close")}
	if status := m.statusView(); status != "" {
		lines = append(lines, "", status)
	}
	return footerStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderInputOverlay(base, title string) string {
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("project: " + fallbackText(m.currentProjectName(), "none")),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(42, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderConfirmTerminate(base string) string {
	project := fallbackText(m.currentProjectName(), "none")
	lines := []string{
		titleStyle.Render("Terminate Project"),
		mutedStyle.Render("Terminate project " + project + "?"),
		"",
		hintStyle.Render("Enter or y confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(28, min(46, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderProjectOverlay(base string) string {
	title := "Project Overlay"
	description := "Type a project hint."
	if m.mode == inputMoveSession {
		title = "Move Session"
		description = "Type a target project hint."
	}
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render(description),
		mutedStyle.Render("hint: " + fallbackText(m.overlay.Query, "type letters")),
		"",
	}
	if len(m.overlay.Targets) == 0 {
		lines = append(lines, mutedStyle.Render("No matching projects."))
	} else {
		for _, target := range m.overlay.Targets {
			lines = append(lines, m.renderOverlayTarget(target))
		}
	}
	lines = append(lines, "")
	if m.mode == inputProjectOverlay {
		lines = append(lines, hintStyle.Render("Type a hint. Press n to create a project. Esc closes."))
	} else {
		lines = append(lines, hintStyle.Render("Type a hint. Esc closes."))
	}
	box := overlayStyle.Width(max(28, min(46, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderOverlayTarget(target overlayTarget) string {
	hint := target.Hint
	if strings.HasPrefix(target.Hint, m.overlay.Query) && m.overlay.Query != "" {
		hint = lipgloss.NewStyle().Bold(true).Foreground(yellowColor).Render(target.Hint)
	}
	return fmt.Sprintf("%s %s", countBadgeStyle.Render("["+hint+"]"), target.Project)
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

func titleForSessionType(value sessionType) string {
	if value == sessionTypeAgent {
		return "New Agent Session"
	}
	return "New Terminal Session"
}

func (m *model) startSessionCreate(value sessionType) (tea.Model, tea.Cmd) {
	if m.currentProjectName() == "" {
		m.status = "Create a project first."
		return m, nil
	}
	m.mode = inputCreateSession
	m.createSessionType = value
	m.input.Prompt = "session: "
	if value == sessionTypeAgent {
		m.input.SetValue("agent")
		m.status = fmt.Sprintf("Create a new agent session in %s.", m.currentProjectName())
	} else {
		m.input.SetValue("")
		m.status = fmt.Sprintf("Create a new terminal session in %s.", m.currentProjectName())
	}
	m.input.Focus()
	m.input.CursorEnd()
	return m, nil
}

func startupCommandForSession(cfg projectConfig, kind sessionType) (string, error) {
	switch kind {
	case sessionTypeAgent:
		return "exec " + shellQuote(cfg.agentBinary()), nil
	default:
		return "", nil
	}
}

func projectWorkdir(cfg projectConfig, fallback string) string {
	if strings.TrimSpace(cfg.Workdir) != "" {
		return cfg.Workdir
	}
	return fallback
}

func buildOverlayTargets(projects []string) []overlayTarget {
	hints := buildHints(projects)
	targets := make([]overlayTarget, 0, len(projects))
	for _, project := range projects {
		targets = append(targets, overlayTarget{Project: project, Hint: hints[project]})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Project < targets[j].Project
	})
	return targets
}

func buildHints(projects []string) map[string]string {
	normalized := append([]string(nil), projects...)
	sort.Strings(normalized)
	hints := map[string]string{}
	for _, project := range normalized {
		runes := []rune(project)
		for size := 1; size <= len(runes); size++ {
			prefix := string(runes[:size])
			unique := true
			for _, other := range normalized {
				if other == project {
					continue
				}
				if strings.HasPrefix(other, prefix) {
					unique = false
					break
				}
			}
			if unique || size == len(runes) {
				hints[project] = prefix
				break
			}
		}
	}
	return hints
}

func (m model) overlayProjects(action overlayAction) []string {
	current := m.currentProjectName()
	projects := make([]string, 0, len(m.state.Projects))
	for _, project := range m.projectNames() {
		if project == current {
			continue
		}
		projects = append(projects, project)
	}
	return projects
}

func (m *model) updateOverlayStatus() {
	if m.overlay.Query == "" {
		if m.mode == inputMoveSession {
			m.status = "Move session: type a project hint."
			return
		}
		m.status = "Project overlay: type a project hint."
		return
	}
	for _, target := range m.overlay.Targets {
		if strings.HasPrefix(target.Hint, m.overlay.Query) {
			m.status = "Matching hint: " + target.Hint
			return
		}
	}
	m.status = "No matching hint."
}

func (m model) saveState() error {
	return saveAppState(m.statePath, m.state, m.projectCfg)
}

func (m model) projectConfig(project string) projectConfig {
	project = normalizeProjectName(project)
	if project == "" {
		return projectConfig{}
	}
	if cfg, ok := m.projectCfg[project]; ok {
		return normalizeProjectConfig(cfg)
	}
	return normalizeProjectConfig(projectConfig{Name: project})
}

func appStatePath() string {
	return filepathDir(appConfigPath()) + "/state.json"
}

func findProjectState(state appState, name string) *projectState {
	name = normalizeProjectName(name)
	for i := range state.Projects {
		if state.Projects[i].Name == name {
			return &state.Projects[i]
		}
	}
	return nil
}

func saveAppState(path string, state appState, cfg map[string]projectConfig) error {
	state = normalizeAppState(state)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	return saveProjectConfigs(path, cfg)
}

func loadAppState(path string) (appState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return appState{}, nil
		}
		return appState{}, err
	}
	var state appState
	if err := json.Unmarshal(data, &state); err == nil {
		return normalizeAppState(state), nil
	}
	var legacy legacyAppState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return appState{}, err
	}
	return normalizeLegacyState(legacy), nil
}

func normalizeLegacyState(legacy legacyAppState) appState {
	projects := map[string][]sessionState{}
	for _, name := range legacy.Projects {
		name = normalizeProjectName(name)
		if name == "" {
			continue
		}
		projects[name] = append(projects[name], []sessionState{}...)
	}
	for tmuxName, project := range legacy.SessionProjects {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		projects[project] = append(projects[project], sessionState{
			Name:     sanitizeSessionName(tmuxName),
			TmuxName: sanitizeSessionName(tmuxName),
			Type:     normalizeSessionType(legacy.SessionTypes[tmuxName]),
		})
	}
	state := appState{}
	for project, sessions := range projects {
		state.Projects = append(state.Projects, projectState{Name: project, Sessions: normalizeProjectSessions(sessions)})
	}
	return normalizeAppState(state)
}

func normalizeAppState(state appState) appState {
	seenProjects := map[string]struct{}{}
	normalizedProjects := make([]projectState, 0, len(state.Projects))
	for _, project := range state.Projects {
		project.Name = normalizeProjectName(project.Name)
		if project.Name == "" {
			continue
		}
		if _, ok := seenProjects[project.Name]; ok {
			continue
		}
		seenProjects[project.Name] = struct{}{}
		project.Sessions = normalizeProjectSessions(project.Sessions)
		normalizedProjects = append(normalizedProjects, project)
	}
	sort.Slice(normalizedProjects, func(i, j int) bool {
		return normalizedProjects[i].Name < normalizedProjects[j].Name
	})
	state.Projects = normalizedProjects
	state.CurrentProject = normalizeProjectName(state.CurrentProject)
	if state.CurrentProject == "" && len(state.Projects) > 0 {
		state.CurrentProject = state.Projects[0].Name
	}
	if state.CurrentProject != "" {
		found := false
		for _, project := range state.Projects {
			if project.Name == state.CurrentProject {
				found = true
				break
			}
		}
		if !found {
			state.CurrentProject = ""
			if len(state.Projects) > 0 {
				state.CurrentProject = state.Projects[0].Name
			}
		}
	}
	return state
}

func normalizeProjectSessions(sessions []sessionState) []sessionState {
	seen := map[string]struct{}{}
	result := make([]sessionState, 0, len(sessions))
	for _, session := range sessions {
		session = normalizeSessionState(session)
		if session.Name == "" || session.TmuxName == "" {
			continue
		}
		if _, ok := seen[session.Name]; ok {
			continue
		}
		seen[session.Name] = struct{}{}
		result = append(result, session)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func normalizeSessionState(session sessionState) sessionState {
	session.Name = sanitizeSessionName(session.Name)
	session.TmuxName = sanitizeTmuxName(session.TmuxName)
	session.Type = normalizeSessionType(string(session.Type))
	if session.Type == "" {
		session.Type = sessionTypeTerminal
	}
	return session
}

func normalizeSessionType(value string) sessionType {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(sessionTypeAgent):
		return sessionTypeAgent
	default:
		return sessionTypeTerminal
	}
}

func syncTmuxState(manager tmuxController, state appState) error {
	metadata := map[string]sessionMetadata{}
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			metadata[session.TmuxName] = sessionMetadata{Project: project.Name, DisplayName: session.Name}
		}
	}
	return manager.SyncSessionMetadata(metadata)
}

func (m model) syncTmuxState() error {
	return syncTmuxState(m.tmux, m.state)
}

func nextBootstrapProjectName(state appState) string {
	used := map[string]struct{}{}
	for _, project := range state.Projects {
		used[project.Name] = struct{}{}
	}
	for _, animal := range bootstrapProjectAnimals {
		if _, ok := used[animal]; !ok {
			return animal
		}
	}
	for i := 2; ; i++ {
		for _, animal := range bootstrapProjectAnimals {
			name := fmt.Sprintf("%s-%d", animal, i)
			if _, ok := used[name]; !ok {
				return name
			}
		}
	}
}

func tmuxSessionName(project, session string) string {
	project = sanitizeProjectName(project)
	session = sanitizeSessionName(session)
	if project == "" {
		return session
	}
	if session == "" {
		return project
	}
	return project + "_" + session
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

func sanitizeSessionName(name string) string {
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

func sanitizeTmuxName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastSep := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastSep = false
		case r == '_':
			if !lastSep && b.Len() > 0 {
				b.WriteByte('_')
				lastSep = true
			}
		case r == '-', unicode.IsSpace(r), r == '/', r == '.':
			if !lastSep && b.Len() > 0 {
				b.WriteByte('-')
				lastSep = true
			}
		}
	}
	return strings.Trim(b.String(), "-_")
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
		project = "project"
	}
	hash := 0
	for _, r := range project {
		hash += int(r)
	}
	return palette[hash%len(palette)]
}

func fallbackText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func filepathDir(path string) string {
	index := strings.LastIndex(path, "/")
	if index < 0 {
		return "."
	}
	if index == 0 {
		return "/"
	}
	return path[:index]
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
