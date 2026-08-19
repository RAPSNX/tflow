package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func seedLazyProjectState(t *testing.T, projects ...storedProject) string {
	t.Helper()
	path := t.TempDir() + "/store.json"
	if err := saveAppState(path, appState{Projects: projects}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMenuEnterSwitchesSessionAndClosesMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "dev").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "dev" {
		t.Fatalf("switchSession = %q, want dev", msg.switchSession)
	}
}

func TestMenuActionSwitchSessionTriggersQuit(t *testing.T) {
	m := newModel(fakeTmuxController{}, "dev").(model)

	updated, cmd := m.Update(menuActionMsg{switchSession: "dev"})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	if got.exitAction != menuExitSwitchSession {
		t.Fatalf("exitAction = %v, want menuExitSwitchSession", got.exitAction)
	}
	if got.exitSessionName != "dev" {
		t.Fatalf("exitSessionName = %q, want dev", got.exitSessionName)
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("cmd() = %T, want tea.QuitMsg", msg)
	}
}

func TestPStartsProjectSwitchMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "dev").(model)
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputSwitchProject {
		t.Fatalf("mode = %v, want inputSwitchProject", got.mode)
	}
	if got.input.Prompt != "" {
		t.Fatalf("prompt = %q, want no input prompt", got.input.Prompt)
	}
}

func TestNStartsProjectCreateMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputCreateProject {
		t.Fatalf("mode = %v, want inputCreateProject", got.mode)
	}
	if got.input.Prompt != "" {
		t.Fatalf("prompt = %q, want no input prompt", got.input.Prompt)
	}
	if got.status != "" {
		t.Fatalf("status = %q", got.status)
	}
}

func TestProjectSwitchUsesUniquePrefixAndClosesMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "dev").(model)
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}, {Name: "keep"}}
	m.sessionProjects = map[string]string{"dev": "small", "api": "small", "keep": "storage"}
	m.selectedProject = "small"
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pending := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	pending.input.SetValue("sto")

	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
	if got.selectedSession != "keep" {
		t.Fatalf("selectedSession = %q, want keep", got.selectedSession)
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "keep" {
		t.Fatalf("switchSession = %q, want keep", msg.switchSession)
	}
}

func TestProjectSwitchSelectsFirstSessionInTargetProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "dev").(model)
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}, {Name: "keep"}, {Name: "ops"}}
	m.sessionProjects = map[string]string{"dev": "small", "keep": "storage", "ops": "storage"}
	m.selectedProject = "small"
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pending := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	pending.input.SetValue("sto")

	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
	if got.selectedSession != "keep" {
		t.Fatalf("selectedSession = %q, want keep", got.selectedSession)
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "keep" {
		t.Fatalf("switchSession = %q, want keep", msg.switchSession)
	}
}

func TestProjectSwitchFromVolatileSessionRequiresConfirmation(t *testing.T) {
	var switched []string
	m := newModel(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
	}, "scratch-temp").(model)
	m.projects = []string{"storage"}
	m.sessions = []session{{Name: "scratch-temp", Temporary: true}, {Name: "keep"}}
	m.sessionProjects = map[string]string{"keep": "storage"}

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	pending := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	pending.input.SetValue("sto")

	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	confirming := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no switch command before confirmation")
	}
	if confirming.mode != inputConfirmProjectSwitch {
		t.Fatalf("mode = %v, want inputConfirmProjectSwitch", confirming.mode)
	}
	if confirming.switchProjectTarget != "storage" {
		t.Fatalf("switchProjectTarget = %q, want storage", confirming.switchProjectTarget)
	}
	if len(switched) != 0 {
		t.Fatalf("switches before confirmation = %#v", switched)
	}
}

func TestProjectSwitchConfirmationRejectsLegacyYBinding(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.mode = inputConfirmProjectSwitch
	m.switchProjectTarget = "storage"

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("legacy y binding should not confirm")
	}
	if got.mode != inputConfirmProjectSwitch {
		t.Fatalf("mode = %v", got.mode)
	}
}

func TestProjectSessionsIncludePersistedMissingSessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.sessions = []session{{Name: "tflow-p-running", Label: "running"}}
	m.sessionProjects = map[string]string{"tflow-p-running": "small", "tflow-p-missing": "small"}
	m.sessionLabels = map[string]string{"tflow-p-running": "running", "tflow-p-missing": "missing"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-missing", "tflow-p-running"}}

	got := m.projectSessions("small")
	if len(got) != 2 || got[0].Name != "tflow-p-missing" || got[0].Label != "missing" || got[1].Name != "tflow-p-running" {
		t.Fatalf("project sessions = %#v, want ordered persisted sessions", got)
	}
}

func TestSwitchSelectedSessionMaterializesPersistedSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	state := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/work/small", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	}}}
	if err := saveAppState(path, state); err != nil {
		t.Fatal(err)
	}

	var createdName, createdDir, createdCommand string
	var markedProject, markedLabel, temporaryInstance string
	markedTemporary := true
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			createdName, createdDir, createdCommand = name, cwd, command
			return session{Name: name, Windows: 1}, nil
		},
		setSessionProject: func(name, project string) error {
			markedProject = name + "=" + project
			return nil
		},
		setSessionLabel: func(name, label string) error {
			markedLabel = name + "=" + label
			return nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			markedTemporary, temporaryInstance = temporary, instanceID
			return nil
		},
	}
	m, err := buildModel(manager, "scratch")
	if err != nil {
		t.Fatal(err)
	}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-code"

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	if createdName != "tflow-p-code" || createdDir != "/work/small" || createdCommand != "" {
		t.Fatalf("created session = (%q, %q, %q)", createdName, createdDir, createdCommand)
	}
	if markedProject != "tflow-p-code=small" || markedLabel != "tflow-p-code=code" || markedTemporary || temporaryInstance != "" {
		t.Fatalf("restored markers = project %q, label %q, temporary %t, instance %q", markedProject, markedLabel, markedTemporary, temporaryInstance)
	}
	if len(got.sessions) != 1 || got.sessions[0].Name != "tflow-p-code" || got.sessions[0].Label != "code" {
		t.Fatalf("sessions = %#v", got.sessions)
	}
	if msg := cmd(); msg.(menuActionMsg).switchSession != "tflow-p-code" {
		t.Fatalf("switch message = %#v", msg)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 || persisted.Projects[0].Sessions[0].ID != "tflow-p-code" {
		t.Fatalf("persisted state = %#v", persisted)
	}
}

func TestSwitchToProjectMaterializesFirstPersistedSession(t *testing.T) {
	var created string
	statePath := seedLazyProjectState(t, storedProject{
		Name:    "small",
		Workdir: "/work/small",
		Sessions: []persistentSession{
			{ID: "tflow-p-first", Label: "first"},
			{ID: "tflow-p-second", Label: "second"},
		},
	})
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = name
			return session{Name: name}, nil
		},
	}, "scratch").(model)
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
	m.sessionProjects = map[string]string{"tflow-p-first": "small", "tflow-p-second": "small"}
	m.sessionLabels = map[string]string{"tflow-p-first": "first", "tflow-p-second": "second"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-first", "tflow-p-second"}}
	m.statePath = statePath

	updated, cmd := m.switchToProject("small")
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	if created != "tflow-p-first" || got.selectedSession != "tflow-p-first" {
		t.Fatalf("created = %q, selected = %q, want first stored session", created, got.selectedSession)
	}
	if msg := cmd(); msg.(menuActionMsg).switchSession != "tflow-p-first" {
		t.Fatalf("switch message = %#v", msg)
	}
}

func TestSwitchSelectedSessionCleansUpWhenMaterializationSetupFails(t *testing.T) {
	var killed []string
	statePath := seedLazyProjectState(t, storedProject{
		Name: "small", Workdir: "/work/small", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	})
	m := newModel(fakeTmuxController{
		setSessionLabel: func(name, label string) error { return errors.New("label failed") },
		killSession:     func(name string) error { killed = append(killed, name); return nil },
	}, "scratch").(model)
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "code"}
	m.selectedSession = "tflow-p-code"
	m.statePath = statePath

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no switch command")
	}
	if len(killed) != 1 || killed[0] != "tflow-p-code" {
		t.Fatalf("killed sessions = %#v", killed)
	}
	if len(got.sessions) != 0 || !strings.Contains(got.status, "label failed") {
		t.Fatalf("sessions = %#v, status = %q", got.sessions, got.status)
	}
}

