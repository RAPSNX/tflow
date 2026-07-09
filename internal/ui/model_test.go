package ui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const defaultProjectName = "default"

func TestMenuStartsWithCurrentSessionSelected(t *testing.T) {
	m := NewMenu().(model)
	m.currentSession = "dev"
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.syncSelection()

	if m.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", m.selectedSession)
	}
}

func TestPrepareStartupCreatesSessionBeforeControlMode(t *testing.T) {
	tmp := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", tmp)

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
	m.expandedProjects = map[string]bool{defaultProjectName: true}
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
	m.expandedProjects = map[string]bool{defaultProjectName: true}
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

func TestNewModelStartsWithoutProjectsWhenConfigIsEmpty(t *testing.T) {
	configHome := t.TempDir()
	configDir := configHome + "/tflow"
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(configDir+"/config.yaml", []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
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

func TestNewPrefixAAttachesCurrentTempSessionToProject(t *testing.T) {
	var tempChanges []string
	m := newModel(fakeTmuxController{
		setSessionTemporary: func(name string, temporary bool) error {
			tempChanges = append(tempChanges, fmt.Sprintf("%s:%t", name, temporary))
			return nil
		},
	}, "otter-temp", "").(model)
	m.mode = inputNew
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "otter-temp", Temporary: true}}
	m.selectedProject = "small"

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got := updated.(model)
	if got.mode != inputNew {
		t.Fatalf("mode = %v, want inputNew before ack", got.mode)
	}
	if cmd == nil {
		t.Fatal("expected attach command")
	}
	msg := cmd().(sessionMovedMsg)
	if msg.err != nil {
		t.Fatalf("attach returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(tempChanges), fmt.Sprint([]string{"otter-temp:false"}); got != want {
		t.Fatalf("tempChanges = %s, want %s", got, want)
	}

	updated, followUp := got.Update(msg)
	final := updated.(model)
	if final.sessionProjects["otter-temp"] != "small" {
		t.Fatalf("sessionProjects[otter-temp] = %q", final.sessionProjects["otter-temp"])
	}
	if final.selectedSession != "otter-temp" {
		t.Fatalf("selectedSession = %q", final.selectedSession)
	}
	if followUp != nil {
		if reload := followUp(); reload == nil {
			t.Fatal("expected reload message from follow-up command")
		}
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

func TestDefaultSessionDirPrefersHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	_ = os.Setenv("HOME", "/tmp/home")

	if got := defaultSessionDir(); got != "/tmp/home" {
		t.Fatalf("defaultSessionDir = %q, want /tmp/home", got)
	}
}

func TestCreateSessionUsesCurrentDirectory(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			if command != "" {
				t.Fatalf("command = %q, want empty", command)
			}
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != m.cwd {
		t.Fatalf("cwd = %q, want %q", gotCWD, m.cwd)
	}
}

func TestCreateSessionUsesProjectDirectoryWhenConfigured(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/project-small"}}
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != "/tmp/project-small" {
		t.Fatalf("cwd = %q, want /tmp/project-small", gotCWD)
	}
}

func TestCreateSessionUsesExpandedHomeDirectoryWhenConfigured(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "~/project-small"}}
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != "/tmp/home/project-small" {
		t.Fatalf("cwd = %q, want /tmp/home/project-small", gotCWD)
	}
}

func TestCreateK9sSessionUsesClusterPath(t *testing.T) {
	var gotCWD string
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Workdir: "/tmp/project-small", Cluster: clusterConfig{Path: "/tmp/kubeconfig"}},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindK9s
	m.input.SetValue("k9s")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "export KUBECONFIG='/tmp/kubeconfig'; exec k9s" {
		t.Fatalf("command = %q", gotCommand)
	}
	if gotCWD != "/tmp/project-small" {
		t.Fatalf("cwd = %q, want /tmp/project-small", gotCWD)
	}
}

func TestCreateK9sSessionUsesConnectionCommand(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Cluster: clusterConfig{ConnectionCmd: "aws eks update-kubeconfig --name prod"}},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindK9s
	m.input.SetValue("k9s")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "aws eks update-kubeconfig --name prod && exec k9s" {
		t.Fatalf("command = %q", gotCommand)
	}
}

