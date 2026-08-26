package ui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRStartsProjectRenameModeWithSelectedSession(t *testing.T) {
	m := newMenu().(model)
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
	synced := map[string]string{}
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			synced[name] = project
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

// TestRenamingProjectWritesMarkersOnlyForItsOwnSessions proves that renaming
// a project rewrites the tmux project marker only for sessions that belong
// to the renamed project, leaving sessions in every other project
// untouched.
func TestRenamingProjectWritesMarkersOnlyForItsOwnSessions(t *testing.T) {
	tmp := t.TempDir()
	var projectWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small", "garden"}
	m.projectConfigs = map[string]projectConfig{
		"small":  {Name: "small", Workdir: "/small"},
		"garden": {Name: "garden", Workdir: "/garden"},
	}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-2"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-2": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-2": "fox", "tflow-p-9": "bee"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-1", "tflow-p-2"}, "garden": {"tflow-p-9"}}
	m.selectedProject = "small"

	updated, cmd := m.Update(projectRenamedMsg{oldName: "small", newName: "garage"})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("project rename reported error: %v", got.err)
	}

	want := []string{"tflow-p-1=garage", "tflow-p-2=garage"}
	if fmt.Sprint(projectWrites) != fmt.Sprint(want) {
		t.Fatalf("project marker writes = %#v, want only the renamed project's sessions: %#v", projectWrites, want)
	}
}

// TestRenamingProjectSkipsVanishedSessions proves that if a session from the
// renamed project was killed in tmux outside tflow after the sidebar loaded,
// the resulting "can't find session" error from SetSessionProject is
// tolerated: the vanished session is skipped and the remaining sessions in
// the same project still get their marker updated, instead of the loop
// stopping early and surfacing a spurious failure.
func TestRenamingProjectSkipsVanishedSessions(t *testing.T) {
	tmp := t.TempDir()
	var projectWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			if name == "tflow-p-1" {
				return errors.New("can't find session: tflow-p-1")
			}
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small", "garden"}
	m.projectConfigs = map[string]projectConfig{
		"small":  {Name: "small", Workdir: "/small"},
		"garden": {Name: "garden", Workdir: "/garden"},
	}
	m.sessions = []session{{Name: "tflow-p-1"}, {Name: "tflow-p-2"}, {Name: "tflow-p-9"}}
	m.sessionProjects = map[string]string{"tflow-p-1": "small", "tflow-p-2": "small", "tflow-p-9": "garden"}
	m.sessionLabels = map[string]string{"tflow-p-1": "otter", "tflow-p-2": "fox", "tflow-p-9": "bee"}
	m.persistentSessionOrder = map[string][]string{"small": {"tflow-p-1", "tflow-p-2"}, "garden": {"tflow-p-9"}}
	m.selectedProject = "small"

	updated, cmd := m.Update(projectRenamedMsg{oldName: "small", newName: "garage"})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("project rename reported error for an already-vanished session: %v", got.err)
	}

	want := []string{"tflow-p-2=garage"}
	if fmt.Sprint(projectWrites) != fmt.Sprint(want) {
		t.Fatalf("project marker writes = %#v, want the vanished session skipped and the rest written: %#v", projectWrites, want)
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
	}, "other").(model)
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
	var labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, name+"="+label)
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

	if fmt.Sprint(labelWrites) != fmt.Sprint([]string{"tflow-p-8f42ac92=lala"}) {
		t.Fatalf("label marker writes = %#v, want exactly one write for the renamed session", labelWrites)
	}
}

