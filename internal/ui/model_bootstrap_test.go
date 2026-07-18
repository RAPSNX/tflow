package ui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuStartsWithCurrentSessionSelected(t *testing.T) {
	m := NewMenu().(model)
	m.currentSession = "dev"
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.syncSelection()

	if m.selectedProject != defaultProjectName {
		t.Fatalf("selectedProject = %q, want %q", m.selectedProject, defaultProjectName)
	}
	if m.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", m.selectedSession)
	}
}

func TestPrepareStartupCreatesSessionBeforeControlMode(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	var calls []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return nil, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			calls = append(calls, "create:"+name)
			if command != "" {
				t.Fatalf("command = %q, want empty", command)
			}
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			calls = append(calls, fmt.Sprintf("temporary:%s:%t:%s", name, temporary, instanceID))
			return nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "control:"+binaryPath)
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if !strings.HasSuffix(name, "-temp") {
		t.Fatalf("name = %q, want temp session name", name)
	}

	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{"create:" + name, "temporary:" + name + ":true:instance-1", "control:/tmp/tflow"}); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestPrepareStartupRetriesWhenTempSessionNameAlreadyExists(t *testing.T) {
	var calls []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "otter-temp"}}, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			calls = append(calls, "create:"+name)
			if name == "fox-temp" {
				return session{}, fmt.Errorf("duplicate session: fox-temp")
			}
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			calls = append(calls, fmt.Sprintf("temporary:%s:%t:%s", name, temporary, instanceID))
			return nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "control:"+binaryPath)
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if got, want := name, "lynx-temp"; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{
		"create:fox-temp",
		"create:lynx-temp",
		"temporary:lynx-temp:true:instance-1",
		"control:/tmp/tflow",
	}); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestStartWithManagerCleansUpInstanceVolatileSessionsAfterAttach(t *testing.T) {
	var cleaned []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return nil, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return nil
		},
		attachCommand: func(name string) (*exec.Cmd, error) {
			return exec.Command("sh", "-c", ":"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-2"); err != nil {
		t.Fatalf("startWithManager returned error: %v", err)
	}
	if got, want := fmt.Sprint(cleaned), fmt.Sprint([]string{"instance-2"}); got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
	if got := os.Getenv(menuInstanceEnv); got != "instance-2" {
		t.Fatalf("%s = %q, want instance-2", menuInstanceEnv, got)
	}
}

func TestNewInstanceIDWithEntropyUsesRandomToken(t *testing.T) {
	now := time.Unix(0, 123456789)
	got := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}), 99)
	if want := "tflow-21i3v9-000102030405"; got != want {
		t.Fatalf("newInstanceIDWithEntropy = %q, want %q", got, want)
	}
}

func TestNewInstanceIDWithEntropyFallsBackToPID(t *testing.T) {
	now := time.Unix(0, 123456789)
	got := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1}), 4242)
	if want := "tflow-21i3v9-4242"; got != want {
		t.Fatalf("newInstanceIDWithEntropy = %q, want %q", got, want)
	}
}

