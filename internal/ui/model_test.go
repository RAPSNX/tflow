package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPrepareStartupBootstrapsProjectAndCodeSession(t *testing.T) {
	configHome := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	var created []string
	var synced map[string]sessionMetadata
	manager := fakeTmuxController{
		ensureControlMode: func(binaryPath string) error { return nil },
		createSession: func(name, cwd, command string) (session, error) {
			created = append(created, fmt.Sprintf("%s|%s|%s", name, cwd, command))
			return session{Name: name}, nil
		},
		syncSessionMetadata: func(metadata map[string]sessionMetadata) error {
			synced = metadata
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if name == "" {
		t.Fatal("expected startup session name")
	}
	if len(created) != 1 {
		t.Fatalf("created = %#v, want one session", created)
	}
	if len(synced) != 1 {
		t.Fatalf("synced = %#v, want one metadata entry", synced)
	}

	state, err := loadAppState(filepath.Join(configHome, "tflow", "state.json"))
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if len(state.Projects) != 1 {
		t.Fatalf("projects = %#v, want one", state.Projects)
	}
	if len(state.Projects[0].Sessions) != 1 || state.Projects[0].Sessions[0].Name != defaultSessionName {
		t.Fatalf("unexpected bootstrap state: %#v", state)
	}
}

func TestBuildHintsUsesShortestUniquePrefixes(t *testing.T) {
	hints := buildHints([]string{"tiger", "table", "mouse"})
	if hints["tiger"] != "ti" || hints["table"] != "ta" || hints["mouse"] != "m" {
		t.Fatalf("unexpected hints: %#v", hints)
	}
}

func TestProjectOverlayExcludesCurrentProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects:       []projectState{{Name: "garden"}, {Name: "alpha"}, {Name: "mouse"}},
	}

	m.openProjectOverlay(overlaySwitchProject)
	if len(m.overlay.Targets) != 2 {
		t.Fatalf("targets = %#v, want two", m.overlay.Targets)
	}
	for _, target := range m.overlay.Targets {
		if target.Project == "garden" {
			t.Fatalf("current project leaked into overlay: %#v", m.overlay.Targets)
		}
	}
}

func TestProjectOverlayNStartsProjectCreation(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.state = appState{CurrentProject: "garden", Projects: []projectState{{Name: "garden"}}}
	m.openProjectOverlay(overlaySwitchProject)

	updated, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(model)
	if got.mode != inputCreateProject {
		t.Fatalf("mode = %v, want inputCreateProject", got.mode)
	}
}

func TestMoveSelectedSessionRenamesTmuxAndUpdatesState(t *testing.T) {
	var renames [][2]string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renames = append(renames, [2]string{oldName, newName})
			return nil
		},
	}, "garden_code", "").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{
			{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code", Type: sessionTypeTerminal}}},
			{Name: "mouse"},
		},
	}
	m.selectedSession = "code"

	_, cmd := m.moveSelectedSession("mouse")
	msg := cmd().(sessionMovedMsg)
	if msg.err != nil {
		t.Fatalf("move returned error: %v", msg.err)
	}
	m.applyMoveSelectedSession("mouse")
	if got := fmt.Sprint(renames); got != fmt.Sprint([][2]string{{"garden_code", "mouse_code"}}) {
		t.Fatalf("renames = %s", got)
	}
	if m.currentProjectName() != "mouse" {
		t.Fatalf("currentProject = %q, want mouse", m.currentProjectName())
	}
	if session := m.findSession("mouse", "code"); session == nil || session.TmuxName != "mouse_code" {
		t.Fatalf("moved session not found: %#v", m.state)
	}
}

func TestTerminateCurrentProjectKillsAllSessions(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "garden_code", "").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{{
			Name:     "garden",
			Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}, {Name: "shell", TmuxName: "garden_shell"}},
		}},
	}

	_, cmd := m.terminateCurrentProject()
	msg := cmd().(projectTerminatedMsg)
	if msg.err != nil {
		t.Fatalf("terminate returned error: %v", msg.err)
	}
	if got := fmt.Sprint(killed); got != fmt.Sprint([]string{"garden_code", "garden_shell"}) {
		t.Fatalf("killed = %s", got)
	}
}

func TestSwitchProjectClosesPaneForNonEmptyTarget(t *testing.T) {
	var switched []string
	var closed []string
	m := newModel(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
		closePane: func(paneID string) error {
			closed = append(closed, paneID)
			return nil
		},
	}, "garden_code", "%3").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{
			{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
			{Name: "mouse", Sessions: []sessionState{{Name: "code", TmuxName: "mouse_code"}}},
		},
	}

	_, cmd := m.switchProject("mouse")
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("switch returned error: %v", msg.err)
	}
	if got := fmt.Sprint(switched); got != fmt.Sprint([]string{"mouse_code"}) {
		t.Fatalf("switched = %s", got)
	}
	if got := fmt.Sprint(closed); got != fmt.Sprint([]string{"%3"}) {
		t.Fatalf("closed = %s", got)
	}
}

type fakeTmuxController struct {
	listSessions        func() ([]session, error)
	createSession       func(name, cwd, command string) (session, error)
	setSessionTemporary func(name string, temporary bool) error
	attachCommand       func(name string) (*exec.Cmd, error)
	killSession         func(name string) error
	renameSession       func(oldName, newName string) error
	switchClient        func(name string) error
	ensureControlMode   func(binaryPath string) error
	syncSessionMetadata func(metadata map[string]sessionMetadata) error
	toggleMenu          func(binaryPath string) error
	closePane           func(paneID string) error
	quitAll             func(paneID string) error
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
	return session{Name: name}, nil
}

func (f fakeTmuxController) SetSessionTemporary(name string, temporary bool) error {
	if f.setSessionTemporary != nil {
		return f.setSessionTemporary(name, temporary)
	}
	return nil
}

func (f fakeTmuxController) AttachCommand(name string) (*exec.Cmd, error) {
	if f.attachCommand != nil {
		return f.attachCommand(name)
	}
	return nil, nil
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

func (f fakeTmuxController) SyncSessionMetadata(metadata map[string]sessionMetadata) error {
	if f.syncSessionMetadata != nil {
		return f.syncSessionMetadata(metadata)
	}
	return nil
}

func (f fakeTmuxController) ToggleMenu(binaryPath string) error {
	if f.toggleMenu != nil {
		return f.toggleMenu(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) ClosePane(paneID string) error {
	if f.closePane != nil {
		return f.closePane(paneID)
	}
	return nil
}

func (f fakeTmuxController) QuitAll(paneID string) error {
	if f.quitAll != nil {
		return f.quitAll(paneID)
	}
	return nil
}
