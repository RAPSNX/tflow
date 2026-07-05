package ui

import (
	"fmt"
	"os"
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

func TestMenuEnterSelectsSession(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if got.exitSession != "dev" {
		t.Fatalf("exitSession = %q, want dev", got.exitSession)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
	}
}

func TestMenuCtrlFTogglesClosed(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlF})
	got := updated.(model)
	if got.exitSession != "" {
		t.Fatalf("exitSession = %q, want empty", got.exitSession)
	}
	if cmd == nil {
		t.Fatal("expected quit command")
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
	moved := map[string]string{}

	m := newModel(fakeSessionManager{
		setProject: func(name, project string) error {
			moved[name] = project
			return nil
		},
	}, "").(model)
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
	if moved["dev"] != defaultProjectName {
		t.Fatalf("session project move = %q, want %q", moved["dev"], defaultProjectName)
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

func TestCreateSessionUsesSelectedProject(t *testing.T) {
	var gotProject string
	m := newModel(fakeSessionManager{
		createSession: func(name, cwd, project string) (session, error) {
			gotProject = project
			return session{Name: name, Windows: 1}, nil
		},
	}, "").(model)
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
	if gotProject != "small" {
		t.Fatalf("project = %q, want %q", gotProject, "small")
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

type fakeSessionManager struct {
	listSessions  func() ([]session, error)
	createSession func(name, cwd, project string) (session, error)
	killSession   func(name string) error
	setProject    func(name, project string) error
}

func (f fakeSessionManager) ListSessions() ([]session, error) {
	if f.listSessions != nil {
		return f.listSessions()
	}
	return nil, nil
}

func (f fakeSessionManager) CreateSession(name, cwd, project string) (session, error) {
	if f.createSession != nil {
		return f.createSession(name, cwd, project)
	}
	return session{Name: name, Windows: 1}, nil
}

func (f fakeSessionManager) KillSession(name string) error {
	if f.killSession != nil {
		return f.killSession(name)
	}
	return nil
}

func (f fakeSessionManager) SetSessionProject(name, project string) error {
	if f.setProject != nil {
		return f.setProject(name, project)
	}
	return nil
}
