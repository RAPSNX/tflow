package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPrepareStartupBootstrapsProjectAndCodeSession(t *testing.T) {
	configHome := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	var created []string
	var synced map[string]sessionMetadata
	manager := fakeTmuxController{
		ensureControlMode: func(binaryPath string) error { return nil },
		createSession: func(name, cwd, command string) (session, error) {
			created = append(created, fmt.Sprintf("%s|%s|%s", name, cwd, command))
			return session{Name: name}, nil
		},
		syncSessionMetadata: func(metadata map[string]sessionMetadata) error {
			synced = metadata
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if name == "" {
		t.Fatal("expected startup session name")
	}
	if len(created) != 1 {
		t.Fatalf("created = %#v, want one session", created)
	}
	if len(synced) != 1 {
		t.Fatalf("synced = %#v, want one metadata entry", synced)
	}

	state, err := loadAppState(filepath.Join(configHome, "tflow", "state.json"))
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if len(state.Projects) != 1 {
		t.Fatalf("projects = %#v, want one", state.Projects)
	}
	if len(state.Projects[0].Sessions) != 1 || state.Projects[0].Sessions[0].Name != defaultSessionName {
		t.Fatalf("unexpected bootstrap state: %#v", state)
	}
}

func TestBuildHintsUsesShortestUniquePrefixes(t *testing.T) {
	hints := buildHints([]string{"tiger", "table", "mouse"})
	if hints["tiger"] != "ti" || hints["table"] != "ta" || hints["mouse"] != "m" {
		t.Fatalf("unexpected hints: %#v", hints)
	}
}

func TestProjectOverlayTargetsPersistentProjectsOnly(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{
			{Name: "garden", Persistent: false},
			{Name: "alpha", Persistent: true},
			{Name: "mouse", Persistent: false},
		},
	}

	m.openProjectOverlay(overlaySwitchProject)
	if len(m.overlay.Targets) != 1 || m.overlay.Targets[0].Project != "alpha" {
		t.Fatalf("targets = %#v, want alpha only", m.overlay.Targets)
	}

	updated, _ := m.updateOverlay(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got := updated.(model)
	if got.mode != inputProjectHints {
		t.Fatalf("mode = %v, want inputProjectHints", got.mode)
	}
}

func TestMoveSelectedSessionRenamesTmuxAndUpdatesState(t *testing.T) {
	var renames [][2]string
	m := newModel(fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renames = append(renames, [2]string{oldName, newName})
			return nil
		},
	}, "garden_code", "").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{
			{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code", Type: sessionTypeTerminal}}},
			{Name: "mouse", Persistent: true},
		},
	}
	m.selectedSession = "code"

	_, cmd := m.moveSelectedSession("mouse")
	msg := cmd().(sessionMovedMsg)
	if msg.err != nil {
		t.Fatalf("move returned error: %v", msg.err)
	}
	m.applyMoveSelectedSession("mouse")
	if got := fmt.Sprint(renames); got != fmt.Sprint([][2]string{{"garden_code", "mouse_code"}}) {
		t.Fatalf("renames = %s", got)
	}
	if m.currentProjectName() != "garden" {
		t.Fatalf("currentProject = %q, want garden", m.currentProjectName())
	}
	if session := m.findSession("mouse", "code"); session == nil || session.TmuxName != "mouse_code" {
		t.Fatalf("moved session not found: %#v", m.state)
	}
}

