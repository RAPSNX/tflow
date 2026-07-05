package ui

import (
	"fmt"
	"os"
	"os/exec"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuStartsWithCurrentSessionSelected(t *testing.T) {
	m := NewMenu().(model)
	m.currentSession = "dev"
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.syncSelection()

	if m.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", m.selectedSession)
	}
}

func TestPrepareStartupCreatesSessionBeforeControlMode(t *testing.T) {
	tmp := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmp)

	var calls []string
	manager := fakeTmuxController{
		createSession: func(name, cwd string) (session, error) {
			calls = append(calls, "create:"+name)
			return session{Name: name}, nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "control:"+binaryPath)
			return nil
		},
	}

	if err := prepareStartup(manager, "/tmp/tflow", "/tmp/project"); err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}

	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{"create:default", "control:/tmp/tflow"}); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestMenuEnterSwitchesSessionAndClosesPane(t *testing.T) {
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
	}, "dev", "%3").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("menu action returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(switched), fmt.Sprint([]string{"dev"}); got != want {
		t.Fatalf("switches = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(closed), fmt.Sprint([]string{"%3"}); got != want {
		t.Fatalf("closed = %s, want %s", got, want)
	}
}

func TestCtrlCClosesMenuPane(t *testing.T) {
	closed := ""
	m := newModel(fakeTmuxController{
		closePane: func(paneID string) error {
			closed = paneID
			return nil
		},
	}, "", "%3").(model)

	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if closed != "%3" {
		t.Fatalf("closed = %q, want %%3", closed)
	}
}

func TestMoveProjectUsesIncrementalPrefix(t *testing.T) {
	m := NewMenu().(model)
	m.sessions = []session{{Name: "dev"}}
	m.projects = []string{defaultProjectName, "small", "storage"}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true, "storage": true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.mode = inputMoveProject
	m.moveQuery = "sm"

	updated, cmd := m.resolveMoveProject()
	got := updated.(model)
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if cmd == nil {
		t.Fatal("expected move command")
	}
	msg := cmd().(sessionMovedMsg)
	if msg.project != "small" || msg.session != "dev" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestDeleteProjectMovesSessionsToDefault(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"

	updated, cmd := m.deleteSelectedProject()
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain deleted project: %#v", got.projects)
	}
	if got.sessionProjects["dev"] != defaultProjectName {
		t.Fatalf("sessionProjects[dev] = %q, want %q", got.sessionProjects["dev"], defaultProjectName)
	}
	if got.selectedProject != defaultProjectName {
		t.Fatalf("selectedProject = %q, want %q", got.selectedProject, defaultProjectName)
	}
}

func TestDeleteProjectRejectsDefault(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.selectedProject = defaultProjectName

	updated, cmd := m.deleteSelectedProject()
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if !containsString(got.projects, defaultProjectName) {
		t.Fatalf("default project removed: %#v", got.projects)
	}
}

func TestDefaultSessionDirPrefersHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	_ = os.Setenv("HOME", "/tmp/home")

	if got := defaultSessionDir(); got != "/tmp/home" {
		t.Fatalf("defaultSessionDir = %q, want /tmp/home", got)
	}
}

func TestCreateSessionUsesCurrentDirectory(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != m.cwd {
		t.Fatalf("cwd = %q, want %q", gotCWD, m.cwd)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "prod-main-01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestProjectNormalizationKeepsDefaultFirst(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"default", "alpha", "small"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}

type fakeTmuxController struct {
	listSessions      func() ([]session, error)
	createSession     func(name, cwd string) (session, error)
	attachCommand     func(name string) (*exec.Cmd, error)
	killSession       func(name string) error
	switchClient      func(name string) error
	ensureControlMode func(binaryPath string) error
	toggleMenu        func(binaryPath string) error
	closePane         func(paneID string) error
}

func (f fakeTmuxController) ListSessions() ([]session, error) {
	if f.listSessions != nil {
		return f.listSessions()
	}
	return nil, nil
}

func (f fakeTmuxController) CreateSession(name, cwd string) (session, error) {
	if f.createSession != nil {
		return f.createSession(name, cwd)
	}
	return session{Name: name, Windows: 1}, nil
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

func (f fakeTmuxController) ClosePane(paneID string) error {
	if f.closePane != nil {
		return f.closePane(paneID)
	}
	return nil
}
