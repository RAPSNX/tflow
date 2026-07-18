package ui

import (
	"os"
	"os/exec"
	"testing"
)

const defaultProjectName = "default"

type fakeTmuxController struct {
	listSessions        func() ([]session, error)
	createSession       func(name, cwd, command string) (session, error)
	setSessionTemporary func(name string, temporary bool, instanceID string) error
	attachCommand       func(name string) (*exec.Cmd, error)
	killSession         func(name string) error
	renameSession       func(oldName, newName string) error
	switchClient        func(name string) error
	ensureControlMode   func(binaryPath string) error
	toggleMenu          func(binaryPath string) error
	closeMenu           func() error
	quitAll             func() error
	cleanupVolatile     func(instanceID string) error
	syncSessionProjects func(sessionProjects map[string]string) error
}

func (f fakeTmuxController) ListSessions() ([]session, error) {
	if f.listSessions != nil {
		return f.listSessions()
	}
	return nil, nil
}

func (f fakeTmuxController) CreateSession(name, cwd, command string) (session, error) {
	if f.createSession != nil {
		return f.createSession(name, cwd, command)
	}
	return session{Name: name, Windows: 1}, nil
}

func (f fakeTmuxController) SetSessionTemporary(name string, temporary bool, instanceID string) error {
	if f.setSessionTemporary != nil {
		return f.setSessionTemporary(name, temporary, instanceID)
	}
	return nil
}

func (f fakeTmuxController) AttachCommand(name string) (*exec.Cmd, error) {
	if f.attachCommand != nil {
		return f.attachCommand(name)
	}
	return exec.Command("sh", "-lc", ":"), nil
}

func (f fakeTmuxController) KillSession(name string) error {
	if f.killSession != nil {
		return f.killSession(name)
	}
	return nil
}

func (f fakeTmuxController) RenameSession(oldName, newName string) error {
	if f.renameSession != nil {
		return f.renameSession(oldName, newName)
	}
	return nil
}

func (f fakeTmuxController) SwitchClient(name string) error {
	if f.switchClient != nil {
		return f.switchClient(name)
	}
	return nil
}

func (f fakeTmuxController) EnsureControlMode(binaryPath string) error {
	if f.ensureControlMode != nil {
		return f.ensureControlMode(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) ToggleMenu(binaryPath string) error {
	if f.toggleMenu != nil {
		return f.toggleMenu(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) CloseMenu() error {
	if f.closeMenu != nil {
		return f.closeMenu()
	}
	return nil
}

func (f fakeTmuxController) QuitAll() error {
	if f.quitAll != nil {
		return f.quitAll()
	}
	return nil
}

func (f fakeTmuxController) CleanupVolatileSessions(instanceID string) error {
	if f.cleanupVolatile != nil {
		return f.cleanupVolatile(instanceID)
	}
	return nil
}

func (f fakeTmuxController) SyncSessionProjects(sessionProjects, sessionLabels map[string]string) error {
	if f.syncSessionProjects != nil {
		return f.syncSessionProjects(sessionProjects)
	}
	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func TestMain(m *testing.M) {
	stateHome, err := os.MkdirTemp("", "tflow-ui-state-")
	if err != nil {
		panic(err)
	}
	configHome, err := os.MkdirTemp("", "tflow-ui-config-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(stateHome)
	defer os.RemoveAll(configHome)

	if err := os.Setenv("XDG_STATE_HOME", stateHome); err != nil {
		panic(err)
	}
	if err := os.Setenv("XDG_CONFIG_HOME", configHome); err != nil {
		panic(err)
	}

	os.Exit(m.Run())
}