func TestCleanupOwnedVolatileProjectKillsOnlyVolatileSessions(t *testing.T) {
	configHome := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	statePath := filepath.Join(configHome, "tflow", "state.json")
	state := appState{Projects: []projectState{
		{Name: "otter", Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}},
		{Name: "work", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "work_code"}}},
	}}
	if err := saveAppState(statePath, state, map[string]projectConfig{"work": {Name: "work"}}); err != nil {
		t.Fatalf("saveAppState returned error: %v", err)
	}

	var killed []string
	manager := fakeTmuxController{
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
		sessionCWD: func(name string) (string, error) {
			if name == "work_code" {
				return "/tmp/work-current", nil
			}
			return "", fmt.Errorf("can't find session: %s", name)
		},
	}
	if err := cleanupOwnedVolatileProject(manager, "otter_code"); err != nil {
		t.Fatalf("cleanupOwnedVolatileProject returned error: %v", err)
	}
	if got := fmt.Sprint(killed); got != fmt.Sprint([]string{"otter_code"}) {
		t.Fatalf("killed = %s", got)
	}
	state, err := loadAppState(statePath)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	work := projectForTmuxSession(state, "work_code")
	if work == nil || projectForTmuxSession(state, "otter_code") != nil {
		t.Fatalf("unexpected state after cleanup: %#v", state)
	}
	if got := work.Sessions[0].CWD; got != "/tmp/work-current" {
		t.Fatalf("persistent cwd = %q, want /tmp/work-current", got)
	}
}

func TestNewModelTracksPersistentCWDOnSidebarOpen(t *testing.T) {
	configHome := t.TempDir()
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	statePath := filepath.Join(configHome, "tflow", "state.json")
	cfgs := map[string]projectConfig{"work": {Name: "work", Workdir: "/tmp/work", AgentBinary: "codex"}}
	state := appState{
		CurrentProject: "work",
		Projects: []projectState{{
			Name:       "work",
			Persistent: true,
			Sessions:   []sessionState{{Name: "code", TmuxName: "work_code", Type: sessionTypeTerminal, CWD: "/tmp/old"}},
		}},
	}
	if err := saveAppState(statePath, state, cfgs); err != nil {
		t.Fatalf("saveAppState returned error: %v", err)
	}

	_ = newModel(fakeTmuxController{sessionCWD: func(name string) (string, error) {
		if name != "work_code" {
			t.Fatalf("SessionCWD called for %q, want work_code", name)
		}
		return "/tmp/current", nil
	}}, "work_code", "")

	state, err := loadAppState(statePath)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if got := state.Projects[0].Sessions[0].CWD; got != "/tmp/current" {
		t.Fatalf("cwd = %q, want /tmp/current", got)
	}
}

func TestStateFromPersistentConfigRestoresSessionsAndIgnoresVolatileProjects(t *testing.T) {
	previous := appState{Projects: []projectState{
		{Name: "work", Persistent: true, Sessions: []sessionState{{Name: "agent", TmuxName: "work_agent", Type: sessionTypeAgent, CWD: "/tmp/service"}}},
		{Name: "otter", Persistent: false, Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}},
		{Name: "removed", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "removed_code"}}},
	}}
	cfgs := map[string]projectConfig{
		"work": {Name: "work", Workdir: "/tmp/work", AgentBinary: "codex"},
		"new":  {Name: "new", Workdir: "/tmp/new", AgentBinary: "codex"},
	}

	state := stateFromPersistentConfig(previous, cfgs, "/tmp/fallback")
	if projectForTmuxSession(state, "otter_code") != nil || projectForTmuxSession(state, "removed_code") != nil {
		t.Fatalf("unexpected volatile or removed project restored: %#v", state)
	}
	if session := findProjectState(state, "work").Sessions[0]; session.Name != "agent" || session.CWD != "/tmp/service" {
		t.Fatalf("persistent session not restored: %#v", state)
	}
	if session := findProjectState(state, "new").Sessions[0]; session.Name != defaultSessionName || session.CWD != "/tmp/new" {
		t.Fatalf("default code session not created for new persistent project: %#v", state)
	}
}

func TestHelpHiddenUntilQuestionMark(t *testing.T) {
	m := newModel(fakeTmuxController{}, "otter_code", "").(model)
	m.width = 48
	m.height = 24
	m.state = appState{CurrentProject: "otter", Projects: []projectState{{Name: "otter", Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}}}}
	m.syncSelection()

	if strings.Contains(m.View(), "t ▶️ new terminal") {
		t.Fatal("help action visible before ?")
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(model)
	if !strings.Contains(m.View(), "t ▶️ new terminal") || !strings.Contains(m.View(), "P ▶️ persist project") {
		t.Fatalf("help not rendered after ?: %q", m.View())
	}
}