func TestLazyMaterializationLeavesTargetWhenClientSwitchFails(t *testing.T) {
	created := false
	killed := false
	statePath := seedLazyProjectState(t, storedProject{
		Name: "small", Workdir: "/work/small", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	})
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = true
			return session{Name: name}, nil
		},
		killSession:  func(name string) error { killed = true; return nil },
		switchClient: func(name string) error { return errors.New("switch failed") },
	}
	m := newModel(manager, "scratch").(model)
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "code"}
	m.selectedSession = "tflow-p-code"
	m.statePath = statePath

	updated, cmd := m.switchSelectedSession()
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	final, _ := updated.Update(cmd())
	if err := runMenuExitAction(manager, final); err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("runMenuExitAction error = %v", err)
	}
	if !created || killed {
		t.Fatalf("created = %t, killed = %t, want created target retained", created, killed)
	}
}

func TestSwitchSelectedSessionReportsMaterializationCreateFailure(t *testing.T) {
	statePath := seedLazyProjectState(t, storedProject{
		Name: "small", Workdir: "/work/small", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	})
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{}, errors.New("create failed")
		},
	}, "scratch").(model)
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "code"}
	m.selectedSession = "tflow-p-code"
	m.statePath = statePath

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd != nil || !strings.Contains(got.status, "create failed") || len(got.sessions) != 0 {
		t.Fatalf("cmd = %v, status = %q, sessions = %#v", cmd, got.status, got.sessions)
	}
}

func TestDeletingRunningSessionPreservesAbsentPersistedSibling(t *testing.T) {
	initial := appState{Projects: []storedProject{{
		Name:    "small",
		Workdir: "/work/small",
		Sessions: []persistentSession{
			{ID: "tflow-p-running", Label: "running"},
			{ID: "tflow-p-absent", Label: "absent"},
		},
	}}}
	statePath := seedLazyProjectState(t, initial.Projects...)
	var created string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = name
			return session{Name: name}, nil
		},
	}, "tflow-p-running").(model)
	m.statePath = statePath
	m.stateBase = initial
	m.stateBasePath = statePath
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
	m.sessions = []session{{Name: "tflow-p-running", Label: "running"}}
	m.sessionProjects = map[string]string{"tflow-p-running": "small", "tflow-p-absent": "small"}
	m.sessionLabels = map[string]string{"tflow-p-running": "running", "tflow-p-absent": "absent"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-running", "tflow-p-absent"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-running"

	updated, cmd := m.Update(sessionKilledMsg{name: "tflow-p-running", project: "small"})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected switch to the saved sibling")
	}
	if len(got.projects) != 1 || got.projects[0] != "small" {
		t.Fatalf("projects = %#v, want small retained", got.projects)
	}
	if got.selectedSession != "tflow-p-absent" || created != "tflow-p-absent" {
		t.Fatalf("selected = %q, created = %q, want absent sibling restored", got.selectedSession, created)
	}
	persisted, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 || persisted.Projects[0].Sessions[0].ID != "tflow-p-absent" {
		t.Fatalf("persisted state = %#v, want only absent sibling", persisted)
	}
}

func TestLazyMaterializationDoesNotCreateDeletedSavedSession(t *testing.T) {
	statePath := seedLazyProjectState(t)
	created := false
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = true
			return session{Name: name}, nil
		},
	}, "scratch").(model)
	m.statePath = statePath
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "code"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-code"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-code"

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd != nil || created || got.status != "Session no longer exists." {
		t.Fatalf("cmd = %v, created = %t, status = %q", cmd, created, got.status)
	}
}

