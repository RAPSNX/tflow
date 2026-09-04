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

	// 2 sessions: target d1 -> prev is Second, active is First, next is Second
	gotDuo := computeTargetTopBar("d1", "duo", state, nil, "")
	if !strings.Contains(gotDuo, "First") || !strings.Contains(gotDuo, "Second") {
		t.Fatalf("duo top bar missing labels: %q", gotDuo)
	}
	if strings.Count(gotDuo, "Second") != 2 {
		t.Fatalf("expected Second as both prev and next, got: %q", gotDuo)
	}

	// 3 sessions: target t2 -> prev is One, active is Two, next is t3 (fallback to ID)
	gotTrio := computeTargetTopBar("t2", "trio", state, nil, "")
	if !strings.Contains(gotTrio, "One") || !strings.Contains(gotTrio, "Two") || !strings.Contains(gotTrio, "t3") {
		t.Fatalf("trio top bar missing labels or fallback ID: %q", gotTrio)
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
