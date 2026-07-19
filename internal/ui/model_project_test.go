package ui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRStartsProjectRenameModeWithSelectedSession(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.selectedProject = "small"
	m.sessions = []session{{Name: "tflow-p-8f42ac91"}}
	m.selectedSession = "tflow-p-8f42ac91"
	m.sessionProjects = map[string]string{"tflow-p-8f42ac91": "small"}
	m.sessionLabels = map[string]string{"tflow-p-8f42ac91": "code"}

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputRename {
		t.Fatalf("mode = %v, want inputRename", got.mode)
	}
	if got.renameTarget.project != "small" {
		t.Fatalf("renameTarget = %#v", got.renameTarget)
	}
	if got.input.Value() != "small" {
		t.Fatalf("input = %q, want small", got.input.Value())
	}
}

func TestRenameProjectUpdatesAssignments(t *testing.T) {
	tmp := t.TempDir()
	var synced map[string]string
	m := newModel(fakeTmuxController{
		syncSessionProjects: func(sessionProjects map[string]string) error {
			synced = cloneStringMap(sessionProjects)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "tflow-p-8f42ac91"}}
	m.sessionProjects = map[string]string{"tflow-p-8f42ac91": "small"}
	m.sessionLabels = map[string]string{"tflow-p-8f42ac91": "dev"}
	m.selectedProject = "small"

	updated, cmd := m.Update(projectRenamedMsg{
		oldName: "small",
		newName: "garden",
	})
	got := updated.(model)
	if cmd == nil {
		t.Fatal("expected close command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain old name: %#v", got.projects)
	}
	if !containsString(got.projects, "garden") {
		t.Fatalf("projects missing new name: %#v", got.projects)
	}
	if got.sessionProjects["tflow-p-8f42ac91"] != "garden" {
		t.Fatalf("sessionProjects[tflow-p-8f42ac91] = %q, want garden", got.sessionProjects["tflow-p-8f42ac91"])
	}
	if got.selectedProject != "garden" {
		t.Fatalf("selectedProject = %q, want garden", got.selectedProject)
	}
	if synced["tflow-p-8f42ac91"] != "garden" {
		t.Fatalf("synced project = %#v, want small--dev->garden", synced)
	}
}

func TestProjectRenamePreservesSessionID(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-8f42ac91"}}
	m.sessionProjects = map[string]string{"tflow-p-8f42ac91": "small"}
	m.sessionLabels = map[string]string{"tflow-p-8f42ac91": "code"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-8f42ac91"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	pending := *(updated.(*model))
	if cmd != nil || pending.renameTarget.project != "small" {
		t.Fatalf("project rename state = %#v, cmd = %v", pending.renameTarget, cmd)
	}
	pending.input.SetValue("garden")
	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected project rename command")
	}
	msg := cmd().(projectRenamedMsg)
	if msg.err != nil {
		t.Fatalf("project rename returned error: %v", msg.err)
	}
	pending, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatal("expected menu model after project rename command")
	}
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.selectedSession != "tflow-p-8f42ac91" || got.sessionLabel("tflow-p-8f42ac91") != "code" {
		t.Fatalf("renamed selection = %q, label = %q", got.selectedSession, got.sessionLabel("tflow-p-8f42ac91"))
	}
}

func TestDDeletesProjectWithSelectedSession(t *testing.T) {
	tmp := t.TempDir()
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-8f42ac91"}}
	m.sessionProjects = map[string]string{"tflow-p-8f42ac91": "small"}
	m.sessionLabels = map[string]string{"tflow-p-8f42ac91": "code"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-8f42ac91"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	pending := *(updated.(*model))
	if cmd != nil || pending.deleteTarget.project != "small" {
		t.Fatalf("project delete state = %#v, cmd = %v", pending.deleteTarget, cmd)
	}
	updated, cmd = pending.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected project delete command")
	}
	msg := cmd().(projectDeletedMsg)
	if msg.err != nil {
		t.Fatalf("project delete returned error: %v", msg.err)
	}
	next, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatal("expected menu model after project deletion command")
	}
	pending = next
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if len(got.projects) != 0 || len(got.sessions) != 0 || len(killed) != 1 {
		t.Fatalf("projects = %#v, sessions = %#v, killed = %#v", got.projects, got.sessions, killed)
	}
}

