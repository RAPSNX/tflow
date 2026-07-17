package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputNew
	inputCreateSession
	inputCreateProject
	inputSwitchProject
	inputConfirmDelete
	inputConfirmProjectSwitch
	inputRename
	inputEditProject
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

type projectDeletedMsg struct {
	project string
	err     error
}

type menuActionMsg struct {
	err           error
	switchSession string
	quitAll       bool
}

type menuExitAction int

const (
	menuExitNone menuExitAction = iota
	menuExitSwitchSession
	menuExitQuitAll
)

type renameTarget struct {
	project string
	session string
}

type deleteTarget struct {
	project string
	session string
}

type model struct {
	tmux tmuxController

	width  int
	height int

	mode inputMode

	sessions        []session
	projects        []string
	sessionProjects map[string]string
	sessionTypes    map[string]sessionType
	projectConfigs  map[string]projectConfig
	selectedProject string
	selectedSession string
	currentSession  string

	input               textinput.Model
	renameTarget        renameTarget
	deleteTarget        deleteTarget
	switchProjectTarget string
	createSessionKind   sessionKind
	projectEditConfig   projectConfig
	projectEditField    projectEditField

	cwd       string
	statePath string

	exitAction      menuExitAction
	exitSessionName string
	status          string
	err             error
}

type sessionKind int

const (
	sessionKindTerminal sessionKind = iota
	sessionKindK9s
	sessionKindAgent
)

type sessionType string

const (
	sessionTypeTerminal      sessionType = "terminal"
	sessionTypeK9s           sessionType = "k9s"
	sessionTypeAgent         sessionType = "agent"
	startupSessionRetryLimit             = 128
)

func NewMenu() tea.Model {
	return newModel(newSessionManager(), os.Getenv(menuCurrentEnv))
}

func Start() error {
	cwd := defaultSessionDir()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return startWithManager(newSessionManager(), exe, cwd, newInstanceID())
}

func startWithManager(manager tmuxController, binaryPath, cwd, instanceID string) error {
	if err := os.Setenv(menuInstanceEnv, instanceID); err != nil {
		return err
	}
	sessionName, err := prepareStartup(manager, binaryPath, cwd, instanceID)
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
	cleanupErr := manager.CleanupVolatileSessions(instanceID)
	if runErr != nil {
		if cleanupErr != nil {
			return fmt.Errorf("attach session: %w; cleanup volatile sessions: %v", runErr, cleanupErr)
		}
		return runErr
	}
	return cleanupErr
}

func OpenMenu() error {
	menu, err := buildModel(newSessionManager(), os.Getenv(menuCurrentEnv))
	if err != nil {
		return err
	}
	finalModel, err := tea.NewProgram(menu, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	return runMenuExitAction(newSessionManager(), finalModel)
}

func prepareStartup(manager tmuxController, binaryPath, cwd, instanceID string) (string, error) {
	existing, err := manager.ListSessions()
	if err != nil {
		return "", err
	}
	var name string
	created := false
	for attempts := 0; attempts < startupSessionRetryLimit; attempts++ {
		name = nextTempSessionName(existing)
		if _, err := manager.CreateSession(name, cwd, ""); err != nil {
			if !isSessionExists(err) {
				return "", err
			}
			existing = append(existing, session{Name: name})
			continue
		}
		created = true
		break
	}
	if !created {
		return "", fmt.Errorf("allocate startup session after %d retries", startupSessionRetryLimit)
	}
	if err := manager.SetSessionTemporary(name, true, instanceID); err != nil {
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

func newModel(manager tmuxController, current string, _ ...string) tea.Model {
	menu, err := buildModel(manager, current)
	if err != nil {
		menu.err = err
		menu.status = err.Error()
	}
	return menu
}

func buildModel(manager tmuxController, current string, _ ...string) (model, error) {
	cwd, _ := os.Getwd()

	input := textinput.New()
	input.CharLimit = 40
	input.Width = 28
	input.Blur()

	statePath := appStatePath()
	state, err := loadAppState(statePath)
	if err != nil {
		return model{}, fmt.Errorf("load state %q: %w", statePath, err)
	}
	state = normalizeAppState(state)
	return model{
		tmux:            manager,
		mode:            inputNone,
		projects:        state.Projects,
		sessionProjects: state.SessionProjects,
		sessionTypes:    normalizeSessionTypes(state.SessionTypes),
		projectConfigs:  state.ProjectConfigs,
		selectedProject: "",
		currentSession:  current,
		input:           input,
		cwd:             cwd,
		statePath:       statePath,
		status:          "",
		err:             nil,
	}, nil
}

func (m model) Init() tea.Cmd {
	return m.loadSessionsCmd()
}

func runMenuExitAction(manager tmuxController, final tea.Model) error {
	menu, ok := unwrapMenuModel(final)
	if !ok {
		return nil
	}
	switch menu.exitAction {
	case menuExitSwitchSession:
		if strings.TrimSpace(menu.exitSessionName) == "" {
			return nil
		}
		return manager.SwitchClient(menu.exitSessionName)
	case menuExitQuitAll:
		return manager.QuitAll()
	default:
		return nil
	}
}

func unwrapMenuModel(value tea.Model) (model, bool) {
	switch typed := value.(type) {
	case model:
		return typed, true
	case *model:
		if typed == nil {
			return model{}, false
		}
		return *typed, true
	default:
		return model{}, false
	}
}

func newInstanceID() string {
	return "tflow-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