func TestCreateSessionInputRendersInlineInSidebar(t *testing.T) {
	m := newModel(fakeTmuxController{}, "otter_code", "").(model)
	m.width = 48
	m.height = 24
	m.state = appState{CurrentProject: "otter", Projects: []projectState{{Name: "otter", Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}}}}
	m.syncSelection()

	updated, _ := m.startSessionCreate(sessionTypeTerminal)
	m = *updated.(*model)
	view := m.View()
	if !strings.Contains(view, "Sessions") || !strings.Contains(view, "New Terminal Session") || !strings.Contains(view, "Enter saves. Esc cancels.") {
		t.Fatalf("create session input not rendered inline: %q", view)
	}
}

func TestRKeyDoesNotStartProjectRename(t *testing.T) {
	m := newModel(fakeTmuxController{}, "otter_code", "").(model)
	m.state = appState{CurrentProject: "otter", Projects: []projectState{{Name: "otter", Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}}}}
	m.syncSelection()

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	got := updated.(model)
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
}

func TestSwitchProjectClosesPaneForNonEmptyTarget(t *testing.T) {
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
	}, "garden_code", "%3").(model)
	m.state = appState{
		CurrentProject: "garden",
		Projects: []projectState{
			{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
			{Name: "mouse", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "mouse_code"}}},
		},
	}

	_, cmd := m.switchProject("mouse")
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("switch returned error: %v", msg.err)
	}
	if got := fmt.Sprint(switched); got != fmt.Sprint([]string{"mouse_code"}) {
		t.Fatalf("switched = %s", got)
	}
	if got := fmt.Sprint(closed); got != fmt.Sprint([]string{"%3"}) {
		t.Fatalf("closed = %s", got)
	}
}

func TestCreatedSessionSwitchesClientAndClosesSidebar(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	var switched []string
	var closed []string
	m := model{
		tmux: fakeTmuxController{
			switchClient: func(name string) error {
				switched = append(switched, name)
				return nil
			},
			closePane: func(paneID string) error {
				closed = append(closed, paneID)
				return nil
			},
		},
		paneID:             "%7",
		statePath:          statePath,
		projectCfg:         map[string]projectConfig{},
		currentTmuxSession: "garden_code",
		state: appState{CurrentProject: "garden", Projects: []projectState{{
			Name:     "garden",
			Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}},
		}}},
	}

	updated, cmd := m.Update(sessionCreatedMsg{session: sessionState{Name: "shell", TmuxName: "garden_shell", Type: sessionTypeTerminal}})
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("switch command returned error: %v", msg.err)
	}
	got := updated.(model)
	if got.currentTmuxSession != "garden_shell" || got.selectedSession != "shell" || got.selection != selectionSessions {
		t.Fatalf("unexpected model after create: %#v", got)
	}
	if fmt.Sprint(switched) != fmt.Sprint([]string{"garden_shell"}) || fmt.Sprint(closed) != fmt.Sprint([]string{"%7"}) {
		t.Fatalf("switched=%v closed=%v", switched, closed)
	}
}

