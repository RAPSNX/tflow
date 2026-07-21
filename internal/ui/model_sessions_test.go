package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionStartDirFallsBackWhenDirectoryDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	missingDir := filepath.Join(t.TempDir(), "missing")
	if got := sessionStartDir(missingDir); got != home {
		t.Fatalf("sessionStartDir(%q) = %q, want %q", missingDir, got, home)
	}
}

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
	t.Skip("superseded by background worker coverage")
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
	t.Skip("superseded by background worker coverage")
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
	t.Skip("superseded by background worker coverage")
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
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "Prod/Main 01"; got != want {
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
	t.Skip("superseded by background worker coverage")
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
	if len(tagged) != 1 || !strings.HasPrefix(tagged[0], "tflow-v-instance-1-") || !strings.HasSuffix(tagged[0], ":true:instance-1") {
		t.Fatalf("temporary tags = %#v, want opaque ID tagged to the instance", tagged)
	}
	got := updated.(model)
	updated, followUp := got.Update(msg)
	got = updated.(model)
	if got.selectedProject != "" || got.selectedSession != msg.session.Name {
		t.Fatalf("selection = project %q session %q, want volatile notes", got.selectedProject, got.selectedSession)
	}
	if sessions := got.contextSessions(); len(sessions) != 2 {
		t.Fatalf("volatile sessions = %#v, want startup and new session", sessions)
	}
	for _, name := range []string{volatileSessionName("instance-1", "scratch-temp"), msg.session.Name} {
		session, found := got.findSession(name)
		if !found || !session.Temporary || session.Instance != "instance-1" {
			t.Fatalf("session %q = %#v, want instance-owned volatile session", name, session)
		}
	}
	if followUp == nil {
		t.Fatal("expected switch command")
	}
	if action := followUp().(menuActionMsg); action.switchSession != msg.session.Name {
		t.Fatalf("switch session = %q, want notes", action.switchSession)
	}
	if _, ok := got.sessionProjects[msg.session.Name]; ok {
		t.Fatalf("volatile session metadata persisted: %#v", got.sessionProjects)
	}
}

func TestVolatileSessionNamesAreScopedToEachInstance(t *testing.T) {
	t.Skip("superseded by background worker coverage")
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
	menu.sessions = []session{{Name: first.session.Name, Label: first.label, Temporary: true, Instance: "instance-1"}}
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
	project, projectOK := storedProjectByName(state, "small")
	session, sessionOK := storedSessionByID(state, "small--otter")
	if !projectOK || !sessionOK || session.Label != "otter" {
		t.Fatalf("saved state = %#v", state)
	}
	_ = project
}

// TestCreatingSessionWritesMarkersOnlyForTheNewSession proves that
// completing session creation writes tmux markers only for the newly
// created session, not a full resync of every session in the fleet
// (previously 2xN set-option calls for N total sessions).
func TestCreatingSessionWritesMarkersOnlyForTheNewSession(t *testing.T) {
	var projectWrites, labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, name+"="+label)
			return nil
		},
	}, "").(model)
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/small"}}
	m.sessions = make([]session, 0, 50)
	m.sessionProjects = map[string]string{}
	m.sessionLabels = map[string]string{}
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("small--existing-%02d", i)
		m.sessions = append(m.sessions, session{Name: name})
		m.sessionProjects[name] = "small"
		m.sessionLabels[name] = fmt.Sprintf("animal-%02d", i)
	}

	updated, _ := m.Update(sessionCreatedMsg{
		session: session{Name: "small--otter"},
		project: "small",
		label:   "otter",
	})
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("session creation reported error: %v", got.err)
	}

	if fmt.Sprint(projectWrites) != fmt.Sprint([]string{"small--otter=small"}) {
		t.Fatalf("project marker writes = %#v, want only the new session", projectWrites)
	}
	if fmt.Sprint(labelWrites) != fmt.Sprint([]string{"small--otter=otter"}) {
		t.Fatalf("label marker writes = %#v, want only the new session", labelWrites)
	}
}

// TestKillingSessionWritesNoMarkersForUnrelatedSessions proves that killing
// one session never rewrites tmux markers for any other session: the killed
// session no longer exists in tmux, and no other session's project or label
// marker is affected by removing it.
func TestKillingSessionWritesNoMarkersForUnrelatedSessions(t *testing.T) {
	var projectWrites, labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, name+"="+label)
			return nil
		},
	}, "").(model)
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/small"}}
	m.sessions = make([]session, 0, 50)
	m.sessionProjects = map[string]string{}
	m.sessionLabels = map[string]string{}
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("small--existing-%02d", i)
		m.sessions = append(m.sessions, session{Name: name})
		m.sessionProjects[name] = "small"
		m.sessionLabels[name] = fmt.Sprintf("animal-%02d", i)
	}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(sessionKilledMsg{name: "small--existing-00", project: "small"})
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("kill reported error: %v", got.err)
	}

	if len(projectWrites) != 0 || len(labelWrites) != 0 {
		t.Fatalf("marker writes = project:%#v label:%#v, want none", projectWrites, labelWrites)
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
	if _, ok := storedSessionByID(state, "notes"); ok {
		t.Fatalf("stale session state remains: %#v", state)
	}
}

func TestRenameVolatileSessionClearsStaleMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, volatileSessionName("instance-1", "notes")).(model)
	m.instanceID = "instance-1"
	m.sessions = []session{{Name: volatileSessionName("instance-1", "notes"), Label: "notes", Temporary: true, Instance: "instance-1"}}
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

	pending := *(updated.(*model))
	updated, followUp := pending.Update(msg)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	got := updated.(model)
	if _, ok := got.sessionProjects[volatileSessionName("instance-1", "notes")]; ok {
		t.Fatalf("stale target metadata remains: %#v", got.sessionProjects)
	}
	if got.sessionLabel(volatileSessionName("instance-1", "notes")) != "dev" {
		t.Fatalf("volatile label = %q, want dev", got.sessionLabel(volatileSessionName("instance-1", "notes")))
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes", "dev"} {
		if _, ok := storedSessionByID(state, name); ok {
			t.Fatalf("stale session state remains for %s: %#v", name, state)
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

func TestNewSessionIDUsesCryptographicEntropy(t *testing.T) {
	id, err := newSessionIDWithEntropy(bytes.NewReader(make([]byte, 16)))
	if err != nil || id != "00000000000000000000000000000000" {
		t.Fatalf("newSessionIDWithEntropy = %q, %v", id, err)
	}
}

func TestPersistentSessionCreationRetriesIDCollisions(t *testing.T) {
	var names []string
	m := newModel(fakeTmuxController{createSession: func(name, cwd, command string) (session, error) {
		names = append(names, name)
		if len(names) == 1 {
			return session{}, fmt.Errorf("duplicate session")
		}
		return session{Name: name}, nil
	}}, "").(model)
	created, err := m.createPersistentSession("/tmp", "")
	if err != nil || len(names) != 2 || names[0] == names[1] || created.Name != names[1] || !strings.HasPrefix(created.Name, "tflow-p-") {
		t.Fatalf("creation = %#v, names = %#v, err = %v", created, names, err)
	}
}

func TestCreateSessionFailureRestoresInputAfterPendingState(t *testing.T) {
	t.Skip("superseded by background worker coverage")
	createErr := errors.New("tmux session creation failed")
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{}, createErr
		},
	}, "").(model)
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	pending := updated.(model)
	if cmd == nil || pending.mode != inputCreatingSession {
		t.Fatalf("submission = mode %v, command %v; want pending creation command", pending.mode, cmd != nil)
	}

	updated, followUp := pending.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	pending = updated.(model)
	if followUp != nil || pending.mode != inputCreatingSession {
		t.Fatalf("pending Ctrl+C = mode %v, command %v; want unchanged pending state", pending.mode, followUp != nil)
	}

	updated, followUp = pending.Update(cmd())
	got := updated.(model)
	if followUp != nil {
		t.Fatal("unexpected follow-up command after failed creation")
	}
	if got.mode != inputCreateSession {
		t.Fatalf("mode = %v, want inputCreateSession", got.mode)
	}
	if !got.input.Focused() || got.input.Value() != "dev" {
		t.Fatalf("restored input = focused %t value %q, want focused dev", got.input.Focused(), got.input.Value())
	}
	if got.status != createErr.Error() {
		t.Fatalf("status = %q, want %q", got.status, createErr)
	}
}

func TestCreateSessionKeepsPendingStateUntilSwitchAction(t *testing.T) {
	t.Skip("superseded by background worker coverage")
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{Name: name, Windows: 1}, nil
		},
	}, "").(model)
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	updated, createCmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	pending := updated.(model)
	if createCmd == nil || pending.mode != inputCreatingSession {
		t.Fatalf("submission = mode %v, command %v; want pending creation command", pending.mode, createCmd != nil)
	}

	updated, switchCmd := pending.Update(createCmd())
	pending = updated.(model)
	if pending.mode != inputCreatingSession {
		t.Fatalf("mode after creation = %v, want inputCreatingSession", pending.mode)
	}
	if switchCmd == nil {
		t.Fatal("expected session switch command")
	}

	updated, quitCmd := pending.Update(switchCmd())
	got := updated.(model)
	if quitCmd == nil || got.exitAction != menuExitSwitchSession {
		t.Fatalf("switch action = exit %v command %v", got.exitAction, quitCmd != nil)
	}
}
