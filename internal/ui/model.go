package ui

import (
	cryptorand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type inputMode int

const (
	inputNone inputMode = iota
	inputCreateSession
	inputCreatingSession
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
	label   string
	err     error
}

type sessionRenamedMsg struct {
	name     string
	label    string
	volatile bool
	err      error
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

	showHelp bool

	sessions               []session
	projects               []string
	persistentSessionOrder map[string][]string
	sessionProjects        map[string]string
	sessionLabels          map[string]string
	projectConfigs         map[string]projectConfig
	selectedProject        string
	selectedSession        string
	currentSession         string

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

const (
	startupSessionRetryLimit = 128
	sessionIDRetryLimit      = 128
)

func NewMenu() tea.Model {
	return newModel(newSessionManager(), os.Getenv(menuCurrentEnv))
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
	projects := make([]string, 0, len(state.Projects))
	persistentSessionOrder := map[string][]string{}
	sessionProjects := map[string]string{}
	sessionLabels := map[string]string{}
	projectConfigs := map[string]projectConfig{}
	for _, project := range state.Projects {
		projects = append(projects, project.Name)
		projectConfigs[project.Name] = projectConfig{Name: project.Name, Workdir: project.Workdir}
		for _, session := range project.Sessions {
			persistentSessionOrder[project.Name] = append(persistentSessionOrder[project.Name], session.ID)
			sessionProjects[session.ID] = project.Name
			sessionLabels[session.ID] = session.Label
		}
	}
	return model{
		tmux:                   manager,
		instanceID:             os.Getenv(menuInstanceEnv),
		mode:                   inputNone,
		projects:               projects,
		persistentSessionOrder: persistentSessionOrder,
		sessionProjects:        sessionProjects,
		sessionLabels:          sessionLabels,
		projectConfigs:         projectConfigs,
		selectedProject:        "",
		currentSession:         current,
		input:                  input,
		cwd:                    cwd,
		statePath:              statePath,
		status:                 "",
		err:                    nil,
	}, nil
}

func (m model) Init() tea.Cmd {
	return m.loadSessionsCmd()
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

func newSessionID() (string, error) {
	return newSessionIDWithEntropy(cryptorand.Reader)
}

func newSessionIDWithEntropy(entropy io.Reader) (string, error) {
	return randomInstanceToken(entropy, 16)
}

func (m model) createPersistentSession(cwd, command string) (session, error) {
	return m.createGeneratedSession(cwd, command, persistentSessionName)
}

func (m model) createVolatileSession(cwd, command, label string) (session, error) {
	created, err := m.createGeneratedSession(cwd, command, func(id string) string {
		return volatileSessionName(m.instanceID, id)
	})
	if err != nil {
		return session{}, err
	}
	if err := m.tmux.SetSessionTemporary(created.Name, true, m.instanceID); err != nil {
		_ = m.tmux.KillSession(created.Name)
		return session{}, fmt.Errorf("mark volatile session: %w", err)
	}
	if err := m.tmux.SetSessionLabel(created.Name, label); err != nil {
		_ = m.tmux.KillSession(created.Name)
		return session{}, fmt.Errorf("set volatile session label: %w", err)
	}
	created.Temporary = true
	created.Instance = m.instanceID
	created.Label = label
	return created, nil
}

func (m model) createGeneratedSession(cwd, command string, nameForID func(string) string) (session, error) {
	for attempt := 0; attempt < sessionIDRetryLimit; attempt++ {
		id, err := newSessionID()
		if err != nil {
			return session{}, fmt.Errorf("generate session id: %w", err)
		}
		name := nameForID(id)
		if name == "" {
			return session{}, fmt.Errorf("generate session name")
		}
		if _, exists := m.findSession(name); exists {
			continue
		}
		if _, exists := m.sessionProjects[name]; exists {
			continue
		}
		created, err := m.tmux.CreateSession(name, cwd, command)
		if err == nil {
			return created, nil
		}
		if !isSessionExists(err) {
			return session{}, err
		}
	}
	return session{}, fmt.Errorf("allocate generated session after %d retries", sessionIDRetryLimit)
}
