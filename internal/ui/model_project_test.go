package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRStartsProjectRenameModeWithSelectedSession(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.selectedProject = "small"
	m.sessions = []session{{Name: "small--code"}}
	m.selectedSession = "small--code"
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}

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
	m.sessions = []session{{Name: "small--dev"}}
	m.sessionProjects = map[string]string{"small--dev": "small"}
	m.sessionLabels = map[string]string{"small--dev": "dev"}
	m.selectedProject = "small"

	updated, cmd := m.Update(projectRenamedMsg{
		oldName: "small",
		newName: "garden",
		sessionRenames: []sessionRename{{
			oldName: "small--dev",
			newName: "garden--dev",
		}},
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
	if got.sessionProjects["garden--dev"] != "garden" {
		t.Fatalf("sessionProjects[garden--dev] = %q, want garden", got.sessionProjects["garden--dev"])
	}
	if got.selectedProject != "garden" {
		t.Fatalf("selectedProject = %q, want garden", got.selectedProject)
	}
	if synced["garden--dev"] != "garden" {
		t.Fatalf("synced project = %#v, want garden--dev->garden", synced)
	}
}

func TestRRenamesProjectWithSelectedSession(t *testing.T) {
	tmp := t.TempDir()
	var renamed []string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, oldName, newName)
			return nil
		},
	}, "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.selectedProject = "small"
	m.selectedSession = "small--code"

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
	if fmt.Sprint(renamed) != fmt.Sprint([]string{"small--code", "garden--code"}) {
		t.Fatalf("tmux renames = %#v", renamed)
	}
	pending, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatal("expected menu model after project rename command")
	}
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.selectedSession != "garden--code" || got.sessionLabel("garden--code") != "code" {
		t.Fatalf("renamed selection = %q, label = %q", got.selectedSession, got.sessionLabel("garden--code"))
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
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.selectedProject = "small"
	m.selectedSession = "small--code"

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

func TestSessionLoadMigratesProjectSessionsToScopedNames(t *testing.T) {
	tmp := t.TempDir()
	var renamed []string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, oldName, newName)
			return nil
		},
	}, "code").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{"small"}
	m.sessionProjects = map[string]string{"code": "small"}

	updated, cmd := m.Update(sessionsLoadedMsg{sessions: []session{{Name: "code"}}})
	pending := updated.(model)
	if cmd == nil {
		t.Fatal("expected scoped-name migration command")
	}
	msg := cmd().(sessionNamesMigratedMsg)
	if msg.err != nil {
		t.Fatalf("migration returned error: %v", msg.err)
	}
	if got, want := fmt.Sprint(renamed), fmt.Sprint([]string{"code", "small--code"}); got != want {
		t.Fatalf("renames = %s, want %s", got, want)
	}
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.currentSession != "small--code" || got.sessionLabel("small--code") != "code" {
		t.Fatalf("migrated current = %q, labels = %#v", got.currentSession, got.sessionLabels)
	}
}

func TestRenameSessionCallsTmuxAndUpdatesSelection(t *testing.T) {
	tmp := t.TempDir()
	var renamed []string
	var syncedProjects map[string]string
	m := newModel(fakeTmuxController{
		syncSessionProjects: func(projects map[string]string) error {
			syncedProjects = cloneStringMap(projects)
			return nil
		},
		renameSession: func(oldName, newName string) error {
			renamed = []string{oldName, newName}
			return nil
		},
	}, "default--dev").(model)
	m.statePath = tmp + "/state.json"
	m.sessions = []session{{Name: "default--dev"}}
	m.projects = []string{defaultProjectName}
	m.sessionProjects = map[string]string{"default--dev": defaultProjectName}
	m.sessionLabels = map[string]string{"default--dev": "dev"}
	m.selectedProject = defaultProjectName
	m.selectedSession = "default--dev"
	m.mode = inputRename
	m.renameTarget = renameTarget{session: "default--dev"}
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
	if got.selectedSession != "default--dev" {
		t.Fatalf("selectedSession before ack = %q, want default--dev", got.selectedSession)
	}
	updated, followUp := got.Update(msg)
	final := updated.(model)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	if final.selectedSession != "default--lala" {
		t.Fatalf("selectedSession = %q, want default--lala", final.selectedSession)
	}
	if final.currentSession != "default--lala" {
		t.Fatalf("currentSession = %q, want default--lala", final.currentSession)
	}
	if final.sessionProjects["default--lala"] != defaultProjectName {
		t.Fatalf("sessionProjects[default--lala] = %q, want %q", final.sessionProjects["default--lala"], defaultProjectName)
	}
	if final.sessionLabel("default--lala") != "lala" {
		t.Fatalf("session label = %q, want lala", final.sessionLabel("default--lala"))
	}
	if _, found := final.findSession("default--lala"); !found {
		t.Fatalf("renamed session missing from sessions: %#v", final.sessions)
	}
	if _, found := final.findSession("default--dev"); found {
		t.Fatalf("old session remains in sessions: %#v", final.sessions)
	}
	if got := syncedProjects["default--lala"]; got != defaultProjectName {
		t.Fatalf("synced project = %q, want %q", got, defaultProjectName)
	}
	if _, found := syncedProjects["default--dev"]; found {
		t.Fatalf("old session was synced: %#v", syncedProjects)
	}
	if _, ok := final.sessionProjects["default--dev"]; ok {
		t.Fatalf("old session project still present: %#v", final.sessionProjects)
	}
	if fmt.Sprint(renamed) != fmt.Sprint([]string{"default--dev", "default--lala"}) {
		t.Fatalf("renameSession calls = %#v", renamed)
	}
}