// TestRenamingSessionWritesNoMarkersForUnrelatedSessions proves that
// renaming a session's label never rewrites tmux markers for any other
// session, even with a large fleet of unrelated sessions present: the
// rename command writes only the target session's label marker directly,
// and no full-fleet resync follows.
func TestRenamingSessionWritesNoMarkersForUnrelatedSessions(t *testing.T) {
	tmp := t.TempDir()
	var projectWrites, labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, name+"="+label)
			return nil
		},
	}, "tflow-p-target").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/small"}}
	m.sessions = []session{{Name: "tflow-p-target"}}
	m.sessionProjects = map[string]string{"tflow-p-target": "small"}
	m.sessionLabels = map[string]string{"tflow-p-target": "otter"}
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("tflow-p-existing-%02d", i)
		m.sessions = append(m.sessions, session{Name: name})
		m.sessionProjects[name] = "small"
		m.sessionLabels[name] = fmt.Sprintf("animal-%02d", i)
	}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-target"
	m.mode = inputRename
	m.renameTarget = renameTarget{session: "tflow-p-target"}
	m.input.SetValue("newname")

	updated, cmd := m.commitRename()
	pending := *(updated.(*model))
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd().(sessionRenamedMsg)
	if msg.err != nil {
		t.Fatalf("rename returned error: %v", msg.err)
	}

	updated2, _ := pending.Update(msg)
	got := updated2.(model)
	if got.err != nil {
		t.Fatalf("rename ack reported error: %v", got.err)
	}

	if len(projectWrites) != 0 {
		t.Fatalf("project marker writes = %#v, want none for a label-only rename", projectWrites)
	}
	if fmt.Sprint(labelWrites) != fmt.Sprint([]string{"tflow-p-target=newname"}) {
		t.Fatalf("label marker writes = %#v, want exactly one write for the renamed session", labelWrites)
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

// TestPopupFromPersistentSessionCanCreateVolatileFallbackAfterDeletingFinalProject
// models a popup opened while the active session is persistent, whose
// instance ID was recovered via the tmux layer's client-scoped fallback
// (internal/tmux's resolveInstanceID) rather than a session marker --
// unlike TestDeletingFinalProjectSessionCreatesVolatileFallback, which sets
// m.instanceID directly, this builds the model the same way a real popup
// process does: reading TFLOW_INSTANCE_ID from its environment. Before the
// tmux-layer fix, that env var would be empty for a persistent-session
// popup, and createVolatileFallback would fail with "instance id is empty"
// instead of succeeding here.
func TestPopupFromPersistentSessionCanCreateVolatileFallbackAfterDeletingFinalProject(t *testing.T) {
	t.Setenv(menuInstanceEnv, "instance-1")
	var marked string
	m := newModel(fakeTmuxController{
		createSession:       func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { marked = name; return nil },
	}, "").(model)
	if m.instanceID != "instance-1" {
		t.Fatalf("model instanceID = %q, want instance-1 from the environment", m.instanceID)
	}
	m.cwd = "/tmp/workspace"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--otter"}}
	m.sessionProjects = map[string]string{"small--otter": "small"}
	m.sessionLabels = map[string]string{"small--otter": "otter"}
	m.selectedProject = "small"
	m.selectedSession = "small--otter"

	_, cmd := m.Update(sessionKilledMsg{name: "small--otter"})
	if cmd == nil {
		t.Fatal("expected volatile fallback command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("volatile fallback failed: %v", msg.err)
	}
	if !msg.volatile || marked == "" {
		t.Fatalf("fallback = %#v, marked = %q, want a volatile fallback session", msg, marked)
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
	}, "keep").(model)
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
	if followUp != nil {
		t.Fatal("deleted non-active project must not switch sessions")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain deleted project: %#v", got.projects)
	}
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
}

func TestDeleteNonActiveProjectDoesNotSwitchClient(t *testing.T) {
	tmp := t.TempDir()
	var killed []string
	var switched []string
	var created []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			created = append(created, name)
			return session{Name: name}, nil
		},
	}, "keep").(model)
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
	if followUp != nil {
		t.Fatal("expected no session switch command for non-active project deletion")
	}
	if got.currentSession != "keep" {
		t.Fatalf("currentSession = %q, want keep (unchanged)", got.currentSession)
	}
	if len(switched) != 0 || len(created) != 0 {
		t.Fatalf("unexpected tmux calls: switched=%#v created=%#v", switched, created)
	}
}

func TestKillNonActiveSessionDoesNotSwitchClient(t *testing.T) {
	tmp := t.TempDir()
	var switched []string
	var created []string
	m := newModel(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			created = append(created, name)
			return session{Name: name}, nil
		},
	}, "keep").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}, {Name: "keep"}}
	m.sessionProjects = map[string]string{
		"dev":  "small",
		"api":  "small",
		"keep": "storage",
	}

	updated, cmd := m.Update(sessionKilledMsg{name: "dev", project: "small"})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no session switch command for non-active session deletion")
	}
	if got.currentSession != "keep" {
		t.Fatalf("currentSession = %q, want keep (unchanged)", got.currentSession)
	}
	if len(switched) != 0 || len(created) != 0 {
		t.Fatalf("unexpected tmux calls: switched=%#v created=%#v", switched, created)
	}
	if _, exists := got.findSession("dev"); exists {
		t.Fatal("deleted session remains")
	}
}

