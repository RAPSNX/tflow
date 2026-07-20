package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func seedMoveState(t *testing.T, statePath string, projects ...storedProject) {
	t.Helper()
	if err := saveAppState(statePath, appState{Projects: projects}); err != nil {
		t.Fatalf("seed store state: %v", err)
	}
}

func TestMStartsSessionMoveModeExcludingCurrentProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputMoveSession {
		t.Fatalf("mode = %v, want inputMoveSession", got.mode)
	}
	if got.moveTarget.session != "tflow-p-1" {
		t.Fatalf("moveTarget = %#v", got.moveTarget)
	}
	matches := got.matchingMoveProjects("")
	if len(matches) != 1 || matches[0] != "garden" {
		t.Fatalf("matching move projects = %#v, want [garden]", matches)
	}
}

func TestMRequiresASelectedSession(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.status != "No session selected." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestMRejectsVolatileSessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.sessions = []session{{Name: "scratch-temp", Temporary: true, Instance: "instance-1"}}
	m.instanceID = "instance-1"
	m.selectedSession = "scratch-temp"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.status != "Volatile sessions cannot be moved." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestMWithNoOtherProjectReportsStatus(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-1"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.status != "No other project to move into." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestMoveSessionMovesBetweenProjectsAndWritesOnlyItsMarkers(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	var projectWrites, labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, fmt.Sprintf("%s=%s", name, project))
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, fmt.Sprintf("%s=%s", name, label))
			return nil
		},
	}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "bee"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/small"}, "garden": {Name: "garden", Workdir: "/garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}

	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected close command after a successful move")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}

	if got.err != nil {
		t.Fatalf("move reported error: %v (status %q)", got.err, got.status)
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone after a successful move", got.mode)
	}
	if got.sessionProjects["tflow-p-1"] != "garden" {
		t.Fatalf("sessionProjects[tflow-p-1] = %q, want garden", got.sessionProjects["tflow-p-1"])
	}
	if got.sessionLabel("tflow-p-1") != "otter" {
		t.Fatalf("moved session label = %q, want otter (unchanged)", got.sessionLabel("tflow-p-1"))
	}
	if !containsSessionName([]session{{Name: "tflow-p-1"}}, "tflow-p-1") {
		t.Fatal("sanity check helper broken")
	}
	targetSessions := got.projectSessions("garden")
	if len(targetSessions) != 2 || targetSessions[0].Name != "tflow-p-9" || targetSessions[1].Name != "tflow-p-1" {
		t.Fatalf("target project sessions = %#v, want [tflow-p-9 tflow-p-1]", targetSessions)
	}
	if len(got.projectSessions("small")) != 0 {
		t.Fatalf("source project still has sessions: %#v", got.projectSessions("small"))
	}

	if fmt.Sprint(projectWrites) != fmt.Sprint([]string{"tflow-p-1=garden"}) {
		t.Fatalf("project marker writes = %#v, want only the moved session", projectWrites)
	}
	if fmt.Sprint(labelWrites) != fmt.Sprint([]string{"tflow-p-1=otter"}) {
		t.Fatalf("label marker writes = %#v, want only the moved session", labelWrites)
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	small, smallExists := storedProjectByName(saved, "small")
	if smallExists {
		t.Fatalf("source project still persisted: %#v", small)
	}
	garden, ok := storedProjectByName(saved, "garden")
	if !ok || len(garden.Sessions) != 2 || garden.Sessions[1].ID != "tflow-p-1" || garden.Sessions[1].Label != "otter" {
		t.Fatalf("persisted target project = %#v", garden)
	}
}

func TestMoveSessionRejectsLabelCollisionAndDoesNotMutateState(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "otter"}}},
	)

	var projectWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name)
			return nil
		},
	}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "otter"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	updated, cmd := pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}

	if got.err == nil {
		t.Fatal("expected a label collision error")
	}
	if got.status != got.err.Error() {
		t.Fatalf("status = %q, want it to report the error", got.status)
	}
	if got.sessionProjects["tflow-p-1"] != "small" {
		t.Fatalf("sessionProjects[tflow-p-1] = %q, want small (unchanged)", got.sessionProjects["tflow-p-1"])
	}
	if !containsString(got.projects, "small") {
		t.Fatal("source project was removed despite rejected move")
	}
	if len(projectWrites) != 0 {
		t.Fatalf("tmux marker writes = %#v, want none for a rejected move", projectWrites)
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	small, ok := storedProjectByName(saved, "small")
	if !ok || len(small.Sessions) != 1 || small.Sessions[0].ID != "tflow-p-1" {
		t.Fatalf("persisted source project changed: %#v", small)
	}
	garden, ok := storedProjectByName(saved, "garden")
	if !ok || len(garden.Sessions) != 1 || garden.Sessions[0].ID != "tflow-p-9" {
		t.Fatalf("persisted target project changed: %#v", garden)
	}
}

func TestMoveSessionOutOfFinalProjectSessionDeletesSourceProject(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "bee"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	updated, _ = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if got.err != nil {
		t.Fatalf("move reported error: %v", got.err)
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain emptied source project: %#v", got.projects)
	}
	if _, exists := got.projectConfigs["small"]; exists {
		t.Fatal("source project config metadata remains")
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := storedProjectByName(saved, "small"); exists {
		t.Fatal("source project persisted after its final session moved out")
	}
}

func TestMoveSessionDoesNotSwitchClientForActiveSession(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	var switched []string
	m := newModel(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
	}, "tflow-p-1").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "bee"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	updated, _ = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if got.err != nil {
		t.Fatalf("move reported error: %v", got.err)
	}
	if len(switched) != 0 {
		t.Fatalf("switch-client calls = %#v, want none: moving preserves the active session's tmux identity", switched)
	}
	if got.currentSession != "tflow-p-1" {
		t.Fatalf("currentSession = %q, want unchanged tflow-p-1", got.currentSession)
	}
	if got.currentProject() != "garden" {
		t.Fatalf("currentProject() = %q, want garden reflected without a client switch", got.currentProject())
	}
}

func TestEscCancelsSessionMoveWithoutMutatingState(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	updated, cmd := pending.updateModal(tea.KeyMsg{Type: tea.KeyEsc})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.sessionProjects["tflow-p-1"] != "small" {
		t.Fatalf("sessionProjects[tflow-p-1] = %q, want small (unchanged)", got.sessionProjects["tflow-p-1"])
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	small, ok := storedProjectByName(saved, "small")
	if !ok || len(small.Sessions) != 1 {
		t.Fatalf("persisted source project changed after cancelled move: %#v", small)
	}
}

// TestSuccessfulSessionMoveClosesSidebar guards against the successful-move
// path returning nil instead of the shared close command. Per
// .codex/ARCHITECTURE.md, successful sidebar actions close the popup and
// return focus to the active terminal, matching every other successful
// mutation (session creation, rename, deletion).
func TestSuccessfulSessionMoveClosesSidebar(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "bee"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	updated, cmd := pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if got.err != nil {
		t.Fatalf("move reported error: %v", got.err)
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone after a successful move", got.mode)
	}
	if cmd == nil {
		t.Fatal("expected close command after a successful move, got nil (popup would stay open)")
	}
	msg, ok := cmd().(menuActionMsg)
	if !ok {
		t.Fatalf("cmd() = %T, want menuActionMsg", cmd())
	}
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if msg.switchSession != "" {
		t.Fatalf("close msg = %#v, want plain close", msg)
	}
}