func TestNewInstanceIDWithEntropyDiffersWithinSameTick(t *testing.T) {
	now := time.Unix(0, 123456789)
	first := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}), 99)
	second := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{6, 7, 8, 9, 10, 11}), 99)
	if first == second {
		t.Fatalf("same-tick instance ids matched: %q", first)
	}
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
	if msg.err != nil {
		t.Fatalf("menu action returned error: %v", msg.err)
	}
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
	if got.input.Prompt != "project: " {
		t.Fatalf("prompt = %q, want project prompt", got.input.Prompt)
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
	if got.input.Prompt != "project: " {
		t.Fatalf("prompt = %q, want project prompt", got.input.Prompt)
	}
	if got.status != "Create a new project." {
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
	if msg.err != nil {
		t.Fatalf("menu action returned error: %v", msg.err)
	}
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
	if msg.err != nil {
		t.Fatalf("menu action returned error: %v", msg.err)
	}
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

func TestCreateProjectCreatesDefaultCodeSession(t *testing.T) {
	tmp := t.TempDir()
	var createdName, createdDir, createdCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			createdName = name
			createdDir = cwd
			createdCommand = command
			return session{Name: name, Windows: 1}, nil
		},
		listSessions: func() ([]session, error) {
			return []session{{Name: projectSessionName("small", defaultProjectSessionName), Windows: 1}}, nil
		},
	}, "").(model)
	m.statePath = tmp + "/store.json"
	m.cwd = "/tmp/workspace"
	m.mode = inputCreateProject
	m.input.Prompt = "project: "
	m.input.SetValue("Small")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	pending := *(updated.(*model))
	if cmd == nil {
		t.Fatal("expected create project command")
	}
	msg := cmd().(projectCreatedMsg)
	if msg.err != nil {
		t.Fatalf("project create returned error: %v", msg.err)
	}
	if createdName != projectSessionName("small", defaultProjectSessionName) {
		t.Fatalf("created session name = %q, want %q", createdName, projectSessionName("small", defaultProjectSessionName))
	}
	if createdDir != "/tmp/workspace" {
		t.Fatalf("created session dir = %q, want /tmp/workspace", createdDir)
	}
	if createdCommand != "" {
		t.Fatalf("created session command = %q, want empty", createdCommand)
	}

	updated, followUp := pending.Update(msg)
	got := updated.(model)
	if followUp == nil {
		t.Fatal("expected reload command")
	}
	if got.selectedProject != "small" {
		t.Fatalf("selectedProject = %q, want small", got.selectedProject)
	}
	if got.selectedSession != projectSessionName("small", defaultProjectSessionName) {
		t.Fatalf("selectedSession = %q, want %q", got.selectedSession, projectSessionName("small", defaultProjectSessionName))
	}
	if !containsString(got.projects, "small") {
		t.Fatalf("projects = %#v, want small project", got.projects)
	}
	if got.sessionProjects[projectSessionName("small", defaultProjectSessionName)] != "small" {
		t.Fatalf("sessionProjects[%q] = %q, want small", projectSessionName("small", defaultProjectSessionName), got.sessionProjects[projectSessionName("small", defaultProjectSessionName)])
	}
	if got.sessionTypes[projectSessionName("small", defaultProjectSessionName)] != sessionTypeTerminal {
		t.Fatalf("sessionTypes[%q] = %q, want %q", projectSessionName("small", defaultProjectSessionName), got.sessionTypes[projectSessionName("small", defaultProjectSessionName)], sessionTypeTerminal)
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if state.SessionProjects[projectSessionName("small", defaultProjectSessionName)] != "small" {
		t.Fatalf("saved sessionProjects[%q] = %q, want small", projectSessionName("small", defaultProjectSessionName), state.SessionProjects[projectSessionName("small", defaultProjectSessionName)])
	}
	if state.ProjectConfigs["small"].Name != "small" {
		t.Fatalf("saved project config = %#v", state.ProjectConfigs["small"])
	}
}

func TestProjectDefaultSessionNamesAreScoped(t *testing.T) {
	created := map[string]bool{}
	m := newModel(fakeTmuxController{
		createSession: func(name, _ string, _ string) (session, error) {
			if created[name] {
				return session{}, fmt.Errorf("duplicate session %q", name)
			}
			created[name] = true
			return session{Name: name}, nil
		},
	}, "").(model)

	for _, project := range []string{"one", "two"} {
		m.input.SetValue(project)
		_, cmd := m.commitProjectCreate()
		msg := cmd().(projectCreatedMsg)
		if msg.err != nil {
			t.Fatalf("create project %q: %v", project, msg.err)
		}
	}

	for _, name := range []string{"one--code", "two--code"} {
		if !created[name] {
			t.Fatalf("created sessions = %#v, missing %q", created, name)
		}
	}
}

func TestDDeletesSelectedSession(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	pending := *(updated.(*model))
	if pending.mode != inputConfirmDelete {
		t.Fatalf("mode = %v, want inputConfirmDelete", pending.mode)
	}
	_, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected kill command")
	}
	msg := cmd().(sessionKilledMsg)
	if msg.err != nil {
		t.Fatalf("kill returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(killed), fmt.Sprint([]string{"dev"}); got != want {
		t.Fatalf("killed = %s, want %s", got, want)
	}
}

