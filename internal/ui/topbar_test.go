package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTopBarFormatting(t *testing.T) {
	state := appState{
		Projects: []storedProject{
			{
				Name: "solo",
				Sessions: []persistentSession{
					{ID: "s1", Label: "Alone"},
				},
			},
			{
				Name: "duo",
				Sessions: []persistentSession{
					{ID: "d1", Label: "First"},
					{ID: "d2", Label: "Second"},
				},
			},
			{
				Name: "trio",
				Sessions: []persistentSession{
					{ID: "t1", Label: "One"},
					{ID: "t2", Label: "Two"},
					{ID: "t3", Label: ""}, // empty label -> fallback to name
				},
			},
		},
	}

	// 1 session: single pill containing "Alone"
	gotSolo := computeTargetTopBar("s1", "solo", state, nil, "")
	if !strings.Contains(gotSolo, "Alone") {
		t.Fatalf("expected single pill with Alone, got: %q", gotSolo)
	}
	// Verify it does NOT contain prev/next spacing or duplicate pills
	if strings.Count(gotSolo, "Alone") != 1 {
		t.Fatalf("expected exactly 1 instance of Alone, got: %q", gotSolo)
	}

	// 2 sessions: target d1 -> First is active pill, Second is inactive text; each appears once
	gotDuo := computeTargetTopBar("d1", "duo", state, nil, "")
	if !strings.Contains(gotDuo, "First") || !strings.Contains(gotDuo, "Second") {
		t.Fatalf("duo top bar missing labels: %q", gotDuo)
	}
	if strings.Count(gotDuo, "First") != 1 || strings.Count(gotDuo, "Second") != 1 {
		t.Fatalf("expected each session to appear exactly once, got: %q", gotDuo)
	}
	if strings.Index(gotDuo, "First") >= strings.Index(gotDuo, "Second") {
		t.Fatalf("expected First < Second, got: %q", gotDuo)
	}

	// 3 sessions: target t2 -> One is inactive, Two is active pill, t3 is inactive; each appears once in order
	gotTrio := computeTargetTopBar("t2", "trio", state, nil, "")
	if !strings.Contains(gotTrio, "One") || !strings.Contains(gotTrio, "Two") || !strings.Contains(gotTrio, "t3") {
		t.Fatalf("trio top bar missing labels or fallback ID: %q", gotTrio)
	}
	if strings.Count(gotTrio, "One") != 1 || strings.Count(gotTrio, "Two") != 1 || strings.Count(gotTrio, "t3") != 1 {
		t.Fatalf("expected each session to appear once in trio, got: %q", gotTrio)
	}
	oneIdx := strings.Index(gotTrio, "One")
	twoIdx := strings.Index(gotTrio, "Two")
	threeIdx := strings.Index(gotTrio, "t3")
	if !(oneIdx < twoIdx && twoIdx < threeIdx) {
		t.Fatalf("expected order One < Two < t3, got indices: %d, %d, %d in %q", oneIdx, twoIdx, threeIdx, gotTrio)
	}
}

func TestTopBarVolatileFormatting(t *testing.T) {
	sessions := []session{
		{Name: "tflow-v-1", Temporary: true, Instance: "inst-A", Label: "Alpha"},
		{Name: "tflow-v-2", Temporary: true, Instance: "inst-A", Label: "Bravo"},
		{Name: "tflow-v-3", Temporary: true, Instance: "inst-B", Label: "Charlie"},
	}

	// In inst-A, there are only 2 sessions: Alpha and Bravo.
	got := computeTargetTopBar("tflow-v-1", "", appState{}, sessions, "inst-A")
	if !strings.Contains(got, "Alpha") || !strings.Contains(got, "Bravo") {
		t.Fatalf("volatile top bar missing labels: %q", got)
	}
	if strings.Contains(got, "Charlie") {
		t.Fatalf("volatile top bar must not include sessions from other instances: %q", got)
	}
}

func TestDeletingNonActiveVolatileSessionRefreshesActiveTopBar(t *testing.T) {
	var updatedName, updatedContent string
	m := model{
		currentSession: "tflow-v-inst-active",
		instanceID:     "inst",
		tmux: fakeTmuxController{
			setSessionTopBar: func(name, content string) error {
				updatedName = name
				updatedContent = content
				return nil
			},
		},
		sessions: []session{
			{Name: "tflow-v-inst-active", Label: "active", Temporary: true, Instance: "inst"},
			{Name: "tflow-v-inst-deleted", Label: "deleted", Temporary: true, Instance: "inst"},
		},
		sessionProjects: map[string]string{},
	}

	updated, cmd := m.Update(sessionKilledMsg{name: "tflow-v-inst-deleted"})
	if cmd == nil {
		t.Fatal("expected close-menu command")
	}
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("update error: %v", got.err)
	}
	if updatedName != "tflow-v-inst-active" {
		t.Fatalf("updated session = %q, want active volatile session", updatedName)
	}
	if !strings.Contains(updatedContent, "active") || strings.Contains(updatedContent, "deleted") {
		t.Fatalf("updated top bar = %q, want only surviving active session", updatedContent)
	}
}