func TestLazyMaterializationDoesNotCreateSessionMovedToAnotherProject(t *testing.T) {
	statePath := seedLazyProjectState(t, storedProject{
		Name: "garden", Workdir: "/work/garden", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	})
	created := false
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = true
			return session{Name: name}, nil
		},
	}, "scratch").(model)
	m.statePath = statePath
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "code"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-code"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-code"

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd != nil || created || got.status != "Session is no longer in the selected project." {
		t.Fatalf("cmd = %v, created = %t, status = %q", cmd, created, got.status)
	}
}

func TestLazyMaterializationUsesCurrentSavedProjectConfiguration(t *testing.T) {
	statePath := seedLazyProjectState(t, storedProject{
		Name: "small", Workdir: "/work/current", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "current"}},
	})
	var createdDir, markedLabel string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			createdDir = cwd
			return session{Name: name}, nil
		},
		setSessionLabel: func(name, label string) error {
			markedLabel = label
			return nil
		},
	}, "scratch").(model)
	m.statePath = statePath
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/stale"}}
	m.sessionProjects = map[string]string{"tflow-p-code": "small"}
	m.sessionLabels = map[string]string{"tflow-p-code": "stale"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-code"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-code"

	updated, cmd := m.switchSelectedSession()
	got := updated.(model)
	if cmd == nil || createdDir != "/work/current" || markedLabel != "current" {
		t.Fatalf("cmd = %v, workdir = %q, label = %q", cmd, createdDir, markedLabel)
	}
	if got.sessionLabels["tflow-p-code"] != "current" || got.sessions[0].Label != "current" {
		t.Fatalf("materialized session = %#v, labels = %#v", got.sessions, got.sessionLabels)
	}
}

func TestDeletingAbsentPersistentSessionRemovesOnlyItsMetadata(t *testing.T) {
	initial := appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-absent", Label: "absent"}},
	}}}
	statePath := seedLazyProjectState(t, initial.Projects...)
	killed := false
	m := newModel(fakeTmuxController{killSession: func(name string) error {
		killed = true
		return nil
	}}, "other").(model)
	m.statePath = statePath
	m.stateBase = initial
	m.stateBasePath = statePath
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}
	m.sessionProjects = map[string]string{"tflow-p-absent": "small"}
	m.sessionLabels = map[string]string{"tflow-p-absent": "absent"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-absent"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-absent"

	updated, cmd := m.killSession("tflow-p-absent")
	if cmd == nil {
		t.Fatal("expected deletion message")
	}
	msg := cmd().(sessionKilledMsg)
	if msg.err != nil || killed {
		t.Fatalf("delete message = %#v, killed = %t", msg, killed)
	}
	updated, _ = updated.(model).Update(msg)
	got := updated.(model)
	if len(got.projects) != 0 || len(got.projectSessions("small")) != 0 {
		t.Fatalf("model retained deleted placeholder: %#v", got)
	}
	persisted, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 0 {
		t.Fatalf("persisted state = %#v, want deleted placeholder removed", persisted)
	}
}

func TestRenamingAbsentPersistentSessionUpdatesStateWithoutTmuxWrite(t *testing.T) {
	initial := appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-absent", Label: "old"}},
	}}}
	statePath := seedLazyProjectState(t, initial.Projects...)
	labelWrites := 0
	m := newModel(fakeTmuxController{setSessionLabel: func(name, label string) error {
		labelWrites++
		return nil
	}}, "other").(model)
	m.statePath = statePath
	m.stateBase = initial
	m.stateBasePath = statePath
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}
	m.sessionProjects = map[string]string{"tflow-p-absent": "small"}
	m.sessionLabels = map[string]string{"tflow-p-absent": "old"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-absent"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-absent"

	updated, cmd := m.beginRename()
	pending := *(updated.(*model))
	pending.input.SetValue("new")
	updated, cmd = pending.commitRename()
	if cmd == nil {
		t.Fatal("expected rename message")
	}
	msg := cmd().(sessionRenamedMsg)
	if msg.err != nil || labelWrites != 0 {
		t.Fatalf("rename message = %#v, label writes = %d", msg, labelWrites)
	}
	updated, _ = updated.(*model).Update(msg)
	persisted, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Projects[0].Sessions[0].Label != "new" {
		t.Fatalf("persisted state = %#v, want updated label", persisted)
	}
}

