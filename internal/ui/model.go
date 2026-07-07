package ui

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
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
	inputRenameSession
	inputProjectHints
	inputMoveSession
	inputPersistProject
)

type selectionScope int

const (
	selectionSessions selectionScope = iota
	selectionProjects
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
	CurrentProject  string            `json:"current_project"`
	CurrentSessions map[string]string `json:"current_sessions,omitempty"`
	Projects        []projectState    `json:"projects"`
}

type projectState struct {
	Name       string         `json:"name"`
	Persistent bool           `json:"persistent"`
	Sessions   []sessionState `json:"sessions"`
}

type sessionState struct {
	Name     string      `json:"name"`
	TmuxName string      `json:"tmux_name"`
	Type     sessionType `json:"type"`
	CWD      string      `json:"cwd,omitempty"`
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

type projectPersistedMsg struct {
	oldName string
	config  projectConfig
	err     error
}

type sessionMovedMsg struct {
	targetProject string
	err           error
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

type persistProjectInput struct {
	Field    int
	Name     string
	Workdir  string
	AgentCmd string
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
	selectedProject    string
	selection          selectionScope
	tmuxSessions       map[string]session

	mode    inputMode
	overlay overlayState
	input   textinput.Model
	persist persistProjectInput

	createSessionType sessionType
	showHelp          bool
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
	runErr := cmd.Run()
	cleanupErr := cleanupOwnedVolatileProject(manager, sessionName)
	if runErr != nil {
		return runErr
	}
	return cleanupErr
}

func OpenMenu() error {
	_, err := tea.NewProgram(NewMenu(), tea.WithAltScreen()).Run()
	return err
}

func prepareStartup(manager tmuxController, binaryPath, cwd string) (string, error) {
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return "", err
	}

	statePath := appStatePath()
	previous, err := loadAppState(statePath)
	if err != nil {
		return "", err
	}
	cfgs, err := loadProjectConfigs(statePath, previous)
	if err != nil {
		return "", err
	}

	state := stateFromPersistentConfig(previous, cfgs, cwd)
	volatileProject := nextVolatileProjectName(state)
	volatileSession := sessionState{
		Name:     defaultSessionName,
		TmuxName: tmuxSessionName(volatileProject, defaultSessionName),
		Type:     sessionTypeTerminal,
		CWD:      normalizeCWD(cwd),
	}
	state.Projects = append(state.Projects, projectState{
		Name:       volatileProject,
		Persistent: false,
		Sessions:   []sessionState{volatileSession},
	})
	state.CurrentProject = volatileProject
	state = normalizeAppState(state)

	for _, project := range state.Projects {
		cfg := cfgs[project.Name]
		if !project.Persistent {
			cfg = projectConfig{Name: project.Name, Workdir: cwd}
		}
		for _, session := range project.Sessions {
			if err := ensureTmuxSession(manager, cfg, session); err != nil {
				return "", err
			}
			if err := manager.SetSessionTemporary(session.TmuxName, !project.Persistent); err != nil {
				return "", err
			}
		}
	}
	if err := saveAppState(statePath, state, cfgs); err != nil {
		return "", err
	}
	if err := syncTmuxState(manager, state); err != nil {
		return "", err
	}
	return volatileSession.TmuxName, nil
}

func stateFromPersistentConfig(previous appState, cfgs map[string]projectConfig, fallbackCWD string) appState {
	state := appState{}
	names := make([]string, 0, len(cfgs))
	for name := range cfgs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		cfg := normalizeProjectConfig(cfgs[name])
		if cfg.Name == "" {
			continue
		}
		project := projectState{Name: cfg.Name, Persistent: true}
		if previousProject := findProjectState(previous, cfg.Name); previousProject != nil {
			project.Sessions = append([]sessionState(nil), previousProject.Sessions...)
		}
		if len(project.Sessions) == 0 {
			project.Sessions = []sessionState{{
				Name:     defaultSessionName,
				TmuxName: tmuxSessionName(cfg.Name, defaultSessionName),
				Type:     sessionTypeTerminal,
				CWD:      projectWorkdir(cfg, fallbackCWD),
			}}
		}
		state.Projects = append(state.Projects, project)
	}
	return normalizeAppState(state)
}