func TestCreateAgentSessionUsesConfiguredAgentBinary(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", AgentBinary: "aider"},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindAgent
	m.input.SetValue("agent")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "exec 'aider'" {
		t.Fatalf("command = %q", gotCommand)
	}
	if msg.kind != sessionTypeAgent {
		t.Fatalf("kind = %q, want %q", msg.kind, sessionTypeAgent)
	}
}

func TestCreateAgentSessionDefaultsToCodexBinary(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindAgent
	m.input.SetValue("agent")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "exec 'codex'" {
		t.Fatalf("command = %q", gotCommand)
	}
}

func TestProjectEditorCommandSplitsEditorArgs(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")

	cmd, err := projectEditorCommand("/tmp/project.yaml")
	if err != nil {
		t.Fatalf("projectEditorCommand returned error: %v", err)
	}
	if got, want := fmt.Sprint(cmd.Args), fmt.Sprint([]string{"code", "--wait", "/tmp/project.yaml"}); got != want {
		t.Fatalf("args = %s, want %s", got, want)
	}
}

func TestSetProjectDirectoryPersistsValue(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}
	m.selectedProject = "small"
	m.mode = inputSetProjectDir
	m.input.SetValue("/tmp/small-project")

	updated, cmd := m.commitProjectDir()
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.projectConfig("small").Workdir != "/tmp/small-project" {
		t.Fatalf("workdir = %q, want /tmp/small-project", got.projectConfig("small").Workdir)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "prod-main-01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestProjectNormalizationSortsAndDeduplicates(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"alpha", "default", "small"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}

type fakeTmuxController struct {
	listSessions        func() ([]session, error)
	createSession       func(name, cwd, command string) (session, error)
	setSessionTemporary func(name string, temporary bool) error
	attachCommand       func(name string) (*exec.Cmd, error)
	killSession         func(name string) error
	renameSession       func(oldName, newName string) error
	switchClient        func(name string) error
	ensureControlMode   func(binaryPath string) error
	toggleMenu          func(binaryPath string) error
	closePane           func(paneID string) error
	quitAll             func(paneID string) error
	syncSessionProjects func(sessionProjects map[string]string) error
}

func (f fakeTmuxController) ListSessions() ([]session, error) {
	if f.listSessions != nil {
		return f.listSessions()
	}
	return nil, nil
}

func (f fakeTmuxController) CreateSession(name, cwd, command string) (session, error) {
	if f.createSession != nil {
		return f.createSession(name, cwd, command)
	}
	return session{Name: name, Windows: 1}, nil
}

func (f fakeTmuxController) SetSessionTemporary(name string, temporary bool) error {
	if f.setSessionTemporary != nil {
		return f.setSessionTemporary(name, temporary)
	}
	return nil
}

func (f fakeTmuxController) AttachCommand(name string) (*exec.Cmd, error) {
	if f.attachCommand != nil {
		return f.attachCommand(name)
	}
	return exec.Command("sh", "-lc", ":"), nil
}

func (f fakeTmuxController) KillSession(name string) error {
	if f.killSession != nil {
		return f.killSession(name)
	}
	return nil
}

func (f fakeTmuxController) RenameSession(oldName, newName string) error {
	if f.renameSession != nil {
		return f.renameSession(oldName, newName)
	}
	return nil
}

func (f fakeTmuxController) SwitchClient(name string) error {
	if f.switchClient != nil {
		return f.switchClient(name)
	}
	return nil
}

func (f fakeTmuxController) EnsureControlMode(binaryPath string) error {
	if f.ensureControlMode != nil {
		return f.ensureControlMode(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) ToggleMenu(binaryPath string) error {
	if f.toggleMenu != nil {
		return f.toggleMenu(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) ClosePane(paneID string) error {
	if f.closePane != nil {
		return f.closePane(paneID)
	}
	return nil
}

func (f fakeTmuxController) QuitAll(paneID string) error {
	if f.quitAll != nil {
		return f.quitAll(paneID)
	}
	return nil
}

func (f fakeTmuxController) SyncSessionProjects(sessionProjects map[string]string) error {
	if f.syncSessionProjects != nil {
		return f.syncSessionProjects(sessionProjects)
	}
	return nil
}

func cloneStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