func TestDeleteLastProjectSessionRequiresConfirmation(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
	m.selectedProject = "small"
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no delete command before confirmation")
	}
	if got.mode != inputConfirmDelete {
		t.Fatalf("mode = %v, want inputConfirmDelete", got.mode)
	}
	if got.deleteTarget.session != "dev" {
		t.Fatalf("deleteTarget = %#v, want session dev", got.deleteTarget)
	}
	if len(killed) != 0 {
		t.Fatalf("killSession called before confirmation: %#v", killed)
	}
}

func TestKFromFirstSessionWrapsToLastSession(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName, "api": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.selectedProject != defaultProjectName {
		t.Fatalf("selectedProject = %q, want %q", got.selectedProject, defaultProjectName)
	}
	if got.selectedSession != "api" {
		t.Fatalf("selectedSession = %q, want api", got.selectedSession)
	}
}

func TestJFromLastSessionWrapsToFirstSession(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName, "api": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "api"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.selectedProject != defaultProjectName {
		t.Fatalf("selectedProject = %q, want %q", got.selectedProject, defaultProjectName)
	}
	if got.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", got.selectedSession)
	}
}

func TestCtrlCClosesMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}

func TestCtrlCClosesMenuFromModalMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputRename
	m.input.Prompt = "session: "
	m.input.SetValue("dev")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}

func TestCtrlFClosesMenuFromNormalMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}

func TestCtrlFClosesMenuFromModalMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputRename
	m.input.Prompt = "session: "
	m.input.SetValue("dev")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlF})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}

func TestEscCancelsPromptWithoutClosingMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small", "storage"}
	m.mode = inputSwitchProject
	m.input.Prompt = "project: "
	m.input.SetValue("sto")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no close command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.status != "Project switch cancelled." {
		t.Fatalf("status = %q", got.status)
	}
	if got.input.Value() != "" {
		t.Fatalf("input value = %q, want cleared", got.input.Value())
	}
}

func TestEscCancelsDeleteConfirmationWithoutClosingMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "dev"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no close command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.deleteTarget != (deleteTarget{}) {
		t.Fatalf("deleteTarget = %#v, want empty", got.deleteTarget)
	}
	if got.status != "Delete cancelled." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestEscCancelsProjectSwitchConfirmationWithoutClosingMenu(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputConfirmProjectSwitch
	m.switchProjectTarget = "storage"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no close command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.switchProjectTarget != "" {
		t.Fatalf("switchProjectTarget = %q, want empty", got.switchProjectTarget)
	}
	if got.status != "Project switch cancelled." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestNStartsNewPrefixMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNew {
		t.Fatalf("mode = %v, want inputNew", got.mode)
	}
}

func TestNewModelStartsWithoutProjectsWhenStateIsEmpty(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	m := newModel(fakeTmuxController{}, "").(model)
	if len(m.projects) != 0 {
		t.Fatalf("projects = %#v, want none", m.projects)
	}
	if m.selectedProject != "" {
		t.Fatalf("selectedProject = %q, want empty", m.selectedProject)
	}
}

func TestBuildModelFailsWhenStoreIsInvalid(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	path := appStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := buildModel(fakeTmuxController{}, "")
	if err == nil {
		t.Fatal("buildModel returned nil error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want path %q", err, path)
	}
}

func TestNewPrefixTStartsTerminalCreate(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputNew
	m.selectedProject = "small"

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputCreateSession {
		t.Fatalf("mode = %v, want inputCreateSession", got.mode)
	}
	if got.createSessionKind != sessionKindTerminal {
		t.Fatalf("createSessionKind = %v", got.createSessionKind)
	}
}

func TestNewPrefixUnknownKeyShowsHint(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputNew

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.status != "New: use p, t, k, or c." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestRunMenuExitActionSwitchesClientAfterExit(t *testing.T) {
	var switched []string
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
	}, model{exitAction: menuExitSwitchSession, exitSessionName: "dev"})
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(switched), fmt.Sprint([]string{"dev"}); got != want {
		t.Fatalf("switches = %s, want %s", got, want)
	}
}
