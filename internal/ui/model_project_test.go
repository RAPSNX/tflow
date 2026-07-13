package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRStartsProjectRenameMode(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputRename {
		t.Fatalf("mode = %v, want inputRename", got.mode)
	}
	if got.renameRow.kind != rowProject || got.renameRow.project != "small" {
		t.Fatalf("renameRow = %#v", got.renameRow)
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
	}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"

	updated, cmd := m.Update(projectRenamedMsg{oldName: "small", newName: "garden"})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain old name: %#v", got.projects)
	}
	if !containsString(got.projects, "garden") {
		t.Fatalf("projects missing new name: %#v", got.projects)
	}
	if got.sessionProjects["dev"] != "garden" {
		t.Fatalf("sessionProjects[dev] = %q, want garden", got.sessionProjects["dev"])
	}
	if got.selectedProject != "garden" {
		t.Fatalf("selectedProject = %q, want garden", got.selectedProject)
	}
	if synced["dev"] != "garden" {
		t.Fatalf("synced project = %#v, want dev->garden", synced)
	}
}

func TestRenameSessionCallsTmuxAndUpdatesSelection(t *testing.T) {
	tmp := t.TempDir()
	var renamed []string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = []string{oldName, newName}
			return nil
		},
	}, "dev", "").(model)
	m.statePath = tmp + "/state.json"
	m.sessions = []session{{Name: "dev"}}
	m.projects = []string{defaultProjectName}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.mode = inputRename
	m.renameRow = treeRow{kind: rowSession, project: defaultProjectName, session: "dev"}
	m.sessionTypes = map[string]sessionType{"dev": sessionTypeAgent}
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
	if got.selectedSession != "dev" {
		t.Fatalf("selectedSession before ack = %q, want dev", got.selectedSession)
	}
	updated, followUp := got.Update(msg)
	final := updated.(model)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	if final.selectedSession != "lala" {
		t.Fatalf("selectedSession = %q, want lala", got.selectedSession)
	}
	if final.currentSession != "lala" {
		t.Fatalf("currentSession = %q, want lala", final.currentSession)
	}
	if final.sessionProjects["lala"] != defaultProjectName {
		t.Fatalf("sessionProjects[lala] = %q, want %q", final.sessionProjects["lala"], defaultProjectName)
	}
	if final.sessionTypes["lala"] != sessionTypeAgent {
		t.Fatalf("sessionTypes[lala] = %q, want %q", final.sessionTypes["lala"], sessionTypeAgent)
	}
	if _, ok := final.sessionProjects["dev"]; ok {
		t.Fatalf("old session project still present: %#v", final.sessionProjects)
	}
	if fmt.Sprint(renamed) != fmt.Sprint([]string{"dev", "lala"}) {
		t.Fatalf("renameSession calls = %#v", renamed)
	}
}

func TestRenderTreePanelShowsProjectsAndSessionsOnly(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{
		"dev": defaultProjectName,
		"api": "small",
	}
	m.sessionTypes = map[string]sessionType{
		"dev": sessionTypeAgent,
		"api": sessionTypeK9s,
	}
	m.expandedProjects = map[string]bool{
		defaultProjectName: true,
		"small":            true,
	}
	m.currentSession = "dev"
	m.selectedProject = defaultProjectName

	view := m.renderTreePanel(40)
	plain := stripANSI(view)
	for _, want := range []string{"Projects", "[-] default", "[-] small", "[live]  [agent]  dev", "[k9s]  api"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderTreePanel missing %q in %q", want, plain)
		}
	}
	if !strings.Contains(plain, "[live]") {
		t.Fatalf("renderTreePanel missing inline live badge in %q", plain)
	}
	for _, unwanted := range []string{"current dev", "2 projects", "2 sessions", "[open]", "[shut]"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderTreePanel unexpectedly contained %q in %q", unwanted, plain)
		}
	}
}

func stripANSI(s string) string {
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "")
}

func TestSessionsLoadedSyncsTmuxSessionProjects(t *testing.T) {
	var got map[string]string
	m := newModel(fakeTmuxController{
		syncSessionProjects: func(sessionProjects map[string]string) error {
			got = cloneStringMap(sessionProjects)
			return nil
		},
	}, "dev", "%3").(model)
	m.projects = []string{defaultProjectName, "small"}
	m.sessionProjects = map[string]string{
		"dev": "small",
		"api": defaultProjectName,
	}

	updated, cmd := m.Update(sessionsLoadedMsg{
		sessions: []session{{Name: "dev"}, {Name: "api"}},
	})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	_ = updated.(model)

	want := map[string]string{
		"dev": "small",
		"api": defaultProjectName,
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("syncSessionProjects = %#v, want %#v", got, want)
	}
}

func TestSaveStatePersistsSessionTypes(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.sessionTypes = map[string]sessionType{"dev": sessionTypeAgent}
	m.projectConfigs = map[string]projectConfig{defaultProjectName: {Name: defaultProjectName}}
	m.expandedProjects = map[string]bool{defaultProjectName: true}

	if err := m.saveState(); err != nil {
		t.Fatalf("saveState returned error: %v", err)
	}

	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if got := state.SessionTypes["dev"]; got != string(sessionTypeAgent) {
		t.Fatalf("sessionTypes[dev] = %q, want %q", got, sessionTypeAgent)
	}
}

func TestSessionsLoadedSkipsTemporarySessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "otter-temp", "").(model)
	m.statePath = t.TempDir() + "/state.json"
	m.projects = []string{defaultProjectName}
	m.sessionProjects = map[string]string{}
	m.expandedProjects = map[string]bool{defaultProjectName: true}

	updated, cmd := m.Update(sessionsLoadedMsg{
		sessions: []session{{Name: "otter-temp", Temporary: true}},
	})
	if cmd != nil {
		t.Fatal("expected no command")
	}
	got := updated.(model)
	if len(got.sessionProjects) != 0 {
		t.Fatalf("sessionProjects = %#v, want empty", got.sessionProjects)
	}
	if got.selectedSession != "" {
		t.Fatalf("selectedSession = %q, want empty", got.selectedSession)
	}
}

func TestProjectAccentColorIsStable(t *testing.T) {
	if got, want := projectAccentColor("small"), projectAccentColor("small"); got != want {
		t.Fatalf("projectAccentColor not stable: %q != %q", got, want)
	}
	if got := projectAccentColor("small"); got == "" {
		t.Fatal("expected non-empty accent color")
	}
}

func TestMoveProjectUsesIncrementalPrefix(t *testing.T) {
	m := NewMenu().(model)
	m.sessions = []session{{Name: "dev"}}
	m.projects = []string{defaultProjectName, "small", "storage"}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.projectConfigs = map[string]projectConfig{
		defaultProjectName: {Name: defaultProjectName},
		"small":            {Name: "small"},
		"storage":          {Name: "storage"},
	}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true, "storage": true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.mode = inputMoveProject
	m.moveQuery = "sm"

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected move command")
	}
	msg := cmd().(sessionMovedMsg)
	if msg.project != "small" || msg.session != "dev" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestDeleteProjectDeletesSessions(t *testing.T) {
	tmp := t.TempDir()
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}, {Name: "keep"}}
	m.sessionProjects = map[string]string{
		"dev":  "small",
		"api":  "small",
		"keep": "storage",
	}
	m.sessionTypes = map[string]sessionType{
		"dev":  sessionTypeAgent,
		"api":  sessionTypeK9s,
		"keep": sessionTypeTerminal,
	}
	m.expandedProjects = map[string]bool{"small": true, "storage": true}
	m.selectedProject = "small"

	updated, cmd := m.deleteSelectedProject()
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
	got := updated.(model)
	if followUp != nil {
		t.Fatal("expected no follow-up command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("projects still contain deleted project: %#v", got.projects)
	}
	if _, ok := got.sessionProjects["dev"]; ok {
		t.Fatalf("sessionProjects still contains deleted session: %#v", got.sessionProjects)
	}
	if _, ok := got.sessionProjects["api"]; ok {
		t.Fatalf("sessionProjects still contains deleted session: %#v", got.sessionProjects)
	}
	if !containsString(got.projects, "storage") {
		t.Fatalf("remaining project missing: %#v", got.projects)
	}
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
	if len(got.sessions) != 1 || got.sessions[0].Name != "keep" {
		t.Fatalf("sessions = %#v, want only keep", got.sessions)
	}
}

func TestDeleteProjectHandlesProjectWithoutSessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.statePath = t.TempDir() + "/state.json"
	m.projects = []string{"small"}
	m.expandedProjects = map[string]bool{"small": true}
	m.selectedProject = "small"

	updated, cmd := m.deleteSelectedProject()
	pending := updated.(model)
	if cmd == nil {
		t.Fatal("expected delete command")
	}
	msg := cmd().(projectDeletedMsg)
	if msg.err != nil {
		t.Fatalf("delete returned error: %v", msg.err)
	}
	updated, followUp := pending.Update(msg)
	got := updated.(model)
	if followUp != nil {
		t.Fatal("expected no follow-up command")
	}
	if containsString(got.projects, "small") {
		t.Fatalf("deleted project still present: %#v", got.projects)
	}
	if got.selectedProject != "" {
		t.Fatalf("selectedProject = %q, want empty", got.selectedProject)
	}
}

func TestProtectedProjectRejectsDelete(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.projects = []string{defaultProjectName, "small"}
	m.projectConfigs = map[string]projectConfig{
		defaultProjectName: {Name: defaultProjectName},
		"small":            {Name: "small", Protect: true},
	}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"

	updated, cmd := m.beginDelete()
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if got.status != "Project small is protected." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestProtectedProjectStillAllowsSessionDelete(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "", "").(model)
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
	m.projectConfigs = map[string]projectConfig{
		defaultProjectName: {Name: defaultProjectName},
		"small":            {Name: "small", Protect: true},
	}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"
	m.selectedSession = "dev"

	updated, cmd := m.beginDelete()
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.mode != inputConfirmDelete {
		t.Fatalf("mode = %v, want inputConfirmDelete", got.mode)
	}

	_, cmd = got.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
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