func TestTopBarSwitchUpdatesTargetOnly(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "proj",
				Sessions: []persistentSession{
					{ID: "s1", Label: "One"},
					{ID: "s2", Label: "Two"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	topBarUpdated := make(map[string]int)
	fake := fakeTmuxController{
		switchClient: func(name string) error {
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarUpdated[name]++
			return nil
		},
	}

	m := model{
		statePath:       statePath,
		currentSession:  "s1",
		exitAction:      menuExitSwitchSession,
		exitSessionName: "s2",
		sessionProjects: map[string]string{"s1": "proj", "s2": "proj"},
		sessions: []session{
			{Name: "s1", Label: "One"},
			{Name: "s2", Label: "Two"},
		},
	}

	if err := runMenuExitAction(fake, m); err != nil {
		t.Fatalf("runMenuExitAction: %v", err)
	}

	if topBarUpdated["s2"] != 1 {
		t.Fatalf("top bar updated for target s2 %d times, want 1", topBarUpdated["s2"])
	}
	if topBarUpdated["s1"] != 0 {
		t.Fatalf("top bar must not be updated for inactive s1, got %d calls", topBarUpdated["s1"])
	}
}

func TestTopBarPostSwitchCleanupRefreshesTargetAgain(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "proj",
				Sessions: []persistentSession{
					{ID: "s1", Label: "Dead"},
					{ID: "s2", Label: "Alive"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	topBarUpdated := make(map[string]int)
	fake := fakeTmuxController{
		switchClient: func(name string) error {
			return nil
		},
		sessionPanesAllDead: func(name string) (bool, error) {
			return true, nil
		},
		killSession: func(name string) error {
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarUpdated[name]++
			return nil
		},
	}

	m := model{
		statePath:       statePath,
		currentSession:  "s1",
		exitAction:      menuExitSwitchSession,
		exitSessionName: "s2",
		sessionProjects: map[string]string{"s1": "proj", "s2": "proj"},
		sessions: []session{
			{Name: "s1", Label: "Dead"},
			{Name: "s2", Label: "Alive"},
		},
	}

	if err := runMenuExitAction(fake, m); err != nil {
		t.Fatalf("runMenuExitAction: %v", err)
	}

	// 1 call on initial switch, 1 call after removable dead outgoing cleanup = 2 calls on s2
	if topBarUpdated["s2"] != 2 {
		t.Fatalf("top bar updated for target s2 %d times, want 2 (initial + post-cleanup)", topBarUpdated["s2"])
	}
	if topBarUpdated["s1"] != 0 {
		t.Fatalf("top bar must not be updated for outgoing s1")
	}
}

func TestTopBarDerivedMetadataNeverPersisted(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "demo",
				Sessions: []persistentSession{
					{ID: "d1", Label: "Label1"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)
	forbidden := []string{"status-left", "top-bar", "topbar", "", "", "Label1 #[default]"}
	for _, f := range forbidden {
		if strings.Contains(rawStr, f) {
			t.Fatalf("JSON state must never contain derived top-bar metadata, found %q", f)
		}
	}
}

func TestTopBarMutationRefreshesActiveSessionOnly(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "proj",
				Sessions: []persistentSession{
					{ID: "active", Label: "Active"},
					{ID: "sibling", Label: "Sibling"},
				},
			},
			{
				Name: "other",
				Sessions: []persistentSession{
					{ID: "unrelated", Label: "Unrelated"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	topBarUpdated := make(map[string]int)
	fake := fakeTmuxController{
		setSessionTopBar: func(name, content string) error {
			topBarUpdated[name]++
			return nil
		},
		closeMenu: func() error {
			return nil
		},
		setSessionLabel: func(name, label string) error {
			return nil
		},
	}

	m := model{
		statePath:      statePath,
		stateBasePath:  statePath,
		stateBase:      state,
		currentSession: "active",
		tmux:           fake,
		sessionProjects: map[string]string{
			"active":    "proj",
			"sibling":   "proj",
			"unrelated": "other",
		},
		sessionLabels: map[string]string{
			"active":    "Active",
			"sibling":   "Sibling",
			"unrelated": "Unrelated",
		},
		sessions: []session{
			{Name: "active", Label: "Active"},
			{Name: "sibling", Label: "Sibling"},
			{Name: "unrelated", Label: "Unrelated"},
		},
		projects: []string{"proj", "other"},
		persistentSessionOrder: map[string][]string{
			"proj":  {"active", "sibling"},
			"other": {"unrelated"},
		},
	}

	// 1. Rename sibling (same context as active)
	topBarUpdated = make(map[string]int)
	updatedModel, _ := m.Update(sessionRenamedMsg{
		name:  "sibling",
		label: "RenamedSibling",
	})
	m = updatedModel.(model)
	if m.err != nil {
		t.Fatalf("sessionRenamedMsg error: %v", m.err)
	}

	if topBarUpdated["active"] != 1 {
		t.Fatalf("top bar for active session updated %d times on sibling rename, want 1", topBarUpdated["active"])
	}
	if topBarUpdated["sibling"] != 0 {
		t.Fatalf("top bar must not be updated for inactive session 'sibling' on rename")
	}
	if topBarUpdated["unrelated"] != 0 {
		t.Fatalf("top bar must not be updated for unrelated session")
	}

	// 2. Rename unrelated (different context)
	topBarUpdated = make(map[string]int)
	updatedModel, _ = m.Update(sessionRenamedMsg{
		name:  "unrelated",
		label: "RenamedOther",
	})
	m = updatedModel.(model)

	if topBarUpdated["active"] != 0 {
		t.Fatalf("top bar for active session must not update when unrelated context renames")
	}

	// 3. Non-active deletion: kill sibling
	topBarUpdated = make(map[string]int)
	updatedModel, _ = m.Update(sessionKilledMsg{
		name:    "sibling",
		project: "proj",
	})
	m = updatedModel.(model)

	if topBarUpdated["active"] != 1 {
		t.Fatalf("top bar for active session updated %d times on non-active deletion, want 1", topBarUpdated["active"])
	}
	if topBarUpdated["sibling"] != 0 {
		t.Fatalf("top bar must not update for deleted session")
	}

	// 4. Project settings change for proj (same context as active)
	tempFile := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(tempFile, []byte("workdir: /new/proj\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	topBarUpdated = make(map[string]int)
	updatedModel, _ = m.Update(projectEditorFinishedMsg{
		project:  "proj",
		tempPath: tempFile,
	})
	m = updatedModel.(model)
	if m.err != nil {
		t.Fatalf("project settings edit error: %v", m.err)
	}

	if topBarUpdated["active"] != 1 {
		t.Fatalf("top bar for active session updated %d times on project settings edit, want 1", topBarUpdated["active"])
	}

	// 5. Project settings change for other (different context)
	otherFile := filepath.Join(t.TempDir(), "other.yaml")
	if err := os.WriteFile(otherFile, []byte("workdir: /new/other\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	topBarUpdated = make(map[string]int)
	updatedModel, _ = m.Update(projectEditorFinishedMsg{
		project:  "other",
		tempPath: otherFile,
	})
	m = updatedModel.(model)

	if topBarUpdated["active"] != 0 {
		t.Fatalf("top bar for active session must not update when unrelated project settings change")
	}
}

func TestTopBarRefreshUsesMergedPersistentState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	base := appState{Projects: []storedProject{{
		Name: "proj", Workdir: "/proj",
		Sessions: []persistentSession{
			{ID: "active", Label: "Active"},
			{ID: "sibling", Label: "Sibling"},
		},
	}}}
	if err := saveAppState(path, base); err != nil {
		t.Fatal(err)
	}

	var topBar string
	m := model{
		statePath:      path,
		stateBasePath:  path,
		stateBase:      base,
		currentSession: "active",
		tmux: fakeTmuxController{
			setSessionLabel: func(name, label string) error { return nil },
			setSessionTopBar: func(name, content string) error {
				if name == "active" {
					topBar = content
				}
				return nil
			},
		},
		projects: []string{"proj"},
		projectConfigs: map[string]projectConfig{
			"proj": {Name: "proj", Workdir: "/proj"},
		},
		sessions: []session{
			{Name: "active", Label: "Active"},
			{Name: "sibling", Label: "Sibling"},
		},
		sessionProjects: map[string]string{"active": "proj", "sibling": "proj"},
		sessionLabels:   map[string]string{"active": "Active", "sibling": "Sibling"},
		persistentSessionOrder: map[string][]string{
			"proj": {"active", "sibling"},
		},
	}

	if _, err := mutateAppState(path, func(state appState) (appState, error) {
		state.Projects[0].Sessions = append(state.Projects[0].Sessions, persistentSession{
			ID: "concurrent", Label: "Concurrent",
		})
		return state, nil
	}); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(sessionRenamedMsg{name: "sibling", label: "Renamed"})
	if got := updated.(model); got.err != nil {
		t.Fatalf("rename update: %v", got.err)
	}
	if !strings.Contains(topBar, "Active") || !strings.Contains(topBar, "Renamed") || !strings.Contains(topBar, "Concurrent") {
		t.Fatalf("top bar = %q, want merged persistent labels", topBar)
	}
}
