package ui

import (
	"os/exec"

	runtmux "tflow/internal/tmux"
)

const (
	menuCurrentEnv = runtmux.CurrentSessionEnv
	menuClientEnv  = runtmux.CurrentClientEnv
)

type session = runtmux.Session

type tmuxController interface {
	ListSessions() ([]session, error)
	CreateSession(name, cwd, command string) (session, error)
	SetSessionTemporary(name string, temporary bool) error
	AttachCommand(name string) (*exec.Cmd, error)
	KillSession(name string) error
	RenameSession(oldName, newName string) error
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string) error
	SyncSessionProjects(sessionProjects map[string]string) error
	ToggleMenu(binaryPath string) error
	ClosePane(paneID string) error
	QuitAll(paneID string) error
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

func (m sessionManager) ListSessions() ([]session, error) {
	return m.inner.ListSessions()
}

func (m sessionManager) CreateSession(name, cwd, command string) (session, error) {
	return m.inner.CreateSession(name, cwd, command)
}

func (m sessionManager) SetSessionTemporary(name string, temporary bool) error {
	return m.inner.SetSessionTemporary(name, temporary)
}

func (m sessionManager) AttachCommand(name string) (*exec.Cmd, error) {
	return m.inner.AttachCommand(name)
}

func (m sessionManager) KillSession(name string) error {
	return m.inner.KillSession(name)
}

func (m sessionManager) RenameSession(oldName, newName string) error {
	return m.inner.RenameSession(oldName, newName)
}

func (m sessionManager) SwitchClient(name string) error {
	return m.inner.SwitchClient(name)
}

func (m sessionManager) EnsureControlMode(binaryPath string) error {
	palette := themePaletteForName("catppuccin")
	return m.inner.EnsureControlMode(binaryPath, runtmux.Palette{
		Surface0: palette.Surface0,
		Subtext:  palette.Subtext,
		Text:     palette.Text,
		Blue:     palette.Blue,
		Mantle:   palette.Mantle,
		Teal:     palette.Teal,
	})
}

func (m sessionManager) SyncSessionProjects(sessionProjects map[string]string) error {
	return m.inner.SyncSessionProjects(sessionProjects)
}

func (m sessionManager) ToggleMenu(binaryPath string) error {
	return m.inner.ToggleMenu(binaryPath)
}

func (m sessionManager) ClosePane(paneID string) error {
	return m.inner.ClosePane(paneID)
}

func (m sessionManager) QuitAll(paneID string) error {
	return m.inner.QuitAll(paneID)
}

func normalizeCWD(cwd string) string {
	return runtmux.NormalizeCWD(cwd)
}

func sanitizeSessionName(name string) string {
	return runtmux.SanitizeSessionName(name)
}

func shellQuote(value string) string {
	return runtmux.ShellQuote(value)
}

func nextTempSessionName(existing []session) string {
	return runtmux.NextTempSessionName(existing)
}
