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

func TestMoveProjectNavigationCycles(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small", "garden", "forest", "desert"}
	m.sessions = []session{{Name: "tflow-p-1"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	got := *(updated.(*model))
	if got.moveProjectIndex != 0 {
		t.Fatalf("initial index = %d, want 0", got.moveProjectIndex)
	}

	// Down arrow moves to index 1
	updated, _ = got.updateModal(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(model)
	if got.moveProjectIndex != 1 {
		t.Fatalf("index after down = %d, want 1", got.moveProjectIndex)
	}

	// Down arrow moves to index 2
	updated, _ = got.updateModal(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(model)
	if got.moveProjectIndex != 2 {
		t.Fatalf("index after down = %d, want 2", got.moveProjectIndex)
	}

	// Down arrow wraps to 0
	updated, _ = got.updateModal(tea.KeyMsg{Type: tea.KeyDown})
	got = updated.(model)
	if got.moveProjectIndex != 0 {
		t.Fatalf("index after wrap down = %d, want 0", got.moveProjectIndex)
	}

	// Up arrow wraps to 2
	updated, _ = got.updateModal(tea.KeyMsg{Type: tea.KeyUp})
	got = updated.(model)
	if got.moveProjectIndex != 2 {
		t.Fatalf("index after wrap up = %d, want 2", got.moveProjectIndex)
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
	if msg.switchSession != "tflow-p-1" {
		t.Fatalf("switch msg = %#v, want moved session", msg)
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

func TestMoveSessionSwitchesClientToMovedSession(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	m := newModel(fakeTmuxController{}, "tflow-p-1").(model)
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
	if cmd == nil {
		t.Fatal("expected moved-session switch command")
	}
	action := cmd().(menuActionMsg)
	if action.switchSession != "tflow-p-1" {
		t.Fatalf("switch session = %q, want moved session", action.switchSession)
	}
	final, _ := got.Update(action)
	var switched []string
	if err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { switched = append(switched, name); return nil },
	}, final); err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(switched), fmt.Sprint([]string{"tflow-p-1"}); got != want {
		t.Fatalf("switch-client calls = %s, want %s", got, want)
	}
	if got.currentProject() != "garden" {
		t.Fatalf("currentProject() = %q, want garden after moving the active session", got.currentProject())
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
	if msg.switchSession != "tflow-p-1" {
		t.Fatalf("switch msg = %#v, want moved session", msg)
	}
}

// TestSuccessfulSessionMoveDoesNotLoseConcurrentState guards against a
// three-way merge data-loss race. mutateAppState reloads the latest on-disk
// state before applying MoveSession, so the state returned to
// applySessionMove can include a project or label change written by another
// tflow instance since this popup's model was built. applySessionMove only
// folds the moved session into m.projects/m.sessionProjects/m.sessionLabels;
// if it anchored stateBase to that broader returned state instead of to its
// own tracked view, a later unrelated saveState from this sidebar would see
// the concurrent entries as present in base but missing from desired and
// delete or revert them (mergeAppStates in helpers.go treats that shape as
// an intentional deletion).
func TestSuccessfulSessionMoveDoesNotLoseConcurrentState(t *testing.T) {
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
	// Anchor stateBase to what this model actually observed for statePath,
	// mirroring buildModel, so the concurrent write below is only visible
	// through the mutation's own "latest" reload, never through this
	// model's tracked base.
	initial, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	m.stateBase = initial
	m.stateBasePath = statePath

	// Simulate a second tflow instance writing to the shared store after
	// this popup opened: it adds a brand-new project with its own session
	// and relabels an existing, unrelated session this model already knows
	// about.
	if _, err := mutateAppState(statePath, func(latest appState) (appState, error) {
		latest.Projects = append(latest.Projects, storedProject{
			Name:     "concurrent",
			Sessions: []persistentSession{{ID: "tflow-p-99", Label: "otter99"}},
		})
		for i := range latest.Projects {
			if latest.Projects[i].Name != "garden" {
				continue
			}
			for j := range latest.Projects[i].Sessions {
				if latest.Projects[i].Sessions[j].ID == "tflow-p-9" {
					latest.Projects[i].Sessions[j].Label = "wasp"
				}
			}
		}
		return latest, nil
	}); err != nil {
		t.Fatalf("simulate concurrent write: %v", err)
	}

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

	// A later, unrelated save from this same sidebar (e.g. a subsequent
	// rename) must not delete or revert the concurrent instance's writes.
	if err := got.saveState(); err != nil {
		t.Fatalf("saveState after move: %v", err)
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	concurrent, ok := storedProjectByName(saved, "concurrent")
	if !ok {
		t.Fatal("concurrent project was deleted by a later save after a successful move")
	}
	if len(concurrent.Sessions) != 1 || concurrent.Sessions[0].ID != "tflow-p-99" || concurrent.Sessions[0].Label != "otter99" {
		t.Fatalf("concurrent project sessions = %#v, want unchanged tflow-p-99=otter99", concurrent.Sessions)
	}
	garden, ok := storedProjectByName(saved, "garden")
	if !ok {
		t.Fatal("garden project missing after move and save")
	}
	var gardenLabel string
	for _, s := range garden.Sessions {
		if s.ID == "tflow-p-9" {
			gardenLabel = s.Label
		}
	}
	if gardenLabel != "wasp" {
		t.Fatalf("tflow-p-9 label = %q, want concurrent relabel to survive the later save", gardenLabel)
	}
}

// TestSuccessfulSessionMoveWritesReloadedLabelNotStaleModelLabel guards
// against another instance of the same stale-model-vs-freshly-reloaded-store
// race: store.MoveSession is a pure project reassignment that preserves
// whatever label the moved session currently has on disk. If another tflow
// instance renames the session being moved after this popup's model was
// built but before Enter is pressed, the locked mutation reloads that new
// label before applying the move. The tmux label marker write must use that
// reloaded label, not m.sessionLabels (the popup's stale, pre-mutation
// model), or it silently writes the old name back into tmux even though the
// store now has the new one.
func TestSuccessfulSessionMoveWritesReloadedLabelNotStaleModelLabel(t *testing.T) {
	tmp := t.TempDir()
	statePath := tmp + "/state.json"
	seedMoveState(t, statePath,
		storedProject{Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		storedProject{Name: "garden", Sessions: []persistentSession{{ID: "tflow-p-9", Label: "bee"}}},
	)

	var labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, fmt.Sprintf("%s=%s", name, label))
			return nil
		},
	}, "").(model)
	m.statePath = statePath
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-9": "garden"}
	// This model's own bookkeeping still has the pre-rename label: it was
	// built before the concurrent rename below happened.
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-9": "bee"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}, "garden": {Name: "garden"}}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-1"

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	pending := *(updated.(*model))

	// Simulate a second tflow instance renaming the session being moved,
	// after this popup opened but before Enter is pressed.
	if _, err := mutateAppState(statePath, func(latest appState) (appState, error) {
		for i := range latest.Projects {
			if latest.Projects[i].Name != "small" {
				continue
			}
			for j := range latest.Projects[i].Sessions {
				if latest.Projects[i].Sessions[j].ID == "tflow-p-1" {
					latest.Projects[i].Sessions[j].Label = "raccoon"
				}
			}
		}
		return latest, nil
	}); err != nil {
		t.Fatalf("simulate concurrent rename: %v", err)
	}

	updated, _ = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if got.err != nil {
		t.Fatalf("move reported error: %v", got.err)
	}

	if fmt.Sprint(labelWrites) != fmt.Sprint([]string{"tflow-p-1=raccoon"}) {
		t.Fatalf("label marker writes = %#v, want the reloaded label raccoon, not the stale model label otter", labelWrites)
	}

	saved, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	garden, ok := storedProjectByName(saved, "garden")
	if !ok || len(garden.Sessions) != 2 || garden.Sessions[1].ID != "tflow-p-1" || garden.Sessions[1].Label != "raccoon" {
		t.Fatalf("persisted target project = %#v, want tflow-p-1 to keep its concurrently renamed label", garden)
	}
	if got.sessionLabels["tflow-p-1"] != "raccoon" {
		t.Fatalf("in-memory session label = %q, want the reloaded label raccoon, not the stale model label otter", got.sessionLabels["tflow-p-1"])
	}
	if got.status != "Moved raccoon to garden." {
		t.Fatalf("status = %q, want the success message to reflect the reloaded label", got.status)
	}
}