func TestProjectRowsAreSelectableAndEnterSwitchesProject(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	var switched []string
	var closed []string
	m := model{
		tmux: fakeTmuxController{
			switchClient: func(name string) error {
				switched = append(switched, name)
				return nil
			},
			closePane: func(paneID string) error {
				closed = append(closed, paneID)
				return nil
			},
		},
		paneID:             "%8",
		statePath:          statePath,
		projectCfg:         map[string]projectConfig{"alpha": {Name: "alpha"}},
		currentTmuxSession: "garden_code",
		state: appState{CurrentProject: "garden", Projects: []projectState{
			{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
			{Name: "alpha", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "alpha_code"}}},
		}},
	}
	m.syncSelection()

	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(model)
	if m.selection != selectionProjects || m.selectedProject != "alpha" {
		t.Fatalf("selection = %v project = %q, want alpha project row", m.selection, m.selectedProject)
	}
	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("switch returned error: %v", msg.err)
	}
	if fmt.Sprint(switched) != fmt.Sprint([]string{"alpha_code"}) || fmt.Sprint(closed) != fmt.Sprint([]string{"%8"}) {
		t.Fatalf("switched=%v closed=%v", switched, closed)
	}
}

func TestSwitchProjectUsesGloballyTrackedCurrentSession(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	var switched []string
	m := model{
		tmux: fakeTmuxController{switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		}},
		statePath:  statePath,
		projectCfg: map[string]projectConfig{"alpha": {Name: "alpha", AgentBinary: "codex"}},
		state: appState{
			CurrentProject:  "garden",
			CurrentSessions: map[string]string{"alpha": "agent"},
			Projects: []projectState{
				{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
				{Name: "alpha", Persistent: true, Sessions: []sessionState{
					{Name: "code", TmuxName: "alpha_code"},
					{Name: "agent", TmuxName: "alpha_agent", Type: sessionTypeAgent},
				}},
			},
		},
	}

	_, cmd := m.switchProject("alpha")
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("switch returned error: %v", msg.err)
	}
	if fmt.Sprint(switched) != fmt.Sprint([]string{"alpha_agent"}) {
		t.Fatalf("switched = %v, want alpha_agent", switched)
	}
}

func TestCtrlCClosesSidebarFromModalState(t *testing.T) {
	var closed []string
	m := model{
		tmux: fakeTmuxController{closePane: func(paneID string) error {
			closed = append(closed, paneID)
			return nil
		}},
		paneID: "%9",
		mode:   inputCreateSession,
		state:  appState{CurrentProject: "garden", Projects: []projectState{{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}}}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if fmt.Sprint(closed) != fmt.Sprint([]string{"%9"}) {
		t.Fatalf("closed = %v, want %%9", closed)
	}
}

func TestCtrlQQuitsTflowFromNormalState(t *testing.T) {
	var quit []string
	m := model{
		tmux: fakeTmuxController{quitAll: func(paneID string) error {
			quit = append(quit, paneID)
			return nil
		}},
		paneID: "%10",
		state:  appState{CurrentProject: "garden", Projects: []projectState{{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}}}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("quit returned error: %v", msg.err)
	}
	if fmt.Sprint(quit) != fmt.Sprint([]string{"%10"}) {
		t.Fatalf("quit = %v, want %%10", quit)
	}
}

func TestCtrlQQuitsTflowFromModalState(t *testing.T) {
	var quit []string
	m := model{
		tmux: fakeTmuxController{quitAll: func(paneID string) error {
			quit = append(quit, paneID)
			return nil
		}},
		paneID: "%11",
		mode:   inputCreateSession,
		state:  appState{CurrentProject: "garden", Projects: []projectState{{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}}}},
	}

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("quit returned error: %v", msg.err)
	}
	if fmt.Sprint(quit) != fmt.Sprint([]string{"%11"}) {
		t.Fatalf("quit = %v, want %%11", quit)
	}
}

func TestTabSwitchesBetweenSidebarSections(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.state = appState{CurrentProject: "garden", Projects: []projectState{
		{Name: "garden", Sessions: []sessionState{
			{Name: "code", TmuxName: "garden_code"},
			{Name: "shell", TmuxName: "garden_shell"},
		}},
		{Name: "alpha", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "alpha_code"}}},
		{Name: "beta", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "beta_code"}}},
	}}
	m.syncSelection()

	m.shiftSelection(1)
	if m.selection != selectionSessions || m.selectedSession != "shell" {
		t.Fatalf("selection = %v session = %q, want shell session", m.selection, m.selectedSession)
	}
	updated, _ := m.updateNormal(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(model)
	if m.selection != selectionProjects || m.selectedProject != "alpha" {
		t.Fatalf("selection = %v project = %q, want alpha project", m.selection, m.selectedProject)
	}
	m.shiftSelection(1)
	if m.selection != selectionProjects || m.selectedProject != "beta" {
		t.Fatalf("selection = %v project = %q, want beta project", m.selection, m.selectedProject)
	}
}