func TestMovingAbsentPersistentSessionUpdatesStateWithoutTmuxWrites(t *testing.T) {
	initial := appState{Projects: []storedProject{
		{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-absent", Label: "absent"}}},
		{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-live", Label: "live"}}},
	}}
	statePath := seedLazyProjectState(t, initial.Projects...)
	markerWrites := 0
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error { markerWrites++; return nil },
		setSessionLabel:   func(name, label string) error { markerWrites++; return nil },
	}, "other").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.sessionProjects = map[string]string{"tflow-p-absent": "small", "tflow-p-live": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-absent": "absent", "tflow-p-live": "live"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-absent"}, "garden": {"tflow-p-live"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-absent"

	updated, cmd := m.applySessionMove("tflow-p-absent", "garden")
	if cmd == nil || markerWrites != 0 {
		t.Fatalf("cmd = %v, marker writes = %d", cmd, markerWrites)
	}
	got := updated.(model)
	if got.sessionProjects["tflow-p-absent"] != "garden" {
		t.Fatalf("project = %q, want garden", got.sessionProjects["tflow-p-absent"])
	}
	persisted, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := storedProjectByName(persisted, "small"); found {
		t.Fatalf("persisted state = %#v, want empty source removed", persisted)
	}
}

func TestRenameRejectsLabelUsedByAbsentPersistentSession(t *testing.T) {
	labelWrites := 0
	m := newModel(fakeTmuxController{setSessionLabel: func(name, label string) error {
		labelWrites++
		return nil
	}}, "tflow-p-live").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-live", Label: "live"}}
	m.sessionProjects = map[string]string{"tflow-p-live": "small", "tflow-p-absent": "small"}
	m.sessionLabels = map[string]string{"tflow-p-live": "live", "tflow-p-absent": "reserved"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-live", "tflow-p-absent"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-live"
	m.renameTarget = renameTarget{session: "tflow-p-live"}
	m.input.SetValue("reserved")

	updated, cmd := m.commitRename()
	got := *(updated.(*model))
	if cmd != nil || labelWrites != 0 || got.status != "Session name already exists in this project." {
		t.Fatalf("cmd = %v, writes = %d, status = %q", cmd, labelWrites, got.status)
	}
}

func TestSecondLazyMaterializationReusesSessionCreatedByFirstPopup(t *testing.T) {
	statePath := seedLazyProjectState(t, storedProject{
		Name: "small", Workdir: "/work/small", Sessions: []persistentSession{{ID: "tflow-p-code", Label: "code"}},
	})
	var running []session
	createCalls := 0
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return append([]session(nil), running...), nil },
		createSession: func(name, cwd, command string) (session, error) {
			createCalls++
			created := session{Name: name, Windows: 1}
			running = append(running, created)
			return created, nil
		},
	}
	newPopup := func() model {
		m := newModel(manager, "scratch").(model)
		m.statePath = statePath
		m.projects = []string{"small"}
		m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/work/small"}}
		m.sessionProjects = map[string]string{"tflow-p-code": "small"}
		m.sessionLabels = map[string]string{"tflow-p-code": "code"}
		m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-code"}}
		m.selectedProject = "small"
		m.selectedSession = "tflow-p-code"
		return m
	}

	first, firstCmd := newPopup().switchSelectedSession()
	if firstCmd == nil || createCalls != 1 {
		t.Fatalf("first materialization cmd = %v, creates = %d", firstCmd, createCalls)
	}
	second, secondCmd := newPopup().switchSelectedSession()
	if secondCmd == nil || createCalls != 1 {
		t.Fatalf("second materialization cmd = %v, creates = %d", secondCmd, createCalls)
	}
	if len(second.(model).sessions) != 1 || second.(model).sessions[0].Name != "tflow-p-code" {
		t.Fatalf("second popup sessions = %#v", second.(model).sessions)
	}
	_ = first
}
