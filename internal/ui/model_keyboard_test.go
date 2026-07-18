package ui

import (
	"fmt"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

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
