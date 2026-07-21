package ui

import (
	"context"
	"os"
	"os/exec"

	runtmux "github.com/rapsnx/tflow/internal/tmux"
)

const (
	menuCurrentEnv  = runtmux.CurrentSessionEnv
	menuClientEnv   = runtmux.CurrentClientEnv
	menuInstanceEnv = runtmux.CurrentInstanceEnv
)

type session = runtmux.Session

type tmuxController interface {
	ListSessions() ([]session, error)
	CreateSession(name, cwd, command string) (session, error)
	RenameSession(oldName, newName string) error
	SetSessionProject(name, project string) error
	RunBackground(command string) error
	DisplayMessage(message string) error
	CurrentPaneDir() (string, error)
	SetSessionTemporary(name string, temporary bool, instanceID string) error
	SetSessionLabel(name, label string) error
	AttachCommand(ctx context.Context, name string) (*exec.Cmd, error)
	KillSession(name string) error
	SessionPanesAllDead(name string) (bool, error)
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string) error
	ToggleMenu(binaryPath string) error
	CloseMenu() error
	QuitAll() error
	CleanupVolatileSessions(instanceID string) error
}

type sessionManager struct {
	inner runtmux.Controller
}

func newSessionManager() tmuxController {
	return sessionManager{inner: runtmux.New()}
}

func ToggleMenu() error {
	return runtmux.ToggleMenu()
}

func OpenQuit() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return runtmux.New().OpenQuit(exe)
}

func (m sessionManager) ListSessions() ([]session, error) {
	return m.inner.ListSessions()
}

func (m sessionManager) CreateSession(name, cwd, command string) (session, error) {
	return m.inner.CreateSession(name, cwd, command)
}
func (m sessionManager) RenameSession(oldName, newName string) error {
	return m.inner.RenameSession(oldName, newName)
}
func (m sessionManager) SetSessionProject(name, project string) error {
	return m.inner.SetSessionProject(name, project)
}
func (m sessionManager) RunBackground(command string) error  { return m.inner.RunBackground(command) }
func (m sessionManager) DisplayMessage(message string) error { return m.inner.DisplayMessage(message) }
func (m sessionManager) CurrentPaneDir() (string, error)     { return m.inner.CurrentPaneDir() }

func (m sessionManager) SetSessionTemporary(name string, temporary bool, instanceID string) error {
	return m.inner.SetSessionTemporary(name, temporary, instanceID)
}

func (m sessionManager) SetSessionLabel(name, label string) error {
	return m.inner.SetSessionLabel(name, label)
}

func (m sessionManager) AttachCommand(ctx context.Context, name string) (*exec.Cmd, error) {
	return m.inner.AttachCommand(ctx, name)
}

func (m sessionManager) KillSession(name string) error {
	return m.inner.KillSession(name)
}

func (m sessionManager) SessionPanesAllDead(name string) (bool, error) {
	return m.inner.SessionPanesAllDead(name)
}

func (m sessionManager) SwitchClient(name string) error {
	return m.inner.SwitchClient(name)
}

func (m sessionManager) EnsureControlMode(binaryPath string) error {
	palette := catppuccinPalette()
	return m.inner.EnsureControlMode(binaryPath, runtmux.Palette{
		Surface0: palette.Surface0,
		Subtext:  palette.Subtext,
		Text:     palette.Text,
		Blue:     palette.Blue,
		Mantle:   palette.Mantle,
		Teal:     palette.Teal,
	})
}

func (m sessionManager) ToggleMenu(binaryPath string) error {
	return m.inner.ToggleMenu(binaryPath)
}

func (m sessionManager) CloseMenu() error {
	return m.inner.CloseMenu()
}

func (m sessionManager) QuitAll() error {
	return m.inner.QuitAll()
}

func (m sessionManager) CleanupVolatileSessions(instanceID string) error {
	return m.inner.CleanupVolatileSessions(instanceID)
}

// sanitizeSessionName normalizes a user-entered session label. Labels
// preserve their exact casing and characters; only surrounding whitespace is
// trimmed. This is distinct from tmux's internal slug-shaped session names.
func sanitizeSessionName(name string) string {
	return runtmux.NormalizeSessionLabel(name)
}

func shellQuote(value string) string {
	return runtmux.ShellQuote(value)
}

func nextTempSessionName(existing []session) string {
	return runtmux.NextTempSessionName(existing)
}

func nextTempSessionNameForInstance(existing []session, instanceID string) string {
	return runtmux.NextTempSessionNameForInstance(existing, instanceID)
}

func volatileSessionName(instanceID, id string) string {
	return runtmux.VolatileSessionName(instanceID, id)
}

func persistentSessionName(id string) string {
	return runtmux.PersistentSessionName(id)
}

func randomAnimalName() string {
	return runtmux.RandomAnimalName()
}

func isSessionExists(err error) bool {
	return runtmux.IsSessionExists(err)
}