func TestSessionLoadDoesNotMigrateLegacyNames(t *testing.T) {
	m := newModel(fakeTmuxController{}, "code").(model)
	m.projects = []string{"small"}
	m.sessionProjects = map[string]string{"code": "small"}
	updated, cmd := m.Update(sessionsLoadedMsg{sessions: []session{{Name: "code"}}})
	if cmd != nil || updated.(model).currentSession != "code" {
		t.Fatalf("legacy session was migrated: %#v", updated)
	}
}

func TestRenameSessionUpdatesMetadataWithoutTmuxRename(t *testing.T) {
	tmp := t.TempDir()
	var syncedProjects map[string]string
	m := newModel(fakeTmuxController{
		syncSessionProjects: func(projects map[string]string) error {
			syncedProjects = cloneStringMap(projects)
			return nil
		},
	}, "tflow-p-8f42ac92").(model)
	m.statePath = tmp + "/state.json"
	m.sessions = []session{{Name: "tflow-p-8f42ac92"}}
	m.projects = []string{defaultProjectName}
	m.sessionProjects = map[string]string{"tflow-p-8f42ac92": defaultProjectName}
	m.sessionLabels = map[string]string{"tflow-p-8f42ac92": "dev"}
	m.selectedProject = defaultProjectName
	m.selectedSession = "tflow-p-8f42ac92"
	m.mode = inputRename
	m.renameTarget = renameTarget{session: "tflow-p-8f42ac92"}
	m.input.SetValue("lala")

	updated, cmd := m.commitRename()
	got := *(updated.(*model))
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd().(sessionRenamedMsg)
	if msg.err != nil {
		t.Fatalf("rename returned error: %v", msg.err)
	}
	if got.selectedSession != "tflow-p-8f42ac92" {
		t.Fatalf("selectedSession before ack = %q, want tflow-p-8f42ac92", got.selectedSession)
	}
	updated, followUp := got.Update(msg)
	final := updated.(model)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	if final.selectedSession != "tflow-p-8f42ac92" {
		t.Fatalf("selectedSession = %q, want tflow-p-8f42ac92", final.selectedSession)
	}
	if final.currentSession != "tflow-p-8f42ac92" {
		t.Fatalf("currentSession = %q, want tflow-p-8f42ac92", final.currentSession)
	}
	if final.sessionProjects["tflow-p-8f42ac92"] != defaultProjectName {
		t.Fatalf("sessionProjects[tflow-p-8f42ac92] = %q, want %q", final.sessionProjects["tflow-p-8f42ac92"], defaultProjectName)
	}
	if final.sessionLabel("tflow-p-8f42ac92") != "lala" {
		t.Fatalf("session label = %q, want lala", final.sessionLabel("tflow-p-8f42ac92"))
	}
	if _, found := final.findSession("tflow-p-8f42ac92"); !found {
		t.Fatalf("renamed session missing from sessions: %#v", final.sessions)
	}

	if got := syncedProjects["tflow-p-8f42ac92"]; got != defaultProjectName {
		t.Fatalf("synced project = %q, want %q", got, defaultProjectName)
	}

}