func TestSidebarRendersSeparatePanelsAndHidesLegacyLegend(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_agent", "").(model)
	m.width = 54
	m.height = 24
	m.state = appState{CurrentProject: "garden", Projects: []projectState{
		{Name: "garden", Sessions: []sessionState{{Name: "agent", TmuxName: "garden_agent", Type: sessionTypeAgent}}},
		{Name: "alpha", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "alpha_code"}}},
	}}
	m.syncSelection()

	view := m.View()
	if strings.Count(view, "╭") < 2 || !strings.Contains(view, "Sessions") || !strings.Contains(view, "Projects") {
		t.Fatalf("sidebar did not render distinct bordered sections: %q", view)
	}
	if strings.Contains(view, "j/k move") || strings.Contains(view, "Enter open") {
		t.Fatalf("legacy key legend is visible while help is hidden: %q", view)
	}
	if !strings.Contains(view, "agent") || !strings.Contains(view, "live") {
		t.Fatalf("session badges not rendered: %q", view)
	}
	lines := strings.Split(view, "\n")
	firstPanel := -1
	firstPanelEnd := -1
	secondPanel := -1
	for i, line := range lines {
		if strings.Contains(line, "╭") {
			if firstPanel == -1 {
				firstPanel = i
			} else {
				secondPanel = i
				break
			}
		}
		if firstPanel != -1 && firstPanelEnd == -1 && strings.Contains(line, "╰") {
			firstPanelEnd = i
		}
	}
	if firstPanel < 3 || firstPanelEnd == -1 || secondPanel-firstPanelEnd < 3 {
		t.Fatalf("sidebar spacing did not keep the larger header gap and panel gap rhythm: %q", view)
	}
}

func TestSelectedSessionBadgeUsesRowHighlight(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_agent", "").(model)
	m.state = appState{CurrentProject: "garden", Projects: []projectState{{
		Name: "garden",
		Sessions: []sessionState{{
			Name:     "agent",
			TmuxName: "garden_agent",
			Type:     sessionTypeAgent,
		}},
	}}}
	m.syncSelection()

	row := m.renderSessionRow(48, "garden", m.state.Projects[0].Sessions[0])
	// Row should contain styled badges and the session name
	agentBadge := countBadgeStyle.Render("agent")
	liveBadge := currentBadgeStyle.Render("live")
	if !strings.Contains(row, agentBadge) || !strings.Contains(row, liveBadge) || !strings.Contains(row, "agent") {
		t.Fatalf("selected badge and text row rendering is unexpected: %q", row)
	}
}

func TestProjectHintsPreviewSelectedProjectSessionsAndInlineHint(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.width = 54
	m.height = 24
	m.state = appState{CurrentProject: "garden", Projects: []projectState{
		{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
		{Name: "alpha", Persistent: true, Sessions: []sessionState{{Name: "review", TmuxName: "alpha_review"}}},
	}}
	m.syncSelection()
	m.openProjectOverlay(overlaySwitchProject)

	view := m.View()
	if !strings.Contains(view, "review") {
		t.Fatalf("selected project sessions not previewed during hint flow: %q", view)
	}
	if strings.Contains(view, "[a]") || strings.Contains(view, "[alpha]") {
		t.Fatalf("hint rendered as prefixed badge instead of inline highlight: %q", view)
	}
}

func TestSessionUniquenessIsProjectScoped(t *testing.T) {
	m := newModel(fakeTmuxController{}, "garden_code", "").(model)
	m.state = appState{CurrentProject: "garden", Projects: []projectState{
		{Name: "garden", Sessions: []sessionState{{Name: "code", TmuxName: "garden_code"}}},
		{Name: "alpha", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "alpha_code"}}},
	}}
	m.syncSelection()

	updated, _ := m.startSessionCreate(sessionTypeTerminal)
	m = *updated.(*model)
	m.input.SetValue("code")
	updated, _ = m.updateCreateSession(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.status != "Session already exists in this project." {
		t.Fatalf("status = %q, want duplicate-session error", m.status)
	}
	m.input.SetValue("shell")
	_, cmd := m.updateCreateSession(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("creating a unique session in the current project returned no command")
	}
}

