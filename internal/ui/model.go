package ui

import (
	"os"
	"strings"

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

type sessionsLoadedMsg struct {
	sessions []session
	err      error
}

type sessionCreatedMsg struct {
	session session
	kind    sessionType
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

type projectDeletedMsg struct {
	project string
	err     error
}

type menuActionMsg struct {
	err error
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
	sessionTypes     map[string]sessionType
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
	sessionKindAgent
)

type sessionType string

const (
	sessionTypeTerminal sessionType = "terminal"
	sessionTypeK9s      sessionType = "k9s"
	sessionTypeAgent    sessionType = "agent"
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
	existing, err := manager.ListSessions()
	if err != nil {
		return "", err
	}
	name := nextTempSessionName(existing)
	if _, err := manager.CreateSession(name, cwd, ""); err != nil {
		return "", err
	}
	if err := manager.SetSessionTemporary(name, true); err != nil {
		return "", err
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return "", err
	}
	if err := ensureStartupState(); err != nil {
		return "", err
	}
	return name, nil
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
			Projects:         []string{},
			SessionProjects:  map[string]string{},
			SessionTypes:     map[string]string{},
			ProjectDirs:      map[string]string{},
			ExpandedProjects: map[string]bool{},
		}
	}
	state = normalizeAppState(state)
	projectConfigs, configErr := loadProjectConfigs(statePath, state)
	if configErr != nil {
		projectConfigs = map[string]projectConfig{}
		if err == nil {
			err = configErr
		}
	}
	if err == nil {
		err = nil
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
		sessionTypes:     normalizeSessionTypes(state.SessionTypes),
		projectConfigs:   projectConfigs,
		expandedProjects: state.ExpandedProjects,
		selectedProject:  "",
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