func ensureTmuxSession(manager tmuxController, cfg projectConfig, session sessionState) error {
	command, err := startupCommandForSession(cfg, session.Type)
	if err != nil {
		return err
	}
	_, err = manager.CreateSession(session.TmuxName, sessionCWD(session, cfg, defaultSessionDir()), command)
	return err
}

func cleanupOwnedVolatileProject(manager tmuxController, tmuxSession string) error {
	statePath := appStatePath()
	state, err := loadAppState(statePath)
	if err != nil {
		return err
	}
	cfgs, err := loadProjectConfigs(statePath, state)
	if err != nil {
		return err
	}
	if nextState, changed, err := updatePersistentSessionCWDs(manager, state); err != nil {
		return err
	} else if changed {
		state = nextState
	}
	project := projectForTmuxSession(state, tmuxSession)
	if project == nil || project.Persistent {
		return saveAppState(statePath, state, cfgs)
	}
	for _, session := range project.Sessions {
		if err := manager.KillSession(session.TmuxName); err != nil {
			return err
		}
	}
	removeProjectFromState(&state, project.Name)
	return saveAppState(statePath, state, cfgs)
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
	projectCfg, cfgErr2 := loadProjectConfigs(statePath, state)
	if cfgErr2 != nil {
		projectCfg = map[string]projectConfig{}
		if err == nil {
			err = cfgErr2
		}
	}
	state = reconcileRuntimeState(state, projectCfg, cwd)
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
	if nextState, changed, cwdErr := updatePersistentSessionCWDs(m.tmux, m.state); cwdErr != nil {
		if m.err == nil {
			m.err = cwdErr
		}
		m.status = cwdErr.Error()
	} else if changed {
		m.state = nextState
		if saveErr := m.saveState(); saveErr != nil {
			if m.err == nil {
				m.err = saveErr
			}
			m.status = saveErr.Error()
		}
	}
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
		m.setCurrentSession(project, msg.session.Name)
		m.selectedSession = msg.session.Name
		m.selection = selectionSessions
		m.currentTmuxSession = msg.session.TmuxName
		m.status = ""
		m.err = nil
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.switchToSessionCmd(msg.session.TmuxName)
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.renameSelectedSessionState(msg.oldName, msg.newName)
		m.setCurrentSession(m.currentProjectName(), msg.newName)
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
	case projectPersistedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.applyPersistProject(msg.oldName, msg.config)
		m.mode = inputNone
		m.input.Blur()
		m.input.Prompt = ""
		m.persist = persistProjectInput{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.status = "Project persisted."
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
	case tea.KeyCtrlC, tea.KeyEsc:
		if err := m.updateCurrentSessionCWD(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.closeMenuCmd()
	}

	switch msg.String() {
	case "ctrl+q":
		if err := m.updateCurrentSessionCWD(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.quitAllCmd()
	case "tab":
		m.toggleSelectionScope()
		return m, nil
	case "j", "down":
		m.shiftSelection(1)
		return m, nil
	case "k", "up":
		m.shiftSelection(-1)
		return m, nil
	case "enter":
		if m.selection == selectionProjects {
			return m.switchSelectedProject()
		}
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
	case "p":
		m.openProjectOverlay(overlaySwitchProject)
		return m, nil
	case "P":
		return m.startPersistProject()
	case "m":
		if m.selectedSessionState() == nil {
			m.status = "Select a session to move."
			return m, nil
		}
		m.openProjectOverlay(overlayMoveSession)
		return m, nil
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	}

	return m, nil
}

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+q" {
		if err := m.updateCurrentSessionCWD(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.quitAllCmd()
	}
	if msg.Type == tea.KeyCtrlC {
		if err := m.updateCurrentSessionCWD(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		return m, m.closeMenuCmd()
	}
	switch m.mode {
	case inputCreateSession:
		return m.updateCreateSession(msg)
	case inputRenameSession:
		return m.updateRenameSession(msg)
	case inputProjectHints, inputMoveSession:
		return m.updateOverlay(msg)
	case inputPersistProject:
		return m.updatePersistProject(msg)
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
			return sessionCreatedMsg{session: sessionState{Name: name, TmuxName: tmuxName, Type: m.createSessionType, CWD: cwd}}
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

func (m model) updatePersistProject(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.resetInput("Persist cancelled.")
		m.persist = persistProjectInput{}
		return m, nil
	case tea.KeyEnter:
		m.savePersistField()
		if m.persist.Field < 2 {
			m.persist.Field++
			m.loadPersistField()
			return m, nil
		}
		cfg := normalizeProjectConfig(projectConfig{
			Name:        m.persist.Name,
			Workdir:     m.persist.Workdir,
			AgentBinary: m.persist.AgentCmd,
		})
		if cfg.Name == "" {
			m.status = "Project name is empty."
			m.persist.Field = 0
			m.loadPersistField()
			return m, nil
		}
		oldName := m.currentProjectName()
		if oldName == "" {
			m.status = "No project selected."
			return m, nil
		}
		if project := m.findProject(oldName); project == nil || project.Persistent {
			m.status = "Current project is already persistent."
			return m, nil
		}
		if existing := m.findProject(cfg.Name); existing != nil && existing.Name != oldName {
			m.status = "Project already exists."
			return m, nil
		}
		project := m.findProject(oldName)
		renames := make([][2]string, 0, len(project.Sessions))
		for _, session := range project.Sessions {
			renames = append(renames, [2]string{session.TmuxName, tmuxSessionName(cfg.Name, session.Name)})
		}
		m.resetInput("")
		return m, func() tea.Msg {
			for _, rename := range renames {
				if err := m.tmux.RenameSession(rename[0], rename[1]); err != nil {
					return projectPersistedMsg{oldName: oldName, config: cfg, err: err}
				}
			}
			return projectPersistedMsg{oldName: oldName, config: cfg}
		}
	}
	next, cmd := m.input.Update(msg)
	m.input = next
	return m, cmd
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
	if len(m.overlay.Targets) > 0 {
		m.selectedProject = m.overlay.Targets[0].Project
	}
	if action == overlayMoveSession {
		m.mode = inputMoveSession
		m.status = "Move session: type a project hint."
		return
	}
	m.mode = inputProjectHints
	if len(m.overlay.Targets) == 0 {
		m.status = "No persistent projects."
	} else {
		m.status = "Switch project: type a project hint."
	}
}

func (m model) switchSelectedProject() (tea.Model, tea.Cmd) {
	if m.selectedProject == "" {
		m.status = "No project selected."
		return m, nil
	}
	return m.switchProject(m.selectedProject)
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	session := m.selectedSessionState()
	if session == nil {
		m.status = "No session selected."
		return m, nil
	}
	if err := m.updateCurrentSessionCWD(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	project := m.currentProjectName()
	m.setCurrentSession(project, session.Name)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	cfg := m.projectConfig(project)
	return m, func() tea.Msg {
		err := ensureTmuxSession(m.tmux, cfg, *session)
		if err == nil {
			err = m.tmux.SwitchClient(session.TmuxName)
		}
		if err == nil {
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
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
	if projectState == nil || !projectState.Persistent {
		m.status = "Persistent project not found."
		return m, nil
	}
	if err := m.updateCurrentSessionCWD(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.state.CurrentProject = project
	m.selectedSession = ""
	m.syncSelection()
	if len(projectState.Sessions) == 0 {
		m.mode = inputNone
		m.overlay = overlayState{}
		m.status = "Switched to empty project."
		return m, nil
	}
	target := m.projectCurrentSession(*projectState)
	m.setCurrentSession(project, target.Name)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	cfg := m.projectConfig(project)
	return m, func() tea.Msg {
		err := ensureTmuxSession(m.tmux, cfg, target)
		if err == nil {
			err = m.tmux.SwitchClient(target.TmuxName)
		}
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
	project := m.findProject(targetProject)
	if project == nil || !project.Persistent {
		m.status = "Target must be persistent."
		return m, nil
	}
	if m.findSession(targetProject, session.Name) != nil {
		m.status = "Target project already has a session with that name."
		return m, nil
	}
	if err := m.updateCurrentSessionCWD(); err != nil {
		m.err = err
		m.status = err.Error()
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
	if m.currentSessionName(sourceProject) == session.Name {
		m.clearCurrentSession(sourceProject)
	}
	m.setCurrentSession(targetProject, moved.Name)
	m.selectedSession = ""
	if strings.TrimSpace(m.currentTmuxSession) == strings.TrimSpace(session.TmuxName) {
		m.currentTmuxSession = moved.TmuxName
	}
}

func (m *model) applyPersistProject(oldName string, cfg projectConfig) {
	oldName = normalizeProjectName(oldName)
	cfg = normalizeProjectConfig(cfg)
	project := m.findProject(oldName)
	if project == nil || cfg.Name == "" {
		return
	}
	for i := range project.Sessions {
		oldTmuxName := project.Sessions[i].TmuxName
		project.Sessions[i].TmuxName = tmuxSessionName(cfg.Name, project.Sessions[i].Name)
		if project.Sessions[i].CWD == "" {
			project.Sessions[i].CWD = projectWorkdir(cfg, m.cwd)
		}
		if strings.TrimSpace(m.currentTmuxSession) == strings.TrimSpace(oldTmuxName) {
			m.currentTmuxSession = project.Sessions[i].TmuxName
		}
	}
	if currentSession := m.currentSessionName(oldName); currentSession != "" {
		m.clearCurrentSession(oldName)
		m.setCurrentSession(cfg.Name, currentSession)
	}
	project.Name = cfg.Name
	project.Persistent = true
	m.projectCfg[cfg.Name] = cfg
	if oldName != cfg.Name {
		delete(m.projectCfg, oldName)
	}
	m.state.CurrentProject = cfg.Name
	m.syncSelection()
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
	projects := m.persistentProjectNames()
	if len(sessions) == 0 && len(projects) == 0 {
		m.selectedSession = ""
		m.selectedProject = ""
		m.selection = selectionSessions
		return
	}

	if m.selection == selectionProjects {
		if len(projects) == 0 {
			m.selection = selectionSessions
			m.shiftSelection(delta)
			return
		}
		index := 0
		for i, project := range projects {
			if project == m.selectedProject {
				index = i
				break
			}
		}
		index = (index + delta + len(projects)) % len(projects)
		m.selectedProject = projects[index]
		return
	}

	if len(sessions) == 0 {
		m.selection = selectionProjects
		m.shiftSelection(delta)
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

func (m *model) toggleSelectionScope() {
	sessions := m.projectSessions(m.currentProjectName())
	projects := m.persistentProjectNames()
	if m.selection == selectionSessions && len(projects) > 0 {
		m.selection = selectionProjects
		if !containsString(projects, m.selectedProject) {
			m.selectedProject = projects[0]
		}
		return
	}
	if len(sessions) > 0 {
		m.selection = selectionSessions
		if !containsSessionName(sessions, m.selectedSession) {
			m.selectedSession = sessions[0].Name
		}
	}
}

func (m *model) syncSelection() {
	m.state = normalizeAppState(m.state)
	current := m.currentProjectName()
	sessions := m.projectSessions(current)
	projects := m.persistentProjectNames()

	if len(projects) == 0 {
		m.selectedProject = ""
	} else if !containsString(projects, m.selectedProject) {
		m.selectedProject = projects[0]
		if containsString(projects, current) {
			m.selectedProject = current
		}
	}

	if current == "" || len(sessions) == 0 {
		m.selectedSession = ""
		if len(projects) > 0 {
			m.selection = selectionProjects
		}
		return
	}

	if !containsSessionName(sessions, m.selectedSession) {
		if currentSession := m.currentSessionName(current); containsSessionName(sessions, currentSession) {
			m.selectedSession = currentSession
		} else if live := m.currentStateSession(); live != nil && liveProjectName(m.state, m.currentTmuxSession) == current {
			m.selectedSession = live.Name
		} else {
			m.selectedSession = sessions[0].Name
		}
	}

	if m.selection == selectionProjects && len(projects) == 0 {
		m.selection = selectionSessions
	}
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func containsSessionName(sessions []sessionState, name string) bool {
	for _, session := range sessions {
		if session.Name == name {
			return true
		}
	}
	return false
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
	if m.selection != selectionSessions || m.selectedSession == "" {
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

func (m model) persistentProjectNames() []string {
	result := make([]string, 0, len(m.state.Projects))
	for _, project := range m.state.Projects {
		if project.Persistent {
			result = append(result, project.Name)
		}
	}
	sort.Strings(result)
	return result
}

func (m model) projectSessions(project string) []sessionState {
	projectState := m.findProject(project)
	if projectState == nil {
		return nil
	}
	return append([]sessionState(nil), projectState.Sessions...)
}

func (m model) currentSessionName(project string) string {
	project = normalizeProjectName(project)
	if project == "" || m.state.CurrentSessions == nil {
		return ""
	}
	return sanitizeSessionName(m.state.CurrentSessions[project])
}

func (m *model) setCurrentSession(project, session string) {
	project = normalizeProjectName(project)
	session = sanitizeSessionName(session)
	if project == "" || session == "" {
		return
	}
	if m.state.CurrentSessions == nil {
		m.state.CurrentSessions = map[string]string{}
	}
	m.state.CurrentSessions[project] = session
}

func (m *model) clearCurrentSession(project string) {
	project = normalizeProjectName(project)
	if project == "" || m.state.CurrentSessions == nil {
		return
	}
	delete(m.state.CurrentSessions, project)
}

func (m model) projectCurrentSession(project projectState) sessionState {
	if len(project.Sessions) == 0 {
		return sessionState{}
	}
	current := m.currentSessionName(project.Name)
	for _, session := range project.Sessions {
		if session.Name == current {
			return session
		}
	}
	return project.Sessions[0]
}

func (m *model) pruneMissingSessions() bool {
	changed := false
	for i := range m.state.Projects {
		if m.state.Projects[i].Persistent {
			continue
		}
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

func (m model) quitAllCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{err: m.tmux.QuitAll(m.paneID)}
	}
}

func (m model) switchToSessionCmd(tmuxName string) tea.Cmd {
	return func() tea.Msg {
		err := m.tmux.SwitchClient(tmuxName)
		if err == nil {
			err = m.tmux.ClosePane(m.paneID)
		}
		return menuActionMsg{err: err}
	}
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}
	base := m.renderSidebar()
	if m.mode == inputPersistProject {
		return appStyle.Width(m.width).Height(m.height).Render(m.renderPersistProjectOverlay(base))
	}
	return appStyle.Width(m.width).Height(m.height).Render(base)
}

func (m model) renderSidebar() string {
	innerWidth := max(28, m.width-4)
	header := m.renderHeader(innerWidth)
	body := m.renderBody(innerWidth)
	footer := m.renderFooter(innerWidth)
	headerGap := strings.Repeat("\n", 2)
	footerGap := "\n"
	return lipgloss.JoinVertical(lipgloss.Left, header, headerGap, body, footerGap, footer)
}

func (m model) renderHeader(width int) string {
	return headerStyle.Width(width).Align(lipgloss.Center).Render(brandBadgeStyle.Render("TFLOW"))
}

func (m model) sessionListProject() string {
	if m.mode == inputProjectHints || m.mode == inputMoveSession {
		if project := m.overlayPreviewProject(); project != "" {
			return project
		}
	}
	return m.currentProjectName()
}

func (m model) overlayPreviewProject() string {
	for _, target := range m.overlay.Targets {
		if target.Project == m.selectedProject {
			return target.Project
		}
	}
	for _, target := range m.overlay.Targets {
		if m.overlay.Query == "" || strings.HasPrefix(target.Hint, m.overlay.Query) {
			return target.Project
		}
	}
	return ""
}

func (m model) renderBody(width int) string {
	sessionProject := m.sessionListProject()
	sessionLines := []string{}
	sessions := m.projectSessions(sessionProject)
	if sessionProject == "" {
		sessionLines = append(sessionLines, mutedStyle.Render("No project selected."))
	} else if len(sessions) == 0 {
		sessionLines = append(sessionLines, mutedStyle.Render("No sessions in this project."))
	} else {
		for _, session := range sessions {
			sessionLines = append(sessionLines, m.renderSessionRow(width, sessionProject, session))
		}
	}
	minSessionRows := 5
	if len(sessions) > 0 && len(sessions) < minSessionRows {
		for i := len(sessions); i < minSessionRows; i++ {
			sessionLines = append(sessionLines, "")
		}
	}

	projectLines := []string{}
	projects := m.persistentProjectNames()
	if len(projects) == 0 {
		projectLines = append(projectLines, mutedStyle.Render("No persistent projects."))
	} else {
		for _, project := range projects {
			projectLines = append(projectLines, m.renderProjectRow(width, project))
		}
	}

	sectionGap := strings.Repeat("\n", 1)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderSectionPanel(width, "Sessions", sessionLines),
		sectionGap,
		m.renderSectionPanel(width, "Projects", projectLines),
	)
}

func (m model) renderSectionPanel(width int, title string, rows []string) string {
	sectionWidth := max(16, width-6)
	lines := []string{sectionTitleStyle.Width(sectionWidth).Align(lipgloss.Center).Render(title), ""}
	lines = append(lines, rows...)
	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderSessionRow(width int, project string, session sessionState) string {
	selected := project == m.currentProjectName() && m.selection == selectionSessions && session.Name == m.selectedSession
	parts := []string{}
	if session.Type == sessionTypeAgent {
		parts = append(parts, m.renderSessionBadge("agent", selected))
	}
	if session.TmuxName == m.currentTmuxSession {
		parts = append(parts, m.renderSessionBadge("live", selected))
	}
	parts = append(parts, session.Name)
	content := strings.Join(parts, " ")
	style := sessionStyle
	if selected {
		style = selectedSessionStyle
	}
	return style.Width(max(16, width-6)).Render(content)
}

func (m model) renderSessionBadge(label string, selected bool) string {
	if selected {
		if label == "live" {
			// A beautiful mixture of Catppuccin: yellow background with dark text for contrast
			return currentBadgeStyle.Render(label)
		}
		// agent badge when selected: surface0 background, styled beautifully
		return countBadgeStyle.Render(label)
	}
	if label == "live" {
		return currentBadgeStyle.Render(label)
	}
	return countBadgeStyle.Render(label)
}

func (m model) renderProjectRow(width int, project string) string {
	selected := m.selection == selectionProjects && project == m.selectedProject
	parts := []string{}
	if m.currentProjectName() == project {
		parts = append(parts, currentBadgeStyle.Render("live"))
	}
	parts = append(parts, m.renderHintProjectName(project))
	style := projectStyle
	if selected {
		style = selectedProjectStyle
	}
	return style.Width(max(16, width-6)).Render(strings.Join(parts, " "))
}

func (m model) renderHintProjectName(project string) string {
	if m.mode != inputProjectHints && m.mode != inputMoveSession {
		return project
	}
	for _, target := range m.overlay.Targets {
		if target.Project != project || target.Hint == "" {
			continue
		}
		projectRunes := []rune(project)
		hintRunes := []rune(target.Hint)
		if len(hintRunes) > len(projectRunes) {
			return project
		}
		style := lipgloss.NewStyle().Bold(true).Foreground(yellowColor)
		if m.overlay.Query != "" && strings.HasPrefix(target.Hint, m.overlay.Query) {
			style = style.Foreground(tealColor)
		}
		return style.Render(string(projectRunes[:len(hintRunes)])) + string(projectRunes[len(hintRunes):])
	}
	return project
}

func (m model) renderFooter(width int) string {
	lines := []string{}
	if m.showHelp {
		lines = append(lines,
			hintStyle.Render("t ▶️ new terminal"),
			hintStyle.Render("a ▶️ new agent"),
			hintStyle.Render("r ▶️ rename session"),
			hintStyle.Render("m ▶️ move session"),
			hintStyle.Render("p ▶️ switch project"),
			hintStyle.Render("P ▶️ persist project"),
			hintStyle.Render("Ctrl+Q ▶️ quit tflow"),
		)
	}
	if m.mode == inputCreateSession || m.mode == inputRenameSession {
		label := titleForSessionType(m.createSessionType)
		if m.mode == inputRenameSession {
			label = "Rename Session"
		}
		lines = append(lines, "", titleStyle.Render(label), inputStyle.Render(m.input.View()), hintStyle.Render("Enter saves. Esc cancels."))
	}
	if m.mode == inputProjectHints || m.mode == inputMoveSession {
		lines = append(lines, "", hintStyle.Render("Type a project hint. Esc cancels."))
	}
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

func (m model) renderPersistProjectOverlay(base string) string {
	field := "name"
	if m.persist.Field == 1 {
		field = "workdir"
	} else if m.persist.Field == 2 {
		field = "agent-cmd"
	}
	lines := []string{
		titleStyle.Render("Persist Project"),
		mutedStyle.Render("field: " + field),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter next. Esc cancels."),
	}
	box := overlayStyle.Width(max(28, min(46, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderOverlayTarget(target overlayTarget) string {
	project := target.Project
	if target.Hint == "" {
		return project
	}
	projectRunes := []rune(project)
	hintRunes := []rune(target.Hint)
	if len(hintRunes) > len(projectRunes) {
		return project
	}
	style := lipgloss.NewStyle().Bold(true).Foreground(yellowColor)
	if strings.HasPrefix(target.Hint, m.overlay.Query) && m.overlay.Query != "" {
		style = style.Foreground(tealColor)
	}
	return style.Render(string(projectRunes[:len(hintRunes)])) + string(projectRunes[len(hintRunes):])
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

func (m model) startPersistProject() (tea.Model, tea.Cmd) {
	project := m.findProject(m.currentProjectName())
	if project == nil {
		m.status = "No project selected."
		return m, nil
	}
	if project.Persistent {
		m.status = "Current project is already persistent."
		return m, nil
	}
	m.mode = inputPersistProject
	m.persist = persistProjectInput{
		Name:     project.Name,
		Workdir:  m.cwd,
		AgentCmd: "codex",
	}
	m.persist.Field = 0
	m.loadPersistField()
	m.status = "Persist project."
	return m, nil
}

func (m *model) savePersistField() {
	value := strings.TrimSpace(m.input.Value())
	switch m.persist.Field {
	case 0:
		m.persist.Name = value
	case 1:
		m.persist.Workdir = value
	case 2:
		m.persist.AgentCmd = value
	}
}

func (m *model) loadPersistField() {
	switch m.persist.Field {
	case 0:
		m.input.Prompt = "name: "
		m.input.SetValue(m.persist.Name)
	case 1:
		m.input.Prompt = "workdir: "
		m.input.SetValue(m.persist.Workdir)
	case 2:
		m.input.Prompt = "agent-cmd: "
		m.input.SetValue(m.persist.AgentCmd)
	}
	m.input.Focus()
	m.input.CursorEnd()
}

func startupCommandForSession(cfg projectConfig, kind sessionType) (string, error) {
	switch kind {
	case sessionTypeAgent:
		command := strings.TrimSpace(cfg.agentBinary())
		if command == "" {
			return "", fmt.Errorf("agent command is not configured")
		}
		return "exec " + shellQuote(command), nil
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
	for _, project := range m.state.Projects {
		if !project.Persistent || project.Name == current {
			continue
		}
		projects = append(projects, project.Name)
	}
	sort.Strings(projects)
	return projects
}

func (m *model) updateOverlayStatus() {
	m.syncOverlaySelection()
	if m.overlay.Query == "" {
		if m.mode == inputMoveSession {
			m.status = "Move session: type a project hint."
			return
		}
		m.status = "Switch project: type a project hint."
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

func (m *model) syncOverlaySelection() {
	for _, target := range m.overlay.Targets {
		if m.overlay.Query == "" || strings.HasPrefix(target.Hint, m.overlay.Query) {
			m.selectedProject = target.Project
			return
		}
	}
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

func (m *model) updateCurrentSessionCWD() error {
	session := m.currentStateSession()
	if session == nil {
		return nil
	}
	project := projectForTmuxSession(m.state, session.TmuxName)
	if project == nil || !project.Persistent {
		return nil
	}
	cwd, err := m.tmux.SessionCWD(session.TmuxName)
	if err != nil {
		return err
	}
	cwd = normalizeCWD(cwd)
	for i := range m.state.Projects {
		if m.state.Projects[i].Name != project.Name {
			continue
		}
		for j := range m.state.Projects[i].Sessions {
			if m.state.Projects[i].Sessions[j].TmuxName == session.TmuxName {
				m.state.Projects[i].Sessions[j].CWD = cwd
				return m.saveState()
			}
		}
	}
	return nil
}

func updatePersistentSessionCWDs(manager tmuxController, state appState) (appState, bool, error) {
	state = normalizeAppState(state)
	changed := false
	for i := range state.Projects {
		if !state.Projects[i].Persistent {
			continue
		}
		for j := range state.Projects[i].Sessions {
			session := state.Projects[i].Sessions[j]
			cwd, err := manager.SessionCWD(session.TmuxName)
			if err != nil {
				if isNoTmuxSession(err) || isNoTmuxServer(err) {
					continue
				}
				return state, false, err
			}
			cwd = normalizeCWD(cwd)
			if cwd != "" && state.Projects[i].Sessions[j].CWD != cwd {
				state.Projects[i].Sessions[j].CWD = cwd
				changed = true
			}
		}
	}
	return state, changed, nil
}

func sessionCWD(session sessionState, cfg projectConfig, fallback string) string {
	if strings.TrimSpace(session.CWD) != "" {
		return normalizeCWD(session.CWD)
	}
	return projectWorkdir(cfg, fallback)
}

func reconcileRuntimeState(previous appState, cfgs map[string]projectConfig, fallbackCWD string) appState {
	state := stateFromPersistentConfig(previous, cfgs, fallbackCWD)
	for _, project := range normalizeAppState(previous).Projects {
		if project.Persistent || project.Name == "" {
			continue
		}
		state.Projects = append(state.Projects, project)
	}
	state.CurrentProject = normalizeProjectName(previous.CurrentProject)
	return normalizeAppState(state)
}

func projectForTmuxSession(state appState, tmuxName string) *projectState {
	tmuxName = strings.TrimSpace(tmuxName)
	for i := range state.Projects {
		for _, session := range state.Projects[i].Sessions {
			if session.TmuxName == tmuxName {
				return &state.Projects[i]
			}
		}
	}
	return nil
}

func removeProjectFromState(state *appState, name string) {
	name = normalizeProjectName(name)
	projects := make([]projectState, 0, len(state.Projects))
	for _, project := range state.Projects {
		if project.Name == name {
			continue
		}
		projects = append(projects, project)
	}
	state.Projects = projects
	if state.CurrentProject == name {
		state.CurrentProject = ""
	}
	*state = normalizeAppState(*state)
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
	state.CurrentSessions = normalizeCurrentSessions(state.Projects, state.CurrentSessions)
	return state
}

func normalizeCurrentSessions(projects []projectState, current map[string]string) map[string]string {
	if len(current) == 0 {
		return nil
	}
	projectSessions := map[string][]sessionState{}
	for _, project := range projects {
		projectSessions[normalizeProjectName(project.Name)] = project.Sessions
	}
	normalized := map[string]string{}
	for projectName, sessionName := range current {
		projectName = normalizeProjectName(projectName)
		sessionName = sanitizeSessionName(sessionName)
		if projectName == "" || sessionName == "" {
			continue
		}
		for _, candidate := range projectSessions[projectName] {
			if candidate.Name == sessionName {
				normalized[projectName] = sessionName
				break
			}
		}
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
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
	if strings.TrimSpace(session.CWD) != "" {
		session.CWD = normalizeCWD(session.CWD)
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
	start := 0
	if len(bootstrapProjectAnimals) > 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(bootstrapProjectAnimals)))); err == nil {
			start = int(n.Int64())
		}
	}
	for offset := 0; offset < len(bootstrapProjectAnimals); offset++ {
		animal := bootstrapProjectAnimals[(start+offset)%len(bootstrapProjectAnimals)]
		if _, ok := used[animal]; !ok {
			return animal
		}
	}
	for i := 2; ; i++ {
		for offset := 0; offset < len(bootstrapProjectAnimals); offset++ {
			animal := bootstrapProjectAnimals[(start+offset)%len(bootstrapProjectAnimals)]
			name := fmt.Sprintf("%s-%d", animal, i)
			if _, ok := used[name]; !ok {
				return name
			}
		}
	}
}

func nextVolatileProjectName(state appState) string {
	used := map[string]struct{}{}
	for _, project := range state.Projects {
		used[project.Name] = struct{}{}
	}
	start := 0
	if len(bootstrapProjectAnimals) > 0 {
		if n, err := rand.Int(rand.Reader, big.NewInt(int64(len(bootstrapProjectAnimals)))); err == nil {
			start = int(n.Int64())
		}
	}
	for offset := 0; offset < len(bootstrapProjectAnimals); offset++ {
		animal := bootstrapProjectAnimals[(start+offset)%len(bootstrapProjectAnimals)]
		if _, ok := used[animal]; !ok {
			return animal
		}
	}
	for i := 2; ; i++ {
		for offset := 0; offset < len(bootstrapProjectAnimals); offset++ {
			animal := bootstrapProjectAnimals[(start+offset)%len(bootstrapProjectAnimals)]
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
