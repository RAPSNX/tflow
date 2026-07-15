package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRStartsProjectRenameModeWithoutSelectedSession(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.selectedProject = "small"

	updated, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
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
	}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
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
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.mode = inputRename
	m.renameTarget = renameTarget{session: "dev"}
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
		t.Fatalf("selectedSession = %q, want lala", final.selectedSession)
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

func TestRenderSessionPanelShowsFlatSessionsOnly(t *testing.T) {
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
	m.currentSession = "dev"
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	view := m.renderSessionPanel(40)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	for _, want := range []string{"Sessions", "[live]  [agent]  dev"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderSessionPanel missing %q in %q", want, plain)
		}
	}
	for _, unwanted := range []string{"Projects", "small", "[-]"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderSessionPanel unexpectedly contained %q in %q", unwanted, plain)
		}
	}
}

func TestRenderSessionPanelShowsVisibleProjectRowWhenProjectSelected(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = ""

	view := m.renderSessionPanel(40)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	for _, want := range []string{"Sessions", "project default", "dev"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderSessionPanel missing %q in %q", want, plain)
		}
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
	if got.selectedProject != "storage" {
		t.Fatalf("selectedProject = %q, want storage", got.selectedProject)
	}
}

func TestEditProjectRequiresProjectContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "", "").(model)

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