func TestRenderHeaderCentersBrandWithoutPopupMetadata(t *testing.T) {
	m := NewMenu().(model)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHeader(40), "")
	firstLine := strings.Split(plain, "\n")[0]
	if got, want := strings.Index(firstLine, "TFLOW"), 17; got != want {
		t.Fatalf("TFLOW offset = %d, want %d in %q", got, want, firstLine)
	}
	for _, unwanted := range []string{"project", "session"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("header unexpectedly contains %q in %q", unwanted, plain)
		}
	}
}

func TestRenderSessionRowUsesDisplayLabel(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.selectedSession = "small--code"
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionRow(0, m.sessions[0]), "")
	if !strings.Contains(plain, "code") || strings.Contains(plain, "small--code") {
		t.Fatalf("session row = %q", plain)
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
	m.currentSession = "dev"
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	view := m.renderSessionPanel(40)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	for _, want := range []string{"Sessions", "live", "dev"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderSessionPanel missing %q in %q", want, plain)
		}
	}
	for _, unwanted := range []string{"Projects", "small", "[-]", "[agent]", "[k9s]"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderSessionPanel unexpectedly contained %q in %q", unwanted, plain)
		}
	}
}

func TestRenderSessionPanelUsesCurrentProjectContext(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{
		"dev": defaultProjectName,
		"api": "small",
	}
	m.selectedProject = "small"
	m.selectedSession = "api"

	view := m.renderSessionPanel(40)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	if !strings.Contains(plain, "api") {
		t.Fatalf("renderSessionPanel missing selected-project session in %q", plain)
	}
	if strings.Contains(plain, "dev") {
		t.Fatalf("renderSessionPanel leaked other-context session in %q", plain)
	}
	if strings.Contains(plain, "Projects") {
		t.Fatalf("renderSessionPanel unexpectedly contained grouped project UI in %q", plain)
	}
}

func TestRenderMenuIncludesBrandSessionPanelAndStatusArea(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.height = 16
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.status = "Type a project prefix to switch."

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderMenu(), "")
	for _, want := range []string{"TFLOW", "Sessions", "Type a project prefix to switch."} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderMenu missing %q in %q", want, plain)
		}
	}
}

func TestRenderFooterShowsOnlyInlineStatusByDefault(t *testing.T) {
	m := NewMenu().(model)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderFooter(60), "")
	if strings.TrimSpace(plain) != "" {
		t.Fatalf("default footer = %q, want empty", plain)
	}
	m.status = "Saved."
	plain = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderFooter(60), "")
	if !strings.Contains(plain, "Saved.") {
		t.Fatalf("status footer = %q", plain)
	}
}

func TestRenderHelpListsOneShortcutPerRow(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.height = 24
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHelp(), "")
	for _, want := range []string{"Ctrl+F", "Ctrl+Q", "Ctrl+C", "Esc", "?", "j", "k", "Enter", "n", "N", "p", "r", "R", "d", "D", "e"} {
		count := 0
		for _, line := range strings.Split(plain, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), want) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("shortcut %q appears %d times in %q", want, count, plain)
		}
	}
}

func TestRenderHelpDoesNotApplyOuterLayout(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.height = 24
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHelp(), "")
	if got, want := strings.Count(plain, "\n")+1, 17; got != want {
		t.Fatalf("help row count = %d, want %d", got, want)
	}
}

