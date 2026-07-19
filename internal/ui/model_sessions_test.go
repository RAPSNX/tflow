package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultSessionDirUsesCurrentDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore current directory: %v", err)
		}
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatalf("create current directory: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("change current directory: %v", err)
	}

	if got := defaultSessionDir(); got != cwd {
		t.Fatalf("defaultSessionDir = %q, want %q", got, cwd)
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
	m.sessions = []session{{Name: volatileSessionName("instance-1", "scratch-temp"), Temporary: true, Instance: "instance-1"}}
	m.currentSession = volatileSessionName("instance-1", "scratch-temp")
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
	if got, want := fmt.Sprint(tagged), fmt.Sprintf("[%s:true:instance-1]", volatileSessionName("instance-1", "notes")); got != want {
		t.Fatalf("temporary tags = %s, want %s", got, want)
	}
	got := updated.(model)
	updated, followUp := got.Update(msg)
	got = updated.(model)
	if got.selectedProject != "" || got.selectedSession != volatileSessionName("instance-1", "notes") {
		t.Fatalf("selection = project %q session %q, want volatile notes", got.selectedProject, got.selectedSession)
	}
	if sessions := got.contextSessions(); len(sessions) != 2 {
		t.Fatalf("volatile sessions = %#v, want startup and new session", sessions)
	}
	for _, name := range []string{volatileSessionName("instance-1", "scratch-temp"), volatileSessionName("instance-1", "notes")} {
		session, found := got.findSession(name)
		if !found || !session.Temporary || session.Instance != "instance-1" {
			t.Fatalf("session %q = %#v, want instance-owned volatile session", name, session)
		}
	}
	if followUp == nil {
		t.Fatal("expected switch command")
	}
	if action := followUp().(menuActionMsg); action.switchSession != volatileSessionName("instance-1", "notes") {
		t.Fatalf("switch session = %q, want notes", action.switchSession)
	}
	if _, ok := got.sessionProjects[volatileSessionName("instance-1", "notes")]; ok {
		t.Fatalf("volatile session metadata persisted: %#v", got.sessionProjects)
	}
}

func TestVolatileSessionNamesAreScopedToEachInstance(t *testing.T) {
	create := func(instanceID string) sessionCreatedMsg {
		m := newModel(fakeTmuxController{
			createSession: func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
			setSessionTemporary: func(name string, temporary bool, gotInstanceID string) error {
				if !temporary || gotInstanceID != instanceID {
					t.Fatalf("temporary session = %q, %t, %q", name, temporary, gotInstanceID)
				}
				return nil
			},
		}, "").(model)
		m.instanceID = instanceID
		m.mode = inputCreateSession
		m.input.SetValue("code")

		_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
		if cmd == nil {
			t.Fatal("expected create command")
		}
		return cmd().(sessionCreatedMsg)
	}

	first := create("instance-1")
	second := create("instance-2")
	if first.err != nil || second.err != nil {
		t.Fatalf("create errors = %v, %v", first.err, second.err)
	}
	if first.session.Name == second.session.Name {
		t.Fatalf("volatile tmux names collide: %q", first.session.Name)
	}
	menu := newModel(fakeTmuxController{}, first.session.Name).(model)
	menu.sessions = []session{{Name: first.session.Name, Temporary: true, Instance: "instance-1"}}
	if got := menu.sessionLabel(first.session.Name); got != "code" {
		t.Fatalf("sidebar label = %q, want code", got)
	}
	for _, created := range []sessionCreatedMsg{first, second} {
		if created.label != "code" {
			t.Fatalf("visible label = %q, want code", created.label)
		}
	}
}

func TestCreateProjectSessionPersistsMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/small"}}

	updated, _ := m.Update(sessionCreatedMsg{
		session: session{Name: "small--otter"},
		project: "small",
		label:   "otter",
	})
	got := updated.(model)
	if _, exists := got.findSession("small--otter"); !exists {
		t.Fatal("new project session was not added to the model")
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.SessionProjects["small--otter"] != "small" || state.SessionLabels["small--otter"] != "otter" {
		t.Fatalf("saved metadata = %#v / %#v", state.SessionProjects, state.SessionLabels)
	}
}

func TestCreateVolatileSessionClearsStaleMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.sessionProjects = map[string]string{"notes": "old-project"}
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
	}, volatileSessionName("instance-1", "notes")).(model)
	m.instanceID = "instance-1"
	m.sessions = []session{{Name: volatileSessionName("instance-1", "notes"), Temporary: true, Instance: "instance-1"}}
	m.currentSession = volatileSessionName("instance-1", "notes")
	m.selectedSession = volatileSessionName("instance-1", "notes")
	m.renameTarget = renameTarget{session: volatileSessionName("instance-1", "notes")}
	m.sessionProjects = map[string]string{volatileSessionName("instance-1", "notes"): "old-project", volatileSessionName("instance-1", "dev"): "other-project"}
	m.sessionLabels = map[string]string{volatileSessionName("instance-1", "notes"): "old-notes", volatileSessionName("instance-1", "dev"): "old-dev"}
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
	if got, want := fmt.Sprint(renamed), fmt.Sprintf("[%s %s]", volatileSessionName("instance-1", "notes"), volatileSessionName("instance-1", "dev")); got != want {
		t.Fatalf("renameSession calls = %s, want %s", got, want)
	}

	pending := *(updated.(*model))
	updated, followUp := pending.Update(msg)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	got := updated.(model)
	for _, name := range []string{volatileSessionName("instance-1", "notes"), volatileSessionName("instance-1", "dev")} {
		if _, ok := got.sessionProjects[name]; ok {
			t.Fatalf("stale project metadata remains for %s: %#v", name, got.sessionProjects)
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