func TestPersistProjectRejectsDuplicateProjectName(t *testing.T) {
	input := textinput.New()
	m := model{
		currentTmuxSession: "otter_code",
		state: appState{CurrentProject: "otter", Projects: []projectState{
			{Name: "otter", Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}}},
			{Name: "work", Persistent: true, Sessions: []sessionState{{Name: "code", TmuxName: "work_code"}}},
		}},
		projectCfg: map[string]projectConfig{"work": {Name: "work"}},
		input:      input,
	}

	updated, _ := m.startPersistProject()
	m = updated.(model)
	m.input.SetValue("work")
	updated, _ = m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.input.SetValue("")
	updated, _ = m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.input.SetValue("")
	updated, _ = m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.status != "Project already exists." {
		t.Fatalf("status = %q, want duplicate-project error", m.status)
	}
}

func TestPersistProjectAllowsEmptyWorkdirAndAgentCommand(t *testing.T) {
	var renames [][2]string
	input := textinput.New()
	m := model{
		tmux: fakeTmuxController{renameSession: func(oldName, newName string) error {
			renames = append(renames, [2]string{oldName, newName})
			return nil
		}},
		cwd:                "/tmp/current",
		currentTmuxSession: "otter_code",
		state: appState{CurrentProject: "otter", Projects: []projectState{{
			Name:     "otter",
			Sessions: []sessionState{{Name: "code", TmuxName: "otter_code"}},
		}}},
		projectCfg: map[string]projectConfig{},
		input:      input,
	}

	updated, _ := m.startPersistProject()
	m = updated.(model)
	m.input.SetValue("work")
	updated, _ = m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.input.SetValue("")
	updated, _ = m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	m.input.SetValue("")
	_, cmd := m.updatePersistProject(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(projectPersistedMsg)
	if msg.err != nil {
		t.Fatalf("persist returned error: %v", msg.err)
	}
	if msg.config.Name != "work" || msg.config.Workdir != "" || msg.config.AgentBinary != "" {
		t.Fatalf("unexpected config: %#v", msg.config)
	}
	if fmt.Sprint(renames) != fmt.Sprint([][2]string{{"otter_code", "work_code"}}) {
		t.Fatalf("renames = %v", renames)
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
	sessionCWD          func(name string) (string, error)
	ensureControlMode   func(binaryPath string) error
	syncSessionMetadata func(metadata map[string]sessionMetadata) error
	toggleMenu          func(binaryPath string) error
	closePane           func(paneID string) error
	quitAll             func(paneID string) error
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
	return session{Name: name}, nil
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
	return nil, nil
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

func (f fakeTmuxController) SessionCWD(name string) (string, error) {
	if f.sessionCWD != nil {
		return f.sessionCWD(name)
	}
	return "/tmp", nil
}

func (f fakeTmuxController) EnsureControlMode(binaryPath string) error {
	if f.ensureControlMode != nil {
		return f.ensureControlMode(binaryPath)
	}
	return nil
}

func (f fakeTmuxController) SyncSessionMetadata(metadata map[string]sessionMetadata) error {
	if f.syncSessionMetadata != nil {
		return f.syncSessionMetadata(metadata)
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
