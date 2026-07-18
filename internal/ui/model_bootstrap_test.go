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

	runtmux "tflow/internal/tmux"
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
	if !runtmux.ContainsAnimalName(name) {
		t.Fatalf("name = %q, want a plain animal name", name)
	}

	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{"create:" + name, "temporary:" + name + ":true:instance-1", "control:/tmp/tflow"}); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestPrepareStartupRetriesWhenTempSessionNameAlreadyExists(t *testing.T) {
	var calls []string
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			calls = append(calls, name)
			if len(calls) == 1 {
				return session{}, fmt.Errorf("duplicate session")
			}
			return session{Name: name}, nil
		},
	}
	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err != nil || len(calls) != 2 || calls[0] == calls[1] || name != calls[1] {
		t.Fatalf("retry name = %q calls = %#v err = %v", name, calls, err)
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

func TestCreateProjectCreatesAnimalNamedSession(t *testing.T) {
	var createdName, createdDir string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			createdName, createdDir = name, cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.cwd = "/tmp/workspace"
	m.mode = inputCreateProject
	m.input.SetValue("small")
	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(projectCreatedMsg)
	if msg.err != nil {
		t.Fatalf("create project: %v", msg.err)
	}
	if !runtmux.ContainsAnimalName(msg.label) || createdName != "small--"+msg.label || createdDir != "/tmp/workspace" {
		t.Fatalf("created = %q in %q with label %q", createdName, createdDir, msg.label)
	}
	pending := *(updated.(*model))
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.sessionProjects[createdName] != "small" || got.sessionLabel(createdName) != msg.label {
		t.Fatalf("project state = %#v labels = %#v", got.sessionProjects, got.sessionLabels)
	}
}

func TestCreateProjectsUseAnimalNamedSessions(t *testing.T) {
	var created []string
	m := newModel(fakeTmuxController{createSession: func(name, cwd, command string) (session, error) {
		created = append(created, name)
		return session{Name: name}, nil
	}}, "").(model)
	m.statePath = t.TempDir() + "/store.json"
	for _, project := range []string{"small", "garden"} {
		m.mode = inputCreateProject
		m.input.SetValue(project)
		updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
		msg := cmd().(projectCreatedMsg)
		pending := *(updated.(*model))
		updated, _ = pending.Update(msg)
		m = updated.(model)
	}
	if len(created) != 2 || !strings.HasPrefix(created[0], "small--") || !strings.HasPrefix(created[1], "garden--") {
		t.Fatalf("created sessions = %#v", created)
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
	msg := cmd().(projectDeletedMsg)
	if msg.err != nil {
		t.Fatalf("delete returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(killed), fmt.Sprint([]string{"dev"}); got != want {
		t.Fatalf("killed = %s, want %s", got, want)
	}
}

func TestDeletingNoncurrentVolatileSessionKeepsCurrentVolatileContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-current").(model)
	m.instanceID = "instance-1"
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.sessions = []session{
		{Name: "scratch-current", Temporary: true, Instance: "instance-1"},
		{Name: "scratch-other", Temporary: true, Instance: "instance-1"},
		{Name: "small--code"},
	}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}
	m.selectedSession = "scratch-other"

	updated, cmd := m.Update(sessionKilledMsg{name: "scratch-other"})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected close command")
	}
	if msg := cmd().(menuActionMsg); msg.switchSession != "" {
		t.Fatalf("close action = %#v, want no session switch", msg)
	}
	if got.selectedProject != "" || got.selectedSession != "scratch-current" {
		t.Fatalf("selection = project %q, session %q", got.selectedProject, got.selectedSession)
	}
	if _, exists := got.findSession("scratch-other"); exists {
		t.Fatal("deleted volatile session remains")
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

func TestCtrlQStartsQuitConfirmation(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command before confirmation")
	}
	if got.mode != inputConfirmQuit {
		t.Fatalf("mode = %v, want inputConfirmQuit", got.mode)
	}
}

func TestQuitConfirmationCancelsAndConfirms(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputConfirmQuit

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	cancelled := updated.(model)
	if cmd != nil || cancelled.mode != inputNone || cancelled.status != "Quit cancelled." {
		t.Fatalf("cancelled state = %#v, cmd = %v", cancelled, cmd)
	}

	cancelled.mode = inputConfirmQuit
	updated, cmd = cancelled.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd().(menuActionMsg)
	if !msg.quit {
		t.Fatalf("quit message = %#v", msg)
	}
	updated, _ = updated.(model).Update(msg)
	if got := updated.(model); got.exitAction != menuExitQuit {
		t.Fatalf("exitAction = %v, want menuExitQuit", got.exitAction)
	}
}

func TestProjectSwitchSearchAcceptsJAndKAndUsesArrowNavigation(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"kjobs", "other"}
	m.mode = inputSwitchProject
	m.input.Prompt = "project: "
	m.input.Focus()

	for _, key := range []rune{'k', 'j'} {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = updated.(model)
	}
	if got := m.input.Value(); got != "kj" {
		t.Fatalf("project search = %q, want kj", got)
	}

	m.input.SetValue("")
	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if cmd != nil {
		t.Fatal("expected no command while navigating")
	}
	if got := updated.(model).projectSwitchIndex; got != 1 {
		t.Fatalf("project switch index = %d, want 1", got)
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

func TestNStartsPlainSessionPrompt(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{110}})
	got := *(updated.(*model))
	if cmd != nil || got.mode != inputCreateSession || got.input.Prompt != "session: " {
		t.Fatalf("n should open a plain session prompt: %#v", got)
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

func TestRunMenuExitActionQuitsCurrentInstance(t *testing.T) {
	called := false
	err := runMenuExitAction(fakeTmuxController{
		quitAll: func() error {
			called = true
			return nil
		},
	}, model{exitAction: menuExitQuit})
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if !called {
		t.Fatal("QuitAll was not called")
	}
}

func TestPrepareStartupValidatesStateBeforeTmuxWork(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	path := filepath.Join(stateHome, "tflow", "store.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	_, err := prepareStartup(fakeTmuxController{
		listSessions: func() ([]session, error) {
			called = true
			return nil, nil
		},
	}, "/tmp/tflow", "/tmp/project", "instance-1")
	if err == nil {
		t.Fatal("prepareStartup returned nil error for invalid state")
	}
	if called {
		t.Fatal("tmux work started before state validation")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want state path", err)
	}
}

func TestPrepareStartupRollsBackCreatedSessionOnLaterFailure(t *testing.T) {
	for _, test := range []struct {
		name        string
		failTag     bool
		failControl bool
		want        []string
	}{
		{name: "tag", failTag: true, want: []string{"create", "tag", "kill"}},
		{name: "control", failControl: true, want: []string{"create", "tag", "control", "kill"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			var calls []string
			manager := fakeTmuxController{
				listSessions: func() ([]session, error) { return nil, nil },
				createSession: func(name, cwd, command string) (session, error) {
					calls = append(calls, "create")
					return session{Name: name}, nil
				},
				setSessionTemporary: func(name string, temporary bool, instanceID string) error {
					calls = append(calls, "tag")
					if test.failTag {
						return fmt.Errorf("tag failed")
					}
					return nil
				},
				ensureControlMode: func(binaryPath string) error {
					calls = append(calls, "control")
					if test.failControl {
						return fmt.Errorf("control failed")
					}
					return nil
				},
				killSession: func(name string) error {
					calls = append(calls, "kill")
					return nil
				},
			}
			if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err == nil {
				t.Fatal("prepareStartup returned nil error")
			}
			if got, want := fmt.Sprint(calls), fmt.Sprint(test.want); got != want {
				t.Fatalf("calls = %s, want %s", got, want)
			}
		})
	}
}

func TestStartWithManagerCleansUpWhenAttachCommandFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var cleaned []string
	manager := fakeTmuxController{
		listSessions:        func() ([]session, error) { return nil, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
		attachCommand: func(name string) (*exec.Cmd, error) {
			return nil, fmt.Errorf("attach unavailable")
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err == nil {
		t.Fatal("startWithManager returned nil error")
	}
	if got, want := fmt.Sprint(cleaned), "[instance-1]"; got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
}

func TestHelpEscReturnsToSessionList(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.mode = inputHelp

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("Esc from help should not close the popup")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
}

func TestUndocumentedKeysDoNotDispatch(t *testing.T) {
	for _, test := range []struct {
		name string
		mode inputMode
		key  tea.KeyMsg
	}{
		{name: "down", key: tea.KeyMsg{Type: tea.KeyDown}},
		{name: "up", key: tea.KeyMsg{Type: tea.KeyUp}},
		{name: "delete confirmation y", mode: inputConfirmDelete, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
		{name: "delete confirmation d", mode: inputConfirmDelete, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{100}}},
		{name: "project switch confirmation y", mode: inputConfirmProjectSwitch, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
		{name: "quit confirmation y", mode: inputConfirmQuit, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := newModel(fakeTmuxController{}, "").(model)
			m.mode = test.mode
			updated, cmd := m.Update(test.key)
			got := updated.(model)
			if cmd != nil {
				t.Fatal("undocumented key dispatched an action")
			}
			if got.mode != test.mode {
				t.Fatalf("mode = %v, want %v", got.mode, test.mode)
			}
		})
	}
}

func TestProjectCreationKeepsVolatileSidebarContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.cwd = "/tmp/workspace"
	m.sessions = []session{{Name: "scratch-temp", Temporary: true, Instance: "instance-1"}}
	m.instanceID = "instance-1"
	m.mode = inputCreateProject
	m.input.SetValue("small")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(projectCreatedMsg)
	pending := *(updated.(*model))
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.selectedProject != "" || got.selectedSession != "" {
		t.Fatalf("project creation changed volatile context: project %q session %q", got.selectedProject, got.selectedSession)
	}
}

func TestProjectCreationKeepsExistingProjectContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "existing--code").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.projects = []string{"existing"}
	m.sessions = []session{{Name: "existing--code"}}
	m.selectedProject = "existing"
	m.selectedSession = "existing--code"
	m.sessionProjects = map[string]string{"existing--code": "existing"}
	m.sessionLabels = map[string]string{"existing--code": "code"}

	updated, _ := m.Update(projectCreatedMsg{
		config:  projectConfig{Name: "new", Workdir: "/tmp/new"},
		session: session{Name: "new--code"},
	})
	got := updated.(model)
	if got.selectedProject != "existing" || got.selectedSession != "existing--code" {
		t.Fatalf("project creation changed context: project %q session %q", got.selectedProject, got.selectedSession)
	}
}

func TestDeletingFinalProjectSessionRemovesProjectMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}

	updated, _ := m.Update(sessionKilledMsg{name: "small--code"})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if len(got.projects) != 0 || len(got.projectConfigs) != 0 {
		t.Fatal("final session left project metadata")
	}
}
