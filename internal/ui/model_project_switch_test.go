package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	runtmux "tflow/internal/tmux"
)

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