func TestConfirmationOverlaysAdvertiseAcceptedKeys(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.height = 24

	for name, render := range map[string]func() string{
		"delete":         m.renderDeleteOverlay,
		"project switch": m.renderProjectSwitchConfirmOverlay,
		"quit":           m.renderQuitConfirmOverlay,
	} {
		t.Run(name, func(t *testing.T) {
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(render(), "")
			for _, keycap := range []string{"Enter", "Esc", "Cancel"} {
				if !strings.Contains(plain, keycap) {
					t.Fatalf("confirmation missing %q in %q", keycap, plain)
				}
			}
		})
	}
}

func TestRenderProjectSwitchDialogListsMatchingProjects(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{"small", "storage"}
	m.mode = inputSwitchProject
	m.input.Prompt = "project: "

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderProjectSwitchOverlay(), "")
	lines := strings.Split(plain, "\n")
	foundSmall := false
	foundStorage := false
	for _, line := range lines {
		if strings.Contains(line, "small") {
			foundSmall = true
		}
		if strings.Contains(line, "storage") {
			foundStorage = true
		}
	}
	if !foundSmall || !foundStorage {
		t.Fatalf("renderFooter missing newline-separated project list in %q", plain)
	}
}

func TestRenderBadgesUseFilledContrastingStyles(t *testing.T) {
	if brandBadgeStyle.GetBackground() != blueColor {
		t.Fatalf("brand badge background = %v, want %v", brandBadgeStyle.GetBackground(), blueColor)
	}
	if currentBadgeStyle.GetBackground() != tealColor {
		t.Fatalf("live badge background = %v, want %v", currentBadgeStyle.GetBackground(), tealColor)
	}
	if currentBadgeStyle.GetBackground() == selectedSessionStyle.GetBackground() {
		t.Fatal("live badge and selected row use the same background")
	}
	m := NewMenu().(model)
	m.width = 48
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": "small", "api": "small"}
	m.sessionLabels = map[string]string{"dev": "dev", "api": "api"}
	m.selectedProject = "small"
	m.currentSession = "dev"
	m.selectedSession = "dev"
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionPanel(40), "")
	if strings.Count(plain, "live") != 1 || !strings.Contains(plain, "live  dev") {
		t.Fatalf("active row = %q", plain)
	}
}

func TestDialogsUseSharedStructuredCardLayout(t *testing.T) {
	m := NewMenu().(model)
	m.width = 64
	m.height = 24
	m.input.Prompt = "name: "
	m.renameTarget = renameTarget{session: "small--otter"}
	m.deleteTarget = deleteTarget{session: "small--otter"}
	m.switchProjectTarget = "small"
	tests := []struct {
		name   string
		render func() string
		badge  string
		title  string
	}{
		{"create", func() string { return m.renderInputOverlay("New Session") }, "CREATE", "New Session"},
		{"settings", func() string { return m.renderInputOverlay("Project Settings") }, "SETTINGS", "Project Settings"},
		{"rename", m.renderRenameOverlay, "RENAME", "Rename Session"},
		{"switch", m.renderProjectSwitchOverlay, "SWITCH", "Switch Project"},
		{"delete", m.renderDeleteOverlay, "DELETE", "Confirm Delete"},
		{"confirm", m.renderProjectSwitchConfirmOverlay, "CONFIRM", "Confirm Project Switch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(test.render(), "")
			for _, want := range []string{test.badge, test.title, "──", "Enter", "Esc", "Cancel"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("dialog missing %q in %q", want, plain)
				}
			}
		})
	}
}

func TestDeleteDialogUsesDestructiveAccentsOnly(t *testing.T) {
	if destructiveBadgeStyle.GetBackground() != redColor || destructiveKeycapStyle.GetBackground() != redColor {
		t.Fatal("delete accents do not use red")
	}
	if dialogHeaderBadgeStyle.GetBackground() == redColor || keycapStyle.GetBackground() == redColor {
		t.Fatal("normal dialog accents use red")
	}
}

func TestDialogKeepsStatusVisibleInFooter(t *testing.T) {
	m := NewMenu().(model)
	m.width = 64
	m.height = 20
	m.status = "Saved project settings."
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderInputOverlay("Project Settings"), "")
	if !strings.Contains(plain, "Saved project settings.") {
		t.Fatalf("dialog status missing from %q", plain)
	}
	if strings.LastIndex(plain, "Saved project settings.") < strings.LastIndex(plain, "Cancel") {
		t.Fatalf("status was not rendered below the dialog: %q", plain)
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
	if got.input.Prompt != "workdir: " {
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
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if got := state.ProjectConfigs["small"].Workdir; got != "/tmp/small" {
		t.Fatalf("saved workdir = %q", got)
	}
}