func TestDeletingActiveProjectFallbackUsesActivePaneWorkdir(t *testing.T) {
	tmp := t.TempDir()
	const distinctiveCwd = "/tmp/distinctive-active-pane-cwd"
	const deletedSession = "small--otter"
	statePath := tmp + "/state.json"
	if err := saveAppState(statePath, appState{Projects: []storedProject{{
		Name: "small", Workdir: distinctiveCwd, Sessions: []persistentSession{{ID: deletedSession, Label: "otter"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var createdCwd string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			createdCwd = cwd
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
	}, deletedSession).(model)
	m.instanceID = "instance-1"
	m.cwd = distinctiveCwd
	m.statePath = statePath
	m.projects = []string{"small"}
	m.sessions = []session{{Name: deletedSession}}
	m.sessionProjects = map[string]string{deletedSession: "small"}
	m.sessionLabels = map[string]string{deletedSession: "otter"}
	m.selectedProject = "small"

	updated, cmd := m.deleteProject("small")
	pending := updated.(model)
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}
	created := cmd().(sessionCreatedMsg)
	if created.err != nil || !created.volatile {
		t.Fatalf("fallback session = %+v", created)
	}
	if createdCwd != distinctiveCwd {
		t.Fatalf("fallback cwd = %q, want %q", createdCwd, distinctiveCwd)
	}

	updated, switchCmd := pending.Update(created)
	if switchCmd == nil {
		t.Fatal("expected fallback switch command")
	}
	action := switchCmd().(menuActionMsg)
	if action.switchSession != created.session.Name || fmt.Sprint(action.deleteSessions) != fmt.Sprint([]string{deletedSession}) {
		t.Fatalf("fallback action = %+v", action)
	}
	final, _ := updated.(model).Update(action)

	var switched, killed []string
	if err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { switched = append(switched, name); return nil },
		killSession:  func(name string) error { killed = append(killed, name); return nil },
	}, final); err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(switched), fmt.Sprint([]string{created.session.Name}); got != want {
		t.Fatalf("switched = %s, want %s", got, want)
	}
	if got, want := fmt.Sprint(killed), fmt.Sprint([]string{deletedSession}); got != want {
		t.Fatalf("killed = %s, want %s", got, want)
	}
	persisted, err := loadAppState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 0 {
		t.Fatalf("persisted state = %+v, want deleted project removed", persisted)
	}
}

func TestFailedFinalProjectFallbackDoesNotArmLaterSwitch(t *testing.T) {
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{}, fmt.Errorf("create failed")
		},
	}, "small--otter").(model)
	m.instanceID = "instance-1"
	m.projects = []string{"small", "garden"}
	m.sessions = []session{{Name: "small--otter"}, {Name: "garden--bee"}}
	m.sessionProjects = map[string]string{"small--otter": "small", "garden--bee": "garden"}
	m.sessionLabels = map[string]string{"small--otter": "otter", "garden--bee": "bee"}
	m.selectedProject = "small"
	m.selectedSession = "small--otter"

	updated, cmd := m.deleteProject("small")
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}
	failed := cmd().(sessionCreatedMsg)
	if failed.err == nil {
		t.Fatal("expected fallback creation failure")
	}
	updated, _ = updated.(model).Update(failed)
	got := updated.(model)
	if len(got.deferredDelete) != 0 {
		t.Fatalf("deferred deletion = %#v, want cleared after fallback failure", got.deferredDelete)
	}

	got.selectedProject = "garden"
	got.selectedSession = "garden--bee"
	_, switchCmd := got.switchSelectedSession()
	if switchCmd == nil {
		t.Fatal("expected explicit session switch command")
	}
	action := switchCmd().(menuActionMsg)
	if len(action.deleteSessions) != 0 {
		t.Fatalf("later switch action = %#v, must not delete the failed project", action)
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

func TestSplitShellWords(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr bool
	}{
		{input: "", want: nil},
		{input: "   ", want: nil},
		{input: "nvim", want: []string{"nvim"}},
		{input: "nvim -c 'set ft=yaml'", want: []string{"nvim", "-c", "set ft=yaml"}},
		{input: `code --wait --new-window`, want: []string{"code", "--wait", "--new-window"}},
		{input: `"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl" -w`, want: []string{"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl", "-w"}},
		{input: `vim "quoted with \"escaped\" quotes"`, want: []string{"vim", `quoted with "escaped" quotes`}},
		{input: `unclosed 'quote`, wantErr: true},
		{input: `unclosed "quote`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := splitShellWords(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitShellWords(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitShellWords(%q) unexpected error: %v", tt.input, err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitShellWords(%q) = %#v, want %#v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("splitShellWords(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestResolveEditorCommand(t *testing.T) {
	t.Run("defaults to nvim", func(t *testing.T) {
		t.Setenv("EDITOR", "")
		cmd, err := resolveEditorCommand("/tmp/test.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != "nvim" && !strings.HasSuffix(cmd.Path, "/nvim") {
			t.Fatalf("expected nvim, got %q", cmd.Path)
		}
		if len(cmd.Args) != 2 || cmd.Args[1] != "/tmp/test.yaml" {
			t.Fatalf("args = %v", cmd.Args)
		}
	})

	t.Run("respects custom EDITOR with flags", func(t *testing.T) {
		t.Setenv("EDITOR", "code --wait")
		cmd, err := resolveEditorCommand("/tmp/test.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != "code" && !strings.HasSuffix(cmd.Path, "/code") {
			t.Fatalf("expected code, got %q", cmd.Path)
		}
		if len(cmd.Args) != 3 || cmd.Args[1] != "--wait" || cmd.Args[2] != "/tmp/test.yaml" {
			t.Fatalf("args = %v", cmd.Args)
		}
	})

	t.Run("preserves quoted arguments with spaces", func(t *testing.T) {
		t.Setenv("EDITOR", `nvim -c "set ft=yaml"`)
		cmd, err := resolveEditorCommand("/tmp/test.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != "nvim" && !strings.HasSuffix(cmd.Path, "/nvim") {
			t.Fatalf("expected nvim, got %q", cmd.Path)
		}
		if len(cmd.Args) != 4 || cmd.Args[1] != "-c" || cmd.Args[2] != "set ft=yaml" || cmd.Args[3] != "/tmp/test.yaml" {
			t.Fatalf("args = %v", cmd.Args)
		}
	})

	t.Run("preserves quoted executable path with spaces", func(t *testing.T) {
		t.Setenv("EDITOR", `"/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl" -w`)
		cmd, err := resolveEditorCommand("/tmp/test.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if cmd.Path != "/Applications/Sublime Text.app/Contents/SharedSupport/bin/subl" {
			t.Fatalf("expected subl path, got %q", cmd.Path)
		}
		if len(cmd.Args) != 3 || cmd.Args[1] != "-w" || cmd.Args[2] != "/tmp/test.yaml" {
			t.Fatalf("args = %v", cmd.Args)
		}
	})
}

func TestActiveTempFilesCleanup(t *testing.T) {
	tempFile, err := os.CreateTemp("", "tflow-test-active-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()

	registerTempFile(tempPath)
	cleanupActiveTempFiles()

	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp file %q to be removed by cleanupActiveTempFiles", tempPath)
	}
}

func TestFormatAndParseProjectSettingsYAML(t *testing.T) {
	cfg := projectConfig{
		Name:    "myproj",
		Workdir: "/path/to/myproj",
	}
	data, err := formatProjectSettingsYAML(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "workdir: /path/to/myproj") {
		t.Fatalf("formatted yaml missing workdir: %s", string(data))
	}

	doc, err := parseProjectSettingsYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Workdir != "/path/to/myproj" {
		t.Fatalf("workdir = %q", doc.Workdir)
	}

	t.Run("rejects unknown keys", func(t *testing.T) {
		invalid := []byte("workdir: /tmp\nextra_key: value\n")
		_, err := parseProjectSettingsYAML(invalid)
		if err == nil {
			t.Fatal("expected error on unknown key")
		}
	})

	t.Run("rejects multiple documents", func(t *testing.T) {
		multiple := []byte("workdir: /tmp\n---\nworkdir: /other\n")
		_, err := parseProjectSettingsYAML(multiple)
		if err == nil {
			t.Fatal("expected error on multiple documents")
		}
	})
}

func TestEditProjectGeneratesYAMLAndExecutesEditor(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Workdir: "/tmp/small"},
	}

	var executedPath string
	oldResolve := resolveEditorCommand
	defer func() { resolveEditorCommand = oldResolve }()
	resolveEditorCommand = func(tempFile string) (*exec.Cmd, error) {
		executedPath = tempFile
		info, err := os.Stat(tempFile)
		if err != nil {
			t.Fatalf("stat temp file: %v", err)
		}
		if mode := info.Mode().Perm(); mode != 0600 {
			t.Fatalf("file mode = %v, want 0600", mode)
		}
		content, err := os.ReadFile(tempFile)
		if err != nil {
			t.Fatalf("read temp file: %v", err)
		}
		if !strings.Contains(string(content), "workdir: /tmp/small") {
			t.Fatalf("content = %s", string(content))
		}
		return exec.Command("true", tempFile), nil
	}

	updated, cmd := m.editProject()
	if cmd == nil {
		t.Fatal("expected command from editProject")
	}
	_ = updated
	if executedPath == "" {
		t.Fatal("expected editor to be invoked with temp file path")
	}
	_ = os.Remove(executedPath)
}

func TestEditProjectHandlesFinishedSuccessAndTildeExpansion(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = tmp + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {
		Name: "small",
	}}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	tempFile, err := os.CreateTemp(tmp, "settings-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.WriteString("workdir: ~/myproject\n"); err != nil {
		t.Fatal(err)
	}
	_ = tempFile.Close()

	updated, cmd := m.handleProjectEditorFinished(projectEditorFinishedMsg{
		project:  "small",
		tempPath: tempPath,
		err:      nil,
	})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	final := updated.(model)
	if final.err != nil {
		t.Fatalf("unexpected error: %v", final.err)
	}

	// Verify temp file is removed
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp file to be removed, got err: %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedWorkdir := filepath.Join(home, "myproject")
	if got := final.projectConfigs["small"].Workdir; got != expectedWorkdir {
		t.Fatalf("workdir = %q, want %q", got, expectedWorkdir)
	}

	savedState, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(savedState, "small")
	if !ok {
		t.Fatal("saved project small is missing")
	}
	if got := project.Workdir; got != expectedWorkdir {
		t.Fatalf("saved workdir = %q, want %q", got, expectedWorkdir)
	}
}

func TestEditProjectRejectsUnknownYAMLFieldsAndPreservesStore(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = tmp + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {
		Name:    "small",
		Workdir: "/original/workdir",
	}}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	tempFile, err := os.CreateTemp(tmp, "settings-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := tempFile.Name()
	if _, err := tempFile.WriteString("workdir: /new/workdir\nunknown_field: true\n"); err != nil {
		t.Fatal(err)
	}
	_ = tempFile.Close()

	updated, cmd := m.handleProjectEditorFinished(projectEditorFinishedMsg{
		project:  "small",
		tempPath: tempPath,
		err:      nil,
	})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	final := updated.(model)
	if final.err == nil {
		t.Fatal("expected error on unknown YAML field")
	}
	if !strings.Contains(final.status, "Invalid settings") {
		t.Fatalf("status = %q", final.status)
	}

	// Verify temp file is removed
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp file to be removed, got err: %v", err)
	}

	savedState, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(savedState, "small")
	if !ok || project.Workdir != "/original/workdir" {
		t.Fatalf("store modified unexpectedly: %#v", project)
	}
}

func TestEditProjectHandlesEditorErrorAndPreservesStore(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = tmp + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {
		Name:    "small",
		Workdir: "/original/workdir",
	}}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	tempFile, err := os.CreateTemp(tmp, "settings-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()

	updated, _ := m.handleProjectEditorFinished(projectEditorFinishedMsg{
		project:  "small",
		tempPath: tempPath,
		err:      fmt.Errorf("editor exited with status 1"),
	})
	final := updated.(model)
	if final.err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(final.status, "Editor error") {
		t.Fatalf("status = %q", final.status)
	}

	// Verify temp file is removed
	if _, err := os.Stat(tempPath); !os.IsNotExist(err) {
		t.Fatalf("expected temp file to be removed, got err: %v", err)
	}

	savedState, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(savedState, "small")
	if !ok || project.Workdir != "/original/workdir" {
		t.Fatalf("store modified unexpectedly: %#v", project)
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