func TestProjectSwitchReturnsFocusToFirstProjectSession(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--otter"}, {Name: "small--fox"}}
	m.sessionProjects = map[string]string{"small--otter": "small", "small--fox": "small"}

	updated, cmd := m.switchToProject("small")
	if cmd == nil {
		t.Fatal("expected session switch action")
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "small--otter" {
		t.Fatalf("switch session = %q", msg.switchSession)
	}
	updated, _ = updated.(model).Update(msg)
	got := updated.(model)
	if got.exitAction != menuExitSwitchSession || got.exitSessionName != "small--otter" {
		t.Fatalf("focus restoration = %#v", got)
	}
}

func TestDeletingFinalProjectSessionCreatesVolatileFallback(t *testing.T) {
	var marked string
	m := newModel(fakeTmuxController{
		createSession:       func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { marked = name; return nil },
	}, "").(model)
	m.instanceID = "instance-1"
	m.cwd = "/tmp/workspace"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--otter"}}
	m.sessionProjects = map[string]string{"small--otter": "small"}
	m.sessionLabels = map[string]string{"small--otter": "otter"}
	m.selectedProject = "small"
	m.selectedSession = "small--otter"

	updated, cmd := m.Update(sessionKilledMsg{name: "small--otter"})
	if cmd == nil {
		t.Fatal("expected volatile fallback command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil || !msg.volatile || marked == "" {
		t.Fatalf("fallback = %#v, marked = %q", msg, marked)
	}
	updated, _ = updated.(model).Update(msg)
	got := updated.(model)
	if got.selectedProject != "" || got.selectedSession != msg.session.Name {
		t.Fatalf("volatile fallback selection = %#v", got)
	}
}

func TestDeleteProjectDeletesSessionsInCurrentProject(t *testing.T) {
	tmp := t.TempDir()
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}, {Name: "keep"}}
	m.sessionProjects = map[string]string{
		"dev":  "small",
		"api":  "small",
		"keep": "storage",
	}
	m.selectedProject = "small"

	updated, cmd := m.deleteProject("small")
	pending := updated.(model)
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	msg := cmd().(projectDeletedMsg)
	if msg.err != nil {
		t.Fatalf("delete returned error: %v", msg.err)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"dev", "api"}) {
		t.Fatalf("killed = %#v", killed)
	}
	updated, followUp := pending.Update(msg)
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if followUp == nil {
		t.Fatal("expected session switch command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain deleted project: %#v", got.projects)
	}
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
}

func TestEditProjectRequiresProjectContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)

	updated, cmd := m.editProject()
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.err == nil {
		t.Fatal("expected error")
	}
	if got.status != "No project selected." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestEditProjectStartsInlineSettingsFlow(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Workdir: "/tmp/small"},
	}

	updated, cmd := m.editProject()
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputEditProject {
		t.Fatalf("mode = %v, want inputEditProject", got.mode)
	}
	if got.input.Prompt != "" {
		t.Fatalf("prompt = %q", got.input.Prompt)
	}
	if got.input.Value() != "/tmp/small" {
		t.Fatalf("value = %q", got.input.Value())
	}
}

func TestEditProjectSavesOnlyWorkdirToStoreState(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = tmp + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {
		Name: "small",
	}}

	updated, _ := m.editProject()
	step := updated.(model)
	step.input.SetValue("/tmp/small")
	updated, cmd := step.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	final := *(updated.(*model))
	if cmd == nil || final.mode != inputNone {
		t.Fatalf("unexpected edit result: %#v", final)
	}
	if got := final.projectConfigs["small"].Workdir; got != "/tmp/small" {
		t.Fatalf("workdir = %q", got)
	}
	savedState, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(savedState, "small")
	if !ok {
		t.Fatal("saved project small is missing")
	}
	if got := project.Workdir; got != "/tmp/small" {
		t.Fatalf("saved workdir = %q", got)
	}
}

func TestSaveStatePreservesLoadedSessionOrder(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	state := appState{Projects: []storedProject{{
		Name: "small",
		Sessions: []persistentSession{
			{ID: "tflow-p-first", Label: "first"},
			{ID: "tflow-p-second", Label: "second"},
		},
	}}}
	if err := saveAppState(appStatePath(), state); err != nil {
		t.Fatal(err)
	}
	m, err := buildModel(fakeTmuxController{}, "")
	if err != nil {
		t.Fatal(err)
	}
	m.sessions = []session{{Name: "tflow-p-new"}, {Name: "tflow-p-second"}, {Name: "tflow-p-first"}}
	m.sessionProjects["tflow-p-new"] = "small"
	m.sessionLabels["tflow-p-new"] = "new"
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}
	m.sessions = []session{{Name: "tflow-p-second"}, {Name: "tflow-p-new"}, {Name: "tflow-p-first"}}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	savedState, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(savedState, "small")
	if !ok {
		t.Fatalf("stored projects = %#v", savedState.Projects)
	}
	want := []string{"tflow-p-first", "tflow-p-second", "tflow-p-new"}
	if len(project.Sessions) != len(want) {
		t.Fatalf("stored sessions = %#v", project.Sessions)
	}
	for index, id := range want {
		if got := project.Sessions[index].ID; got != id {
			t.Fatalf("session %d = %q, want %q", index, got, id)
		}
	}
}
