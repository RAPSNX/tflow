package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
			return exec.Command("sh", "-lc", ":"), nil
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
	m := newModel(fakeTmuxController{}, "dev", "").(model)
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

func TestProjectSwitchFromVolatileSessionRequiresConfirmation(t *testing.T) {
	var switched []string
	m := newModel(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
	}, "scratch-temp", "").(model)
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

func TestDDeletesSelectedSession(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "", "").(model)
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

func TestKFromFirstSessionWrapsToLastSession(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)
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
	m := newModel(fakeTmuxController{}, "", "").(model)
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
	if msg.switchSession != "" || msg.quitAll {
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
	if msg.switchSession != "" || msg.quitAll {
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
	if msg.switchSession != "" || msg.quitAll {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}

func TestNStartsNewPrefixMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)

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

	m := newModel(fakeTmuxController{}, "", "").(model)
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

	_, err := buildModel(fakeTmuxController{}, "", "")
	if err == nil {
		t.Fatal("buildModel returned nil error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want path %q", err, path)
	}
}

func TestNewPrefixTStartsTerminalCreate(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)
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
	m := newModel(fakeTmuxController{}, "", "").(model)
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

func TestRunMenuExitActionQuitsAllAfterExit(t *testing.T) {
	quitAllCalls := 0
	err := runMenuExitAction(fakeTmuxController{
		quitAll: func() error {
			quitAllCalls++
			return nil
		},
	}, model{exitAction: menuExitQuitAll})
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if quitAllCalls != 1 {
		t.Fatalf("quitAllCalls = %d, want 1", quitAllCalls)
	}
}
