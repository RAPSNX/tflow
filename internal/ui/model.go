package ui

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	runtmux "tflow/internal/tmux"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputHelp
	inputCreateSession
	inputCreateProject
	inputSwitchProject
	inputConfirmDelete
	inputConfirmProjectSwitch
	inputConfirmQuit
	inputRename
	inputEditProject
)

type sessionsLoadedMsg struct {
	sessions []session
	err      error
}

type sessionCreatedMsg struct {
	session  session
	volatile bool
	kind     sessionType
	project  string
	label    string
	err      error
}

type sessionKilledMsg struct {
	name    string
	project string
	err     error
}

type projectCreatedMsg struct {
	config  projectConfig
	session session
	err     error
}

type sessionRenamedMsg struct {
	oldName string
	newName string
	label   string
	err     error
}

type sessionNamesMigratedMsg struct {
	renames []sessionRename
	err     error
}

type projectRenamedMsg struct {
	oldName        string
	newName        string
	sessionRenames []sessionRename
	err            error
}

type sessionRename struct {
	oldName string
	newName string
}

type projectDeletedMsg struct {
	project string
	err     error
}

type menuActionMsg struct {
	err           error
	switchSession string
	quit          bool
}

type menuExitAction int

const (
	menuExitNone menuExitAction = iota
	menuExitSwitchSession
	menuExitQuit
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

	instanceID string

	width  int
	height int

	mode inputMode

	sessions        []session
	projects        []string
	sessionProjects map[string]string
	sessionTypes    map[string]sessionType
	sessionLabels   map[string]string
	projectConfigs  map[string]projectConfig
	selectedProject string
	selectedSession string
	currentSession  string

	input               textinput.Model
	renameTarget        renameTarget
	deleteTarget        deleteTarget
	switchProjectTarget string
	projectSwitchIndex  int
	projectEditConfig   projectConfig

	cwd       string
	statePath string

	exitAction      menuExitAction
	exitSessionName string
	status          string
	err             error
}

type sessionType string

const (
	sessionTypeTerminal       sessionType = "terminal"
	sessionTypeK9s            sessionType = "k9s"
	sessionTypeAgent          sessionType = "agent"
	defaultProjectSessionName             = "code"
	startupSessionRetryLimit              = 128
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
		cleanupErr := manager.CleanupVolatileSessions(instanceID)
		if cleanupErr != nil {
			return fmt.Errorf("attach command: %w; cleanup volatile sessions: %v", err, cleanupErr)
		}
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
	if os.Getenv(runtmux.MenuModeEnv) == runtmux.MenuModeQuit {
		menu.mode = inputConfirmQuit
		menu.status = "Confirm shutdown of this tflow instance."
	}
	finalModel, err := tea.NewProgram(menu, tea.WithAltScreen()).Run()
	if err != nil {
		return err
	}
	return runMenuExitAction(newSessionManager(), finalModel)
}

func prepareStartup(manager tmuxController, binaryPath, cwd, instanceID string) (string, error) {
	if err := ensureStartupState(); err != nil {
		return "", fmt.Errorf("initialize state %q: %w", appStatePath(), err)
	}

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
		return "", rollbackStartupSession(manager, name, fmt.Errorf("tag startup session: %w", err))
	}
	if err := manager.EnsureControlMode(binaryPath); err != nil {
		return "", rollbackStartupSession(manager, name, fmt.Errorf("prepare tmux control mode: %w", err))
	}
	return name, nil
}

func rollbackStartupSession(manager tmuxController, name string, startupErr error) error {
	if err := manager.KillSession(name); err != nil {
		return fmt.Errorf("%w; rollback startup session %q: %v", startupErr, name, err)
	}
	return startupErr
}

func defaultSessionDir() string {
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return cwd
	}
	return runtmux.NormalizeCWD("")
}

func newModel(manager tmuxController, current string) tea.Model {
	menu, err := buildModel(manager, current)
	if err != nil {
		menu.err = err
		menu.status = err.Error()
	}
	return menu
}

func buildModel(manager tmuxController, current string) (model, error) {
	cwd := defaultSessionDir()

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
		instanceID:      os.Getenv(menuInstanceEnv),
		mode:            inputNone,
		projects:        state.Projects,
		sessionProjects: state.SessionProjects,
		sessionTypes:    normalizeSessionTypes(state.SessionTypes),
		sessionLabels:   state.SessionLabels,
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
	case menuExitQuit:
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
	return newInstanceIDWithEntropy(time.Now(), cryptorand.Reader, os.Getpid())
}

func newInstanceIDWithEntropy(now time.Time, entropy io.Reader, pid int) string {
	token, err := randomInstanceToken(entropy, 6)
	if err != nil {
		token = strconv.Itoa(pid)
	}
	return "tflow-" + strconv.FormatInt(now.UnixNano(), 36) + "-" + token
}

func randomInstanceToken(entropy io.Reader, size int) (string, error) {
	buf := make([]byte, size)
	if _, err := io.ReadFull(entropy, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
