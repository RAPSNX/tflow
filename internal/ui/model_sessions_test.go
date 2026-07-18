package ui

import (
	"fmt"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			if command != "" {
				t.Fatalf("command = %q, want empty", command)
			}
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
	if gotCWD != m.cwd {
		t.Fatalf("cwd = %q, want %q", gotCWD, m.cwd)
	}
}

func TestCreateSessionUsesProjectDirectoryWhenConfigured(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/project-small"}}
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
	if gotCWD != "/tmp/project-small" {
		t.Fatalf("cwd = %q, want /tmp/project-small", gotCWD)
	}
}

func TestCreateSessionRejectsDuplicateLabelWithinProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.input.SetValue("code")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no create command")
	}
	if got.status != "Session name already exists in this project." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestCreateSessionUsesExpandedHomeDirectoryWhenConfigured(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "~/project-small"}}
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
	if gotCWD != "/tmp/home/project-small" {
		t.Fatalf("cwd = %q, want /tmp/home/project-small", gotCWD)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "prod-main-01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestProjectNormalizationPreservesOrderAndDeduplicates(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"small", "default", "alpha"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}

func TestCreateUnscopedSessionIsVolatileAndDoesNotPersistMetadata(t *testing.T) {
	var tagged []string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{Name: name, Windows: 1}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			tagged = append(tagged, fmt.Sprintf("%s:%t:%s", name, temporary, instanceID))
			return nil
		},
	}, "scratch-temp").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.instanceID = "instance-1"
	m.sessions = []session{{Name: "scratch-temp", Temporary: true, Instance: "instance-1"}}
	m.selectedProject = "old-project"
	m.mode = inputCreateSession
	m.input.SetValue("notes")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected session creation command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("session creation returned error: %v", msg.err)
	}
	if !msg.volatile {
		t.Fatal("created unscoped session is not volatile")
	}
	if got, want := fmt.Sprint(tagged), "[notes:true:instance-1]"; got != want {
		t.Fatalf("temporary tags = %s, want %s", got, want)
	}
	got := updated.(model)
	updated, _ = got.Update(msg)
	got = updated.(model)
	if got.selectedProject != "" || got.selectedSession != "notes" {
		t.Fatalf("selection = project %q session %q, want volatile notes", got.selectedProject, got.selectedSession)
	}
	if _, ok := got.sessionProjects["notes"]; ok {
		t.Fatalf("volatile session metadata persisted: %#v", got.sessionProjects)
	}
}

func TestCreateVolatileSessionClearsStaleMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.sessionProjects = map[string]string{"notes": "old-project"}
	m.sessionTypes = map[string]sessionType{"notes": sessionTypeAgent}
	m.sessionLabels = map[string]string{"notes": "old label"}
	m.statePath = t.TempDir() + "/store.json"
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(sessionCreatedMsg{
		session:  session{Name: "notes", Temporary: true, Instance: "instance-1"},
		volatile: true,
	})
	got := updated.(model)

	if _, ok := got.sessionProjects["notes"]; ok {
		t.Fatalf("stale project metadata remains: %#v", got.sessionProjects)
	}
	if _, ok := got.sessionTypes["notes"]; ok {
		t.Fatalf("stale type metadata remains: %#v", got.sessionTypes)
	}
	if _, ok := got.sessionLabels["notes"]; ok {
		t.Fatalf("stale label metadata remains: %#v", got.sessionLabels)
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := state.SessionProjects["notes"]; ok {
		t.Fatalf("stale project state remains: %#v", state.SessionProjects)
	}
	if _, ok := state.SessionTypes["notes"]; ok {
		t.Fatalf("stale type state remains: %#v", state.SessionTypes)
	}
	if _, ok := state.SessionLabels["notes"]; ok {
		t.Fatalf("stale label state remains: %#v", state.SessionLabels)
	}
}

func TestRenameVolatileSessionClearsStaleMetadata(t *testing.T) {
	var renamed []string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = []string{oldName, newName}
			return nil
		},
	}, "notes").(model)
	m.sessions = []session{{Name: "notes", Temporary: true, Instance: "instance-1"}}
	m.currentSession = "notes"
	m.selectedSession = "notes"
	m.renameTarget = renameTarget{session: "notes"}
	m.sessionProjects = map[string]string{"notes": "old-project", "dev": "other-project"}
	m.sessionTypes = map[string]sessionType{"notes": sessionTypeAgent, "dev": sessionTypeAgent}
	m.sessionLabels = map[string]string{"notes": "old-notes", "dev": "old-dev"}
	m.statePath = t.TempDir() + "/store.json"
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}
	m.input.SetValue("dev")

	updated, cmd := m.commitRename()
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd().(sessionRenamedMsg)
	if msg.err != nil {
		t.Fatalf("rename returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(renamed), "[notes dev]"; got != want {
		t.Fatalf("renameSession calls = %s, want %s", got, want)
	}

	pending := *(updated.(*model))
	updated, followUp := pending.Update(msg)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	got := updated.(model)
	for _, name := range []string{"notes", "dev"} {
		if _, ok := got.sessionProjects[name]; ok {
			t.Fatalf("stale project metadata remains for %s: %#v", name, got.sessionProjects)
		}
		if _, ok := got.sessionTypes[name]; ok {
			t.Fatalf("stale type metadata remains for %s: %#v", name, got.sessionTypes)
		}
		if _, ok := got.sessionLabels[name]; ok {
			t.Fatalf("stale label metadata remains for %s: %#v", name, got.sessionLabels)
		}
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes", "dev"} {
		if _, ok := state.SessionProjects[name]; ok {
			t.Fatalf("stale project state remains for %s: %#v", name, state.SessionProjects)
		}
		if _, ok := state.SessionTypes[name]; ok {
			t.Fatalf("stale type state remains for %s: %#v", name, state.SessionTypes)
		}
		if _, ok := state.SessionLabels[name]; ok {
			t.Fatalf("stale label state remains for %s: %#v", name, state.SessionLabels)
		}
	}
}

func TestVolatileContextShowsOnlyCurrentInstanceSessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.instanceID = "instance-1"
	m.projects = []string{"project"}
	m.sessions = []session{
		{Name: "scratch-temp", Temporary: true, Instance: "instance-1"},
		{Name: "notes", Temporary: true, Instance: "instance-1"},
		{Name: "other", Temporary: true, Instance: "instance-2"},
		{Name: "project--code"},
	}
	m.sessionProjects = map[string]string{"project--code": "project"}
	m.syncSelection()

	if m.selectedProject != "" {
		t.Fatalf("selectedProject = %q, want volatile context", m.selectedProject)
	}
	var names []string
	for _, s := range m.contextSessions() {
		names = append(names, s.Name)
	}
	if got, want := fmt.Sprint(names), "[scratch-temp notes]"; got != want {
		t.Fatalf("visible sessions = %s, want %s", got, want)
	}
}
