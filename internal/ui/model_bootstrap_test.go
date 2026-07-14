package ui

import (
	"fmt"
	"os"
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
		setSessionTemporary: func(name string, temporary bool) error {
			calls = append(calls, fmt.Sprintf("temporary:%s:%t", name, temporary))
			return nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "control:"+binaryPath)
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if !strings.HasSuffix(name, "-temp") {
		t.Fatalf("name = %q, want temp session name", name)
	}

	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{"create:" + name, "temporary:" + name + ":true", "control:/tmp/tflow"}); got != want {
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

func TestColonEntersCommandMode(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "%3").(model)

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputCommand {
		t.Fatalf("mode = %v, want inputCommand", got.mode)
	}
	if got.input.Prompt != ":" {
		t.Fatalf("prompt = %q, want :", got.input.Prompt)
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

func TestCommandModeQAQuitsAll(t *testing.T) {
	quitAllPane := ""
	m := newModel(fakeTmuxController{
		quitAll: func(paneID string) error {
			quitAllPane = paneID
			return nil
		},
	}, "", "%7").(model)
	m.mode = inputCommand
	m.input.Prompt = ":"
	m.input.SetValue("qa!")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected quit-all command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("quit all returned error: %v", msg.err)
	}
	if quitAllPane != "%7" {
		t.Fatalf("quitAllPane = %q, want %%7", quitAllPane)
	}
}

func TestCommandModeUnknownCommandShowsError(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "%7").(model)
	m.mode = inputCommand
	m.input.Prompt = ":"
	m.input.SetValue("bogus")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.err == nil {
		t.Fatal("expected error")
	}
	if got.status != "Unknown command: bogus" {
		t.Fatalf("status = %q", got.status)
	}
}
