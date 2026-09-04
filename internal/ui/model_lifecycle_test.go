package ui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/diag"
)

func TestNewModelStartsWithoutProjectsWhenStateIsEmpty(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	m := newModel(fakeTmuxController{}, "").(model)
	if len(m.projects) != 0 {
		t.Fatalf("projects = %#v, want none", m.projects)
	}
	if m.selectedProject != "" {
		t.Fatalf("selectedProject = %q, want empty", m.selectedProject)
	}
}

func TestBuildModelFailsWhenStoreIsInvalid(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	path := appStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := buildModel(fakeTmuxController{}, "")
	if err == nil {
		t.Fatal("buildModel returned nil error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want path %q", err, path)
	}
}

func TestRunMenuExitActionSwitchesClientAfterExit(t *testing.T) {
	var switched []string
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error {
			switched = append(switched, name)
			return nil
		},
	}, model{exitAction: menuExitSwitchSession, exitSessionName: "dev"})
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(switched), fmt.Sprint([]string{"dev"}); got != want {
		t.Fatalf("switches = %s, want %s", got, want)
	}
}

func TestRunMenuExitActionQuitsCurrentInstance(t *testing.T) {
	called := false
	err := runMenuExitAction(fakeTmuxController{
		quitAll: func() error {
			called = true
			return nil
		},
	}, model{exitAction: menuExitQuit})
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if !called {
		t.Fatal("QuitAll was not called")
	}
}

func TestRunMenuExitActionRemovesDeadVolatileOutgoingSession(t *testing.T) {
	var killed []string
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "dev",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}, {Name: "dev"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) {
			if name != "otter-temp" {
				t.Fatalf("SessionPanesAllDead called for %q, want otter-temp", name)
			}
			return true, nil
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(killed), fmt.Sprint([]string{"otter-temp"}); got != want {
		t.Fatalf("killed = %s, want %s", got, want)
	}
}

func TestRunMenuExitActionRemovesDeadPersistentOutgoingSessionAndItsMetadata(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-old", Label: "otter"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	var killed []string
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "tflow-p-new",
		currentSession:  "tflow-p-old",
		sessions:        []session{{Name: "tflow-p-old"}, {Name: "tflow-p-new"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { return true, nil },
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if got, want := fmt.Sprint(killed), fmt.Sprint([]string{"tflow-p-old"}); got != want {
		t.Fatalf("killed = %s, want %s", got, want)
	}
	got, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 0 {
		t.Fatalf("state = %#v, want the dead outgoing session's metadata (and its now-empty project) removed", got)
	}
}

func TestRunMenuExitActionPreservesOutgoingSessionWithLivePanes(t *testing.T) {
	var killed []string
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "dev",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}, {Name: "dev"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { return false, nil },
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want no cleanup for a session with at least one live pane", killed)
	}
}

func TestRunMenuExitActionPreservesOutgoingSessionWithMixedPanes(t *testing.T) {
	// SessionPanesAllDead itself is responsible for reporting false when only
	// some panes are dead (covered directly in internal/tmux); this exercises
	// that runMenuExitAction correctly treats a false result -- whether from
	// all-live or mixed panes -- as "do not clean up."
	var killed []string
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "dev",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}, {Name: "dev"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { return false, nil }, // mixed dead/live panes
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want no cleanup for a session with mixed dead and live panes", killed)
	}
}

func TestRunMenuExitActionNeverCleansUpWhenSwitchFails(t *testing.T) {
	// The dead-pane check runs before the switch (per the architecture), so
	// it legitimately still fires here -- what must never happen is the
	// actual removal (kill / metadata update), since the switch itself
	// failed.
	var killed []string
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "dev",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}, {Name: "dev"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient:        func(name string) error { return fmt.Errorf("tmux: switch failed") },
		sessionPanesAllDead: func(name string) (bool, error) { return true, nil },
		killSession:         func(name string) error { killed = append(killed, name); return nil },
	}, menu)
	if err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("runMenuExitAction error = %v, want the original switch failure", err)
	}
	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want no cleanup when the switch failed", killed)
	}
}

func TestRunMenuExitActionKeepsPersistentDeletionWhenFallbackSwitchFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-old", Label: "old"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var killed []string
	menu := model{
		exitAction:         menuExitSwitchSession,
		exitSessionName:    "tflow-v-fallback",
		exitDeleteSessions: []string{"tflow-p-old"},
		currentSession:     "tflow-p-old",
		statePath:          path,
		sessions:           []session{{Name: "tflow-p-old"}, {Name: "tflow-v-fallback", Temporary: true}},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return fmt.Errorf("tmux: switch failed") },
		killSession:  func(name string) error { killed = append(killed, name); return nil },
	}, menu)
	if err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("runMenuExitAction error = %v, want fallback switch failure", err)
	}
	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want no persistent deletion before the fallback switch succeeds", killed)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 {
		t.Fatalf("persisted state = %#v, want deletion metadata retained", persisted)
	}
}

func TestRunMenuExitActionKillsDeferredSessionsAbsentFromMenuModel(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{
			{ID: "tflow-p-running", Label: "running"},
			{ID: "tflow-p-lazy", Label: "lazy"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}

	var killed []string
	menu := model{
		exitAction:         menuExitSwitchSession,
		exitSessionName:    "tflow-v-fallback",
		exitDeleteSessions: []string{"tflow-p-running", "tflow-p-lazy"},
		currentSession:     "tflow-p-running",
		statePath:          path,
		sessions: []session{
			{Name: "tflow-p-running"},
			{Name: "tflow-v-fallback", Temporary: true},
		},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return nil },
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	wantKilled := []string{"tflow-p-running", "tflow-p-lazy"}
	if fmt.Sprint(killed) != fmt.Sprint(wantKilled) {
		t.Fatalf("killed = %#v, want %#v", killed, wantKilled)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 0 {
		t.Fatalf("persisted state = %#v, want project and sessions removed", persisted)
	}
}

func TestRunMenuExitActionReportsDiagnosticWhenTmuxCleanupFails(t *testing.T) {
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "dev",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}, {Name: "dev"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { return true, nil },
		killSession:         func(name string) error { return fmt.Errorf("tmux: kill failed") },
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v, want the successful switch preserved despite cleanup failing", err)
	}
	if !strings.Contains(buf.String(), "kill failed") {
		t.Fatalf("diagnostic output = %q, want a kill-failure diagnostic", buf.String())
	}
}

func TestRunMenuExitActionReportsDiagnosticWhenMetadataPersistenceFails(t *testing.T) {
	// XDG_STATE_HOME points at a regular file instead of a directory, so
	// SaveAppState's os.MkdirAll fails for any write attempt.
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_STATE_HOME", blocked)

	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "tflow-p-new",
		currentSession:  "tflow-p-old",
		sessions:        []session{{Name: "tflow-p-old"}, {Name: "tflow-p-new"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { return true, nil },
		killSession:         func(name string) error { return nil },
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v, want the successful switch preserved despite metadata persistence failing", err)
	}
	if !strings.Contains(buf.String(), "remove persisted metadata") {
		t.Fatalf("diagnostic output = %q, want a metadata-persistence failure diagnostic", buf.String())
	}
}

func TestRunMenuExitActionNeverCleansUpNoOpReselectionOfCurrentSession(t *testing.T) {
	checked := false
	menu := model{
		exitAction:      menuExitSwitchSession,
		exitSessionName: "otter-temp",
		currentSession:  "otter-temp",
		sessions:        []session{{Name: "otter-temp", Temporary: true}},
	}
	err := runMenuExitAction(fakeTmuxController{
		sessionPanesAllDead: func(name string) (bool, error) { checked = true; return true, nil },
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if checked {
		t.Fatal("dead-pane cleanup ran for a no-op reselection of the already-current session")
	}
}

func TestPrepareStartupValidatesStateBeforeTmuxWork(t *testing.T) {
	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	path := filepath.Join(stateHome, "tflow", "store.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	called := false
	_, err := prepareStartup(fakeTmuxController{
		listSessions: func() ([]session, error) {
			called = true
			return nil, nil
		},
	}, "/tmp/tflow", "/tmp/project", "instance-1")
	if err == nil {
		t.Fatal("prepareStartup returned nil error for invalid state")
	}
	if called {
		t.Fatal("tmux work started before state validation")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want state path", err)
	}
}

func TestPrepareStartupRollsBackCreatedSessionOnLaterFailure(t *testing.T) {
	for _, test := range []struct {
		name        string
		failTag     bool
		failControl bool
		want        []string
	}{
		{name: "tag", failTag: true, want: []string{"create", "tag", "kill"}},
		{name: "control", failControl: true, want: []string{"create", "tag", "control", "kill"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			var calls []string
			manager := fakeTmuxController{
				listSessions: func() ([]session, error) { return nil, nil },
				createSession: func(name, cwd, command string) (session, error) {
					calls = append(calls, "create")
					return session{Name: name}, nil
				},
				setSessionTemporary: func(name string, temporary bool, instanceID string) error {
					calls = append(calls, "tag")
					if test.failTag {
						return fmt.Errorf("tag failed")
					}
					return nil
				},
				ensureControlMode: func(binaryPath string) error {
					calls = append(calls, "control")
					if test.failControl {
						return fmt.Errorf("control failed")
					}
					return nil
				},
				killSession: func(name string) error {
					calls = append(calls, "kill")
					return nil
				},
			}
			if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err == nil {
				t.Fatal("prepareStartup returned nil error")
			}
			if got, want := fmt.Sprint(calls), fmt.Sprint(test.want); got != want {
				t.Fatalf("calls = %s, want %s", got, want)
			}
		})
	}
}

func TestPrepareStartupEmitsDiagnosticWhenCleanupKillFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return nil, nil },
		createSession: func(name, cwd, command string) (session, error) {
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return fmt.Errorf("tag failed")
		},
		killSession: func(name string) error {
			return fmt.Errorf("kill failed")
		},
	}
	_, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err == nil || !strings.Contains(err.Error(), "tag failed") {
		t.Fatalf("prepareStartup error = %v, want the original tag failure", err)
	}
	if strings.Contains(err.Error(), "kill failed") {
		t.Fatalf("prepareStartup error = %v, want the kill failure not to replace the original error", err)
	}
	if !strings.Contains(buf.String(), "kill orphaned startup session") {
		t.Fatalf("diagnostic output = %q, want a kill-failure diagnostic", buf.String())
	}
}

func TestPrepareStartupRetainsMissingStateAfterEmptyTmuxServer(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-keep", Label: "keep"}, {ID: "tflow-p-missing", Label: "missing"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	lists := 0
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			lists++
			return nil, nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	if lists != 1 {
		t.Fatalf("session lists = %d, want 1", lists)
	}
	state, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 1 || len(state.Projects[0].Sessions) != 2 || state.Projects[0].Sessions[0].ID != "tflow-p-keep" || state.Projects[0].Sessions[1].ID != "tflow-p-missing" {
		t.Fatalf("reconciled state = %#v", state)
	}
}

// TestPrepareStartupRepairsMarkersAfterInterruptedCreation covers a session
// persisted to state (creation reached the point of writing metadata) but
// whose tmux markers were never written because the process died between
// the two steps -- startup must repair the project and label markers from
// state rather than leaving the tmux session unmarked.
func TestPrepareStartupRepairsMarkersAfterInterruptedCreation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-new", Label: "otter"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	var projectCalls, labelCalls, temporaryCalls [][3]string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return []session{{Name: "tflow-p-new"}}, nil },
		setSessionProject: func(name, project string) error {
			projectCalls = append(projectCalls, [3]string{name, project, ""})
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelCalls = append(labelCalls, [3]string{name, label, ""})
			return nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			temporaryCalls = append(temporaryCalls, [3]string{name, fmt.Sprint(temporary), instanceID})
			return nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	// prepareStartup also creates and tags a fresh volatile startup session
	// of its own, which independently calls setSessionLabel/setSessionTemporary
	// with a different, generated name -- so these assertions look for the
	// repaired persistent session's own call rather than assuming it is the
	// only one recorded.
	if !containsCall3(projectCalls, [3]string{"tflow-p-new", "small", ""}) {
		t.Fatalf("project repair calls = %#v, want a call for tflow-p-new", projectCalls)
	}
	if !containsCall3(labelCalls, [3]string{"tflow-p-new", "otter", ""}) {
		t.Fatalf("label repair calls = %#v, want a call for tflow-p-new", labelCalls)
	}
	if !containsCall3(temporaryCalls, [3]string{"tflow-p-new", "false", ""}) {
		t.Fatalf("temporary-clear repair calls = %#v, want a call for tflow-p-new", temporaryCalls)
	}
}

func containsCall3(calls [][3]string, want [3]string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

// TestPrepareStartupRepairClearsStaleMarkersAfterInterruptedPromotion covers
// a session that was promoted from volatile to persistent (its metadata now
// lives under a project in state) but whose stale @tflow-temp/@tflow-instance
// markers were never cleared because the process died mid-promotion.
func TestPrepareStartupRepairClearsStaleMarkersAfterInterruptedPromotion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "garden", Workdir: "/garden", Sessions: []persistentSession{{ID: "tflow-p-promoted", Label: "fox"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	cleared := false
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "tflow-p-promoted", Temporary: true, Instance: "instance-1"}}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			if name == "tflow-p-promoted" && !temporary && instanceID == "" {
				cleared = true
			}
			return nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("startup did not clear stale volatile markers left by an interrupted promotion")
	}
}

// TestPrepareStartupRepairAppliesTargetProjectAfterInterruptedMove covers a
// session moved to a new project in state, whose tmux @tflow-project marker
// still reflects the old project because the process died before the
// marker write that follows the state mutation.
func TestPrepareStartupRepairAppliesTargetProjectAfterInterruptedMove(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "target", Workdir: "/target", Sessions: []persistentSession{{ID: "tflow-p-moved", Label: "lynx"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	var appliedProject string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return []session{{Name: "tflow-p-moved"}}, nil },
		setSessionProject: func(name, project string) error {
			if name == "tflow-p-moved" {
				appliedProject = project
			}
			return nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	if appliedProject != "target" {
		t.Fatalf("applied project = %q, want %q", appliedProject, "target")
	}
}

func TestPrepareStartupRepairSkipsSessionsNotRepresentedInState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{}); err != nil {
		t.Fatal(err)
	}
	const unrelated = "tflow-v-instance-1-abc"
	touched := false
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: unrelated, Temporary: true, Instance: "instance-1"}}, nil
		},
		setSessionProject: func(name, project string) error {
			if name == unrelated {
				touched = true
			}
			return nil
		},
		setSessionLabel: func(name, label string) error {
			// prepareStartup also tags its own freshly created volatile
			// startup session via setSessionLabel; only flag the
			// pre-existing unrelated session repair must never touch.
			if name == unrelated {
				touched = true
			}
			return nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	if touched {
		t.Fatal("startup repair rewrote markers for a session not represented in state")
	}
}

func TestPrepareStartupRepairEmitsDiagnosticWithoutFailingStartup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-keep", Label: "otter"}},
	}}}); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	manager := fakeTmuxController{
		listSessions:      func() ([]session, error) { return []session{{Name: "tflow-p-keep"}}, nil },
		setSessionProject: func(name, project string) error { return fmt.Errorf("tmux: repair failed") },
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "repair") {
		t.Fatalf("diagnostic output = %q, want a marker-repair failure diagnostic", buf.String())
	}
}

// TestPrepareStartupRepairSerializesWithConcurrentMutation guards against a
// state snapshot read during marker repair going stale before the tmux
// writes it drives complete: without holding the state lock across the
// whole repair pass, a concurrent instance's rename/move landing in that
// window would have its fresher tmux markers overwritten by the older
// values repair read moments earlier -- the exact inconsistency repair
// exists to fix.
func TestPrepareStartupRepairSerializesWithConcurrentMutation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-keep", Label: "otter"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	mutationDone := make(chan error, 1)
	started := false
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return []session{{Name: "tflow-p-keep"}}, nil },
		setSessionProject: func(name, project string) error {
			if name != "tflow-p-keep" || started {
				return nil
			}
			started = true
			go func() {
				_, err := mutateAppState(path, func(state appState) (appState, error) { return state, nil })
				mutationDone <- err
			}()
			select {
			case err := <-mutationDone:
				t.Fatalf("concurrent mutation completed while marker repair held the state lock: %v", err)
			case <-time.After(50 * time.Millisecond):
			}
			return nil
		},
	}
	if _, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1"); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-mutationDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("concurrent mutation did not complete after marker repair released the state lock")
	}
}

func TestPrepareStartupPreservesStateWhenSessionListFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	state := appState{Projects: []storedProject{{Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-keep", Label: "keep"}}}}}
	if err := saveAppState(path, state); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareStartup(fakeTmuxController{
		listSessions: func() ([]session, error) { return nil, fmt.Errorf("tmux unavailable") },
	}, "/tmp/tflow", "/tmp/project", "instance-1"); err == nil {
		t.Fatal("prepareStartup returned nil error")
	}
	got, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(state) {
		t.Fatalf("state changed after session-list failure: %#v", got)
	}
}

func TestSidebarRefreshDoesNotReconcilePersistentState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	state := appState{Projects: []storedProject{{Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-missing", Label: "missing"}}}}}
	if err := saveAppState(path, state); err != nil {
		t.Fatal(err)
	}
	menu, err := buildModel(fakeTmuxController{
		listSessions: func() ([]session, error) { return nil, nil },
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	updated, _ := menu.Update(menu.loadSessionsCmd()())
	if got := updated.(model); got.err != nil {
		t.Fatal(got.err)
	}
	got, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got) != fmt.Sprint(state) {
		t.Fatalf("sidebar refresh changed state: %#v", got)
	}
}

func TestStartWithManagerCleansUpWhenAttachCommandFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var cleaned []string
	manager := fakeTmuxController{
		listSessions:        func() ([]session, error) { return nil, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			return nil, fmt.Errorf("attach unavailable")
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(context.Background())); err == nil {
		t.Fatal("startWithManager returned nil error")
	}
	if got, want := fmt.Sprint(cleaned), "[instance-1]"; got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
}

func TestStartWithManagerEmitsDiagnosticWhenCleanupFailsAfterClientError(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	manager := fakeTmuxController{
		listSessions:        func() ([]session, error) { return nil, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, "sh", "-c", "exit 1"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			return fmt.Errorf("cleanup failed")
		},
	}

	err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(context.Background()))
	if err == nil {
		t.Fatal("startWithManager returned nil error")
	}
	if strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("startWithManager error = %v, want the cleanup failure not to replace the client error", err)
	}
	if !strings.Contains(buf.String(), "clean up volatile sessions") {
		t.Fatalf("diagnostic output = %q, want a cleanup-failure diagnostic", buf.String())
	}
}

func TestStartWithManagerBuildsAttachContextOnlyAfterStartupSetup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	var calls []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			calls = append(calls, "listSessions")
			return nil, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			calls = append(calls, "setSessionTemporary")
			return nil
		},
		setSessionLabel: func(name, label string) error {
			calls = append(calls, "setSessionLabel")
			return nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "ensureControlMode")
			return nil
		},
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			calls = append(calls, "attachCommand")
			return exec.CommandContext(ctx, "sh", "-c", ":"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			return nil
		},
	}
	newAttachContext := func() (context.Context, context.CancelFunc) {
		calls = append(calls, "newAttachContext")
		return context.Background(), func() {}
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", newAttachContext); err != nil {
		t.Fatalf("startWithManager returned error: %v", err)
	}

	want := []string{
		"listSessions",
		"setSessionTemporary",
		"setSessionLabel",
		"ensureControlMode",
		"newAttachContext",
		"attachCommand",
	}
	if got := fmt.Sprint(calls); got != fmt.Sprint(want) {
		t.Fatalf("call order = %s, want %s (the signal-aware context must not be built until every non-context-aware startup call has completed)", got, want)
	}
}

func TestStartWithManagerCleansUpWhenContextIsCanceled(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cleaned []string
	manager := fakeTmuxController{
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			cancel()
			return exec.CommandContext(ctx, "sh", "-lc", "exec sleep 30"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(ctx)); err != nil {
		t.Fatalf("startWithManager returned error: %v", err)
	}
	if got, want := fmt.Sprint(cleaned), "[instance-1]"; got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
}

// TestStartWithManagerTreatsGracefulTerminationAsCancellationInduced covers
// the case a plain "was the process signal-terminated" heuristic cannot: a
// client that traps SIGTERM (sent because cmd.Cancel now asks for graceful
// termination instead of an immediate kill) and exits cleanly with its own
// nonzero code. That exit shape is indistinguishable from a genuine
// operational failure by exit code alone, so startWithManager must rely on
// having actually invoked Cancel for this process, not on the shape of the
// resulting error.
func TestStartWithManagerTreatsGracefulTerminationAsCancellationInduced(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := fakeTmuxController{
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			time.AfterFunc(50*time.Millisecond, cancel)
			// Traps SIGTERM and exits 42 (a clean, nonzero exit -- not a
			// signal-terminated one) instead of the sleep-only scripts used
			// elsewhere, which are signal-terminated by default and don't
			// exercise this case. The busy-wait loop (rather than a
			// foreground `sleep`) avoids most shells deferring trap
			// execution until a foreground child's own wait() returns.
			return exec.CommandContext(ctx, "sh", "-c", `trap 'exit 42' TERM; while :; do sleep 0.05; done`), nil
		},
	}

	start := time.Now()
	err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(ctx))
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("startWithManager returned error: %v, want the graceful-exit-42 treated as cancellation-induced", err)
	}
	if elapsed >= attachTerminationWaitDelay {
		t.Fatalf("startWithManager took %v, want it to return promptly once the client's trap exits rather than waiting out the full grace period", elapsed)
	}
}

// TestStartWithManagerForceKillsAfterWaitDelayExpires covers the bounded
// side of "graceful termination first, forceful only after a bounded
// wait": a client that ignores SIGTERM entirely must still be terminated,
// once the grace period elapses, rather than left running indefinitely.
func TestStartWithManagerForceKillsAfterWaitDelayExpires(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	original := attachTerminationWaitDelay
	attachTerminationWaitDelay = 100 * time.Millisecond
	t.Cleanup(func() { attachTerminationWaitDelay = original })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := fakeTmuxController{
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			time.AfterFunc(50*time.Millisecond, cancel)
			return exec.CommandContext(ctx, "sh", "-c", `trap '' TERM; while :; do sleep 0.05; done`), nil
		},
	}

	done := make(chan error, 1)
	go func() {
		done <- startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(ctx))
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("startWithManager returned error: %v, want the force-killed client treated as cancellation-induced", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("startWithManager did not return after the SIGTERM-ignoring client should have been force-killed")
	}
}

func TestStartWithManagerReportsAttachErrorDespitePendingCancellation(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var cleaned []string
	manager := fakeTmuxController{
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			// A signal arrives (canceling the outer context) at roughly the
			// same time the attach client independently fails for its own
			// operational reason (e.g. the target session or tmux server
			// disappeared). This command is deliberately tied to an
			// unrelated, never-canceled context rather than the outer ctx
			// (still via exec.CommandContext, since startWithManager now
			// sets Cancel/WaitDelay on the returned *exec.Cmd, which Go
			// only permits for CommandContext-constructed commands), so its
			// failure is its own, not one caused by the cancellation
			// killing it.
			cancel()
			return exec.CommandContext(context.Background(), "sh", "-c", "exit 7"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-1", fixedAttachContext(ctx))
	if err == nil {
		t.Fatal("startWithManager returned nil error, want the real attach failure reported")
	}
	if ctx.Err() == nil {
		t.Fatal("test setup invalid: context was not canceled")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("startWithManager error = %v, want the real exit status 7 from the attach command", err)
	}
	if got, want := fmt.Sprint(cleaned), "[instance-1]"; got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
}

func TestIsCancellationInducedAttachFailure(t *testing.T) {
	runExit := func(t *testing.T, code int) error {
		t.Helper()
		cmd := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code))
		return cmd.Run()
	}
	runKilled := func(t *testing.T) error {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		cmd := exec.CommandContext(ctx, "sh", "-c", "exec sleep 30")
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start process: %v", err)
		}
		cancel()
		return cmd.Wait()
	}

	if err := runExit(t, 7); !errors.As(err, new(*exec.ExitError)) {
		t.Fatalf("setup: expected *exec.ExitError, got %v (%T)", err, err)
	} else if isCancellationInducedAttachFailure(err) {
		t.Fatal("a process that exited on its own with a real error must not be treated as cancellation-induced")
	}

	if err := runKilled(t); err == nil {
		t.Fatal("setup: expected an error from killing the process")
	} else if !isCancellationInducedAttachFailure(err) {
		t.Fatalf("a process killed by context cancellation must be treated as cancellation-induced, got %v", err)
	}

	if !isCancellationInducedAttachFailure(context.Canceled) {
		t.Fatal("a bare context.Canceled error must be treated as cancellation-induced")
	}
}

func TestOpenMenuCancellationDoesNotQuitInstance(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	quit := false
	err := openMenu(ctx, fakeTmuxController{
		quitAll: func() error {
			quit = true
			return nil
		},
	}, func(ctx context.Context, menu tea.Model) (tea.Model, error) {
		return model{exitAction: menuExitQuit}, context.Canceled
	})
	if err != nil {
		t.Fatalf("openMenu returned error: %v", err)
	}
	if quit {
		t.Fatal("OpenMenu invoked QuitAll after context cancellation")
	}
}

func TestOpenMenuReportsRealErrorDespitePendingCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	realErr := errors.New("terminal write failed")
	quit := false
	err := openMenu(ctx, fakeTmuxController{
		quitAll: func() error {
			quit = true
			return nil
		},
	}, func(ctx context.Context, menu tea.Model) (tea.Model, error) {
		// A signal arrives (canceling the outer context) at roughly the
		// same time the popup program independently fails for its own,
		// unrelated reason. Mirror Bubble Tea's real wrapping shape: every
		// "killed" exit is wrapped in tea.ErrProgramKilled, whether or not
		// the underlying cause was the cancellation.
		cancel()
		return model{exitAction: menuExitQuit}, fmt.Errorf("%w: %w", tea.ErrProgramKilled, realErr)
	})
	if err == nil {
		t.Fatal("openMenu returned nil error, want the real popup failure reported")
	}
	if !errors.Is(err, realErr) {
		t.Fatalf("openMenu error = %v, want it to wrap the real popup failure %v", err, realErr)
	}
	if quit {
		t.Fatal("OpenMenu invoked QuitAll despite reporting a real error")
	}
}

func TestOpenMenuSkipsExitActionOnPureCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := false
	err := openMenu(ctx, fakeTmuxController{
		quitAll: func() error {
			quit = true
			return nil
		},
	}, func(ctx context.Context, menu tea.Model) (tea.Model, error) {
		// Mirror Bubble Tea's real wrapping shape for a pure cancellation:
		// ErrProgramKilled wrapping the context's own cancellation error,
		// with no other real error involved.
		cancel()
		return model{exitAction: menuExitQuit}, fmt.Errorf("%w: %w", tea.ErrProgramKilled, ctx.Err())
	})
	if err != nil {
		t.Fatalf("openMenu returned error: %v", err)
	}
	if quit {
		t.Fatal("OpenMenu invoked QuitAll after context cancellation")
	}
}

func TestIsCancellationInducedPopupExit(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if isCancellationInducedPopupExit(canceledCtx, nil) {
		t.Fatal("a nil error must never be treated as a cancellation-induced exit")
	}
	if !isCancellationInducedPopupExit(canceledCtx, context.Canceled) {
		t.Fatal("a bare context.Canceled error must be treated as cancellation-induced")
	}
	if !isCancellationInducedPopupExit(canceledCtx, fmt.Errorf("%w: %w", tea.ErrProgramKilled, canceledCtx.Err())) {
		t.Fatal("ErrProgramKilled wrapping the context's own error must be treated as cancellation-induced")
	}
	realErr := errors.New("terminal write failed")
	if isCancellationInducedPopupExit(canceledCtx, fmt.Errorf("%w: %w", tea.ErrProgramKilled, realErr)) {
		t.Fatal("ErrProgramKilled wrapping a real, unrelated error must not be treated as cancellation-induced")
	}
	if isCancellationInducedPopupExit(canceledCtx, realErr) {
		t.Fatal("a real, unrelated error must not be treated as cancellation-induced")
	}
	if !isCancellationInducedPopupExit(canceledCtx, fmt.Errorf("%w: %w", tea.ErrProgramKilled, tea.ErrInterrupted)) {
		t.Fatal("ErrProgramKilled wrapping ErrInterrupted while our own context is canceled must be treated as cancellation-induced")
	}

	uncanceledCtx := context.Background()
	if isCancellationInducedPopupExit(uncanceledCtx, fmt.Errorf("%w: %w", tea.ErrProgramKilled, tea.ErrInterrupted)) {
		t.Fatal("ErrInterrupted must not be treated as cancellation-induced when our own context was never canceled")
	}
}

func TestOpenMenuTreatsCompetingSignalHandlerInterruptAsGracefulCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	quit := false
	err := openMenu(ctx, fakeTmuxController{
		quitAll: func() error {
			quit = true
			return nil
		},
	}, func(ctx context.Context, menu tea.Model) (tea.Model, error) {
		// Simulate Bubble Tea's own competing SIGINT handler winning the
		// race: it reports ErrInterrupted (wrapped in ErrProgramKilled) for
		// the real signal, independent of and without wrapping our own
		// signal.NotifyContext-driven ctx, which is canceled by the same
		// SIGINT at roughly the same time.
		cancel()
		return model{exitAction: menuExitQuit}, fmt.Errorf("%w: %w", tea.ErrProgramKilled, tea.ErrInterrupted)
	})
	if err != nil {
		t.Fatalf("openMenu returned error: %v, want graceful cancellation for a real SIGINT", err)
	}
	if quit {
		t.Fatal("OpenMenu invoked QuitAll after signal-driven cancellation")
	}
}

func TestHelpToggleDoesNotBlockShortcuts(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName, "api": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got := updated.(model)
	if cmd != nil || !got.showHelp || got.mode != inputNone {
		t.Fatalf("help toggle state = %#v, cmd = %v", got, cmd)
	}

	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = updated.(model)
	if cmd != nil || got.showHelp || got.selectedSession != "api" {
		t.Fatalf("shortcut did not hide help and move selection: %#v, cmd = %v", got, cmd)
	}

	got.showHelp = true
	updated, cmd = got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = updated.(model)
	if cmd != nil || got.showHelp {
		t.Fatalf("Esc did not hide help: %#v, cmd = %v", got, cmd)
	}
}

func TestUndocumentedKeysDoNotDispatch(t *testing.T) {
	for _, test := range []struct {
		name string
		mode inputMode
		key  tea.KeyMsg
	}{
		{name: "down", key: tea.KeyMsg{Type: tea.KeyDown}},
		{name: "up", key: tea.KeyMsg{Type: tea.KeyUp}},
		{name: "delete confirmation y", mode: inputConfirmDelete, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
		{name: "delete confirmation d", mode: inputConfirmDelete, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{100}}},
		{name: "project switch confirmation y", mode: inputConfirmProjectSwitch, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
		{name: "quit confirmation y", mode: inputConfirmQuit, key: tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{121}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := newModel(fakeTmuxController{}, "").(model)
			m.mode = test.mode
			updated, cmd := m.Update(test.key)
			got := updated.(model)
			if cmd != nil {
				t.Fatal("undocumented key dispatched an action")
			}
			if got.mode != test.mode {
				t.Fatalf("mode = %v, want %v", got.mode, test.mode)
			}
		})
	}
}

func TestDeletingFinalProjectSessionRemovesProjectMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.projects = []string{"small"}
	m.selectedProject = "small"
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}

	updated, _ := m.Update(sessionKilledMsg{name: "small--code"})
	got, ok := unwrapMenuModel(updated)
	if !ok {
		t.Fatalf("updated model = %T", updated)
	}
	if len(got.projects) != 0 || len(got.projectConfigs) != 0 {
		t.Fatal("final session left project metadata")
	}
}

func TestSidebarRefreshUsesOneListQueryAndNoWrites(t *testing.T) {
	const sessionID = "tflow-p-8f42ac91"
	listCalls := 0
	markerWrites := 0
	m := newModel(fakeTmuxController{
		listSessions: func() ([]session, error) {
			listCalls++
			return []session{{Name: sessionID, Label: "code"}}, nil
		},
		setSessionProject: func(name, project string) error {
			markerWrites++
			return nil
		},
		setSessionLabel: func(name, label string) error {
			markerWrites++
			return nil
		},
	}, sessionID).(model)
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.sessions = []session{{Name: sessionID, Label: "code"}}
	m.sessionProjects = map[string]string{sessionID: "small"}
	m.sessionLabels = map[string]string{sessionID: "code"}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(m.statePath)
	if err != nil {
		t.Fatal(err)
	}

	updated, followUp := m.Update(m.Init()())
	if followUp != nil {
		t.Fatalf("refresh follow-up = %v, want nil", followUp)
	}
	after, err := os.ReadFile(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if listCalls != 1 || markerWrites != 0 {
		t.Fatalf("list calls = %d, marker writes = %d", listCalls, markerWrites)
	}
	if string(after) != string(before) {
		t.Fatalf("sidebar refresh changed persistent state\nbefore: %s\nafter:  %s", before, after)
	}
	if got := updated.(model).selectedSession; got != sessionID {
		t.Fatalf("selected session = %q, want %q", got, sessionID)
	}
}

func TestRunMenuExitActionKillsUnusedFallbackOnSwitchFailure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-old", Label: "old"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var killed []string
	menu := model{
		exitAction:          menuExitSwitchSession,
		exitSessionName:     "tflow-v-fallback",
		exitFallbackSession: "tflow-v-fallback",
		exitDeleteSessions:  []string{"tflow-p-old"},
		currentSession:      "tflow-p-old",
		statePath:           path,
		sessions:            []session{{Name: "tflow-p-old"}, {Name: "tflow-v-fallback", Temporary: true}},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return fmt.Errorf("tmux: switch failed") },
		killSession:  func(name string) error { killed = append(killed, name); return nil },
	}, menu)
	if err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("runMenuExitAction error = %v, want fallback switch failure", err)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-v-fallback"}) {
		t.Fatalf("killed = %#v, want unused fallback session killed on switch failure", killed)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 {
		t.Fatalf("persisted state = %#v, want deletion metadata retained", persisted)
	}
}

func TestRunMenuExitActionDoesNotKillTargetWhenNotFallbackOnSwitchFailure(t *testing.T) {
	var killed []string
	menu := model{
		exitAction:          menuExitSwitchSession,
		exitSessionName:     "tflow-p-sibling",
		exitFallbackSession: "",
		exitDeleteSessions:  []string{"tflow-p-old"},
		currentSession:      "tflow-p-old",
		sessions:            []session{{Name: "tflow-p-old"}, {Name: "tflow-p-sibling"}},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return fmt.Errorf("tmux: switch failed") },
		killSession:  func(name string) error { killed = append(killed, name); return nil },
	}, menu)
	if err == nil || !strings.Contains(err.Error(), "switch failed") {
		t.Fatalf("runMenuExitAction error = %v, want switch failure", err)
	}
	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want no sessions killed when sibling switch failed", killed)
	}
}

func TestDeletePersistentSessionsAfterSwitchPartialKillFailure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{
			{ID: "tflow-p-1", Label: "one"},
			{ID: "tflow-p-2", Label: "two"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	var killed []string
	menu := model{
		exitAction:         menuExitSwitchSession,
		exitSessionName:    "tflow-v-fallback",
		exitDeleteSessions: []string{"tflow-p-1", "tflow-p-2"},
		currentSession:     "tflow-p-1",
		statePath:          path,
		sessions: []session{
			{Name: "tflow-p-1"},
			{Name: "tflow-p-2"},
			{Name: "tflow-v-fallback", Temporary: true},
		},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return nil },
		killSession: func(name string) error {
			if name == "tflow-p-1" {
				return fmt.Errorf("kill failed for p1")
			}
			killed = append(killed, name)
			return nil
		},
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-p-2"}) {
		t.Fatalf("killed = %#v, want tflow-p-2 killed despite tflow-p-1 failure", killed)
	}
	if !strings.Contains(buf.String(), "kill failed for p1") {
		t.Fatalf("diagnostic = %q, want diagnostic for p1 kill failure", buf.String())
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 || persisted.Projects[0].Sessions[0].ID != "tflow-p-1" {
		t.Fatalf("persisted state = %#v, want only tflow-p-1 retained", persisted)
	}
}

func TestDeletePersistentSessionsAfterSwitchRemovesProjectWhenExplicit(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "one"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	menu := model{
		exitAction:         menuExitSwitchSession,
		exitSessionName:    "tflow-v-fallback",
		exitDeleteProject:  "small",
		exitDeleteSessions: []string{"tflow-p-1"},
		currentSession:     "tflow-p-1",
		statePath:          path,
		sessions: []session{
			{Name: "tflow-p-1"},
			{Name: "tflow-v-fallback", Temporary: true},
		},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return nil },
		killSession:  func(name string) error { return nil },
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 0 {
		t.Fatalf("persisted state = %#v, want project removed", persisted)
	}
}

func TestDeletePersistentSessionsAfterSwitchVolatileDoesNotTouchStore(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-1", Label: "one"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var killed []string
	menu := model{
		exitAction:         menuExitSwitchSession,
		exitSessionName:    "tflow-v-new",
		exitDeleteSessions: []string{"tflow-v-old"},
		currentSession:     "tflow-v-old",
		statePath:          path,
		sessions: []session{
			{Name: "tflow-v-old", Temporary: true},
			{Name: "tflow-v-new", Temporary: true},
		},
	}
	err := runMenuExitAction(fakeTmuxController{
		switchClient: func(name string) error { return nil },
		killSession:  func(name string) error { killed = append(killed, name); return nil },
	}, menu)
	if err != nil {
		t.Fatalf("runMenuExitAction returned error: %v", err)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-v-old"}) {
		t.Fatalf("killed = %#v", killed)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 1 || len(persisted.Projects[0].Sessions) != 1 {
		t.Fatalf("persisted state = %#v, volatile deletion must not touch store", persisted)
	}
}

func TestConfirmDeleteActiveNonFinalSessionSwitchesToAdjacentSibling(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error { killed = append(killed, name); return nil },
	}, "tflow-p-2").(model)
	m.projects = []string{"small"}
	m.sessions = []session{
		{Name: "tflow-p-1"},
		{Name: "tflow-p-2"},
		{Name: "tflow-p-3"},
	}
	m.sessionProjects = map[string]string{
		"tflow-p-1": "small",
		"tflow-p-2": "small",
		"tflow-p-3": "small",
	}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-2"
	m.currentSession = "tflow-p-2"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-p-2"}

	updated, cmd := m.confirmDelete()
	got := updated.(model)

	if len(killed) != 0 {
		t.Fatalf("killed = %#v, active session must not be killed before switch", killed)
	}
	if got.selectedSession != "tflow-p-3" {
		t.Fatalf("selected session = %q, want adjacent sibling tflow-p-3", got.selectedSession)
	}
	if fmt.Sprint(got.deferredDelete) != fmt.Sprint([]string{"tflow-p-2"}) {
		t.Fatalf("deferred delete = %#v, want tflow-p-2 deferred", got.deferredDelete)
	}
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "tflow-p-3" || fmt.Sprint(msg.deleteSessions) != fmt.Sprint([]string{"tflow-p-2"}) {
		t.Fatalf("menu action msg = %+v", msg)
	}
}

func TestConfirmDeleteActiveLastSessionInProjectSwitchesToPreviousSibling(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error { killed = append(killed, name); return nil },
	}, "tflow-p-2").(model)
	m.projects = []string{"small"}
	m.sessions = []session{
		{Name: "tflow-p-1"},
		{Name: "tflow-p-2"},
	}
	m.sessionProjects = map[string]string{
		"tflow-p-1": "small",
		"tflow-p-2": "small",
	}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-2"
	m.currentSession = "tflow-p-2"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-p-2"}

	updated, cmd := m.confirmDelete()
	got := updated.(model)

	if len(killed) != 0 {
		t.Fatalf("killed = %#v, active session must not be killed before switch", killed)
	}
	if got.selectedSession != "tflow-p-1" {
		t.Fatalf("selected session = %q, want previous sibling tflow-p-1", got.selectedSession)
	}
	if fmt.Sprint(got.deferredDelete) != fmt.Sprint([]string{"tflow-p-2"}) {
		t.Fatalf("deferred delete = %#v, want tflow-p-2 deferred", got.deferredDelete)
	}
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "tflow-p-1" {
		t.Fatalf("switch target = %q, want tflow-p-1", msg.switchSession)
	}
}

func TestConfirmDeleteActiveSoleVolatileSessionCreatesFallback(t *testing.T) {
	var created bool
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = true
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
		setSessionLabel:     func(name, label string) error { return nil },
	}, "tflow-v-1").(model)
	m.instanceID = "inst-1"
	m.sessions = []session{{Name: "tflow-v-1", Temporary: true, Instance: "inst-1"}}
	m.selectedSession = "tflow-v-1"
	m.currentSession = "tflow-v-1"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-v-1"}

	updated, cmd := m.confirmDelete()
	got := updated.(model)

	if fmt.Sprint(got.deferredDelete) != fmt.Sprint([]string{"tflow-v-1"}) {
		t.Fatalf("deferred delete = %#v, want tflow-v-1 deferred", got.deferredDelete)
	}
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}
	res := cmd().(sessionCreatedMsg)
	if res.err != nil || !res.fallback || !created {
		t.Fatalf("fallback creation result = %+v, created = %v", res, created)
	}
}

func TestConfirmDeleteActiveVolatileSessionWithSiblingSwitchesToSibling(t *testing.T) {
	m := newModel(fakeTmuxController{}, "tflow-v-1").(model)
	m.instanceID = "inst-1"
	m.sessions = []session{
		{Name: "tflow-v-1", Temporary: true, Instance: "inst-1"},
		{Name: "tflow-v-2", Temporary: true, Instance: "inst-1"},
	}
	m.selectedSession = "tflow-v-1"
	m.currentSession = "tflow-v-1"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-v-1"}

	updated, cmd := m.confirmDelete()
	got := updated.(model)

	if got.selectedSession != "tflow-v-2" {
		t.Fatalf("selected session = %q, want sibling tflow-v-2", got.selectedSession)
	}
	if fmt.Sprint(got.deferredDelete) != fmt.Sprint([]string{"tflow-v-1"}) {
		t.Fatalf("deferred delete = %#v, want tflow-v-1", got.deferredDelete)
	}
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	msg := cmd().(menuActionMsg)
	if msg.switchSession != "tflow-v-2" {
		t.Fatalf("switch target = %q, want tflow-v-2", msg.switchSession)
	}
}

func TestFinishSessionCreationFollowUpErrorKillsOrphanedFallback(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		killSession: func(name string) error { killed = append(killed, name); return nil },
	}, "tflow-p-old").(model)
	m.deferredDelete = []string{"tflow-p-old"}
	m.deferredDeleteProject = "small"
	m.fallbackSession = "tflow-v-new"

	origErr := fmt.Errorf("marker sync failed")
	updated, cmd := m.finishSessionCreationFollowUpError("tflow-v-new", origErr)
	got := updated.(model)

	if cmd != nil {
		t.Fatalf("expected nil cmd, got %v", cmd)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-v-new"}) {
		t.Fatalf("killed = %#v, want unconfigured fallback killed", killed)
	}
	if len(got.deferredDelete) != 0 || got.deferredDeleteProject != "" || got.fallbackSession != "" {
		t.Fatalf("deferred fields not reset: deferredDelete=%#v, project=%q, fallback=%q",
			got.deferredDelete, got.deferredDeleteProject, got.fallbackSession)
	}
	if got.err != origErr {
		t.Fatalf("err = %v, want original error %v", got.err, origErr)
	}
}

func TestFinalSessionDeletionSuccessfulHandoffEndToEnd(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "small", Sessions: []persistentSession{{ID: "tflow-p-old", Label: "old"}},
	}}}); err != nil {
		t.Fatal(err)
	}

	var killed []string
	var switched string
	controller := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{Name: name, Windows: 1}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error { return nil },
		setSessionLabel:     func(name, label string) error { return nil },
		switchClient: func(name string) error {
			switched = name
			return nil
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}

	m := newModel(controller, "tflow-p-old").(model)
	m.instanceID = "inst-1"
	m.statePath = path
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-old"}}
	m.sessionProjects = map[string]string{"tflow-p-old": "small"}
	m.sessionLabels = map[string]string{"tflow-p-old": "old"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-old"
	m.currentSession = "tflow-p-old"

	// 1. Confirm delete of final session in project
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-p-old"}
	updated, cmd := m.confirmDelete()
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}

	// 2. Fallback creation runs
	createdMsg := cmd().(sessionCreatedMsg)
	if createdMsg.err != nil || !createdMsg.fallback {
		t.Fatalf("createdMsg = %+v", createdMsg)
	}

	// 3. Fallback message updates model and triggers switch
	updated, cmd = updated.(model).Update(createdMsg)
	if cmd == nil {
		t.Fatal("expected switch command")
	}
	actionMsg := cmd().(menuActionMsg)
	if actionMsg.switchSession != createdMsg.session.Name {
		t.Fatalf("switch target = %q, want %q", actionMsg.switchSession, createdMsg.session.Name)
	}

	// 4. Model handles menuActionMsg and prepares for menu exit
	updated, _ = updated.(model).Update(actionMsg)
	menuGot := updated.(model)
	if menuGot.exitAction != menuExitSwitchSession {
		t.Fatalf("exit action = %v, want menuExitSwitchSession", menuGot.exitAction)
	}

	// 5. runMenuExitAction switches client to fallback and deletes persistent session
	err := runMenuExitAction(controller, menuGot)
	if err != nil {
		t.Fatalf("runMenuExitAction failed: %v", err)
	}

	if switched != createdMsg.session.Name {
		t.Fatalf("switched to %q, want %q", switched, createdMsg.session.Name)
	}
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-p-old"}) {
		t.Fatalf("killed = %#v, want tflow-p-old", killed)
	}
	persisted, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(persisted.Projects) != 0 {
		t.Fatalf("persisted state = %#v, want all projects removed", persisted)
	}
}

func TestFinalSessionDeletion_FallbackCreationFailure(t *testing.T) {
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{}, fmt.Errorf("tmux server unavailable")
		},
	}, "tflow-p-old").(model)
	m.instanceID = "inst-1"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-old"}}
	m.sessionProjects = map[string]string{"tflow-p-old": "small"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-old"
	m.currentSession = "tflow-p-old"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-p-old"}

	updated, cmd := m.confirmDelete()
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}
	res := cmd().(sessionCreatedMsg)
	if res.err == nil || !strings.Contains(res.err.Error(), "tmux server unavailable") {
		t.Fatalf("res.err = %v, want tmux server unavailable", res.err)
	}

	updated, followUp := updated.(model).Update(res)
	if followUp != nil {
		t.Fatalf("followUp = %v, want nil", followUp)
	}
	got := updated.(model)
	if len(got.deferredDelete) != 0 || got.deferredDeleteProject != "" || got.fallbackSession != "" {
		t.Fatalf("deferred state not cleared after creation failure: %#v, %q, %q",
			got.deferredDelete, got.deferredDeleteProject, got.fallbackSession)
	}
	if got.err == nil || !strings.Contains(got.err.Error(), "tmux server unavailable") {
		t.Fatalf("got.err = %v, want creation error preserved", got.err)
	}
}

func TestFinalSessionDeletion_FallbackConfigurationFailure(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return fmt.Errorf("tagging temporary failed")
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "tflow-p-old").(model)
	m.instanceID = "inst-1"
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "tflow-p-old"}}
	m.sessionProjects = map[string]string{"tflow-p-old": "small"}
	m.selectedProject = "small"
	m.selectedSession = "tflow-p-old"
	m.currentSession = "tflow-p-old"

	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "tflow-p-old"}

	updated, cmd := m.confirmDelete()
	if cmd == nil {
		t.Fatal("expected fallback creation command")
	}
	res := cmd().(sessionCreatedMsg)
	if res.err == nil || !strings.Contains(res.err.Error(), "tagging temporary failed") {
		t.Fatalf("res.err = %v, want tagging temporary failure", res.err)
	}
	if len(killed) != 1 {
		t.Fatalf("killed = %#v, want newly created untagged fallback killed", killed)
	}

	updated, followUp := updated.(model).Update(res)
	if followUp != nil {
		t.Fatalf("followUp = %v, want nil", followUp)
	}
	got := updated.(model)
	if len(got.deferredDelete) != 0 || got.deferredDeleteProject != "" {
		t.Fatalf("deferred delete not cleared: %#v", got.deferredDelete)
	}
	if got.err == nil || !strings.Contains(got.err.Error(), "tagging temporary failed") {
		t.Fatalf("got.err = %v, want configuration error preserved", got.err)
	}
}

func TestFinalSessionDeletion_MarkerSyncFailureKillsFallback(t *testing.T) {
	var killed []string
	m := newModel(fakeTmuxController{
		setSessionLabel: func(name, label string) error {
			if name == "tflow-v-fallback" {
				return fmt.Errorf("sync label failed")
			}
			return nil
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}, "tflow-p-old").(model)
	m.instanceID = "inst-1"
	m.deferredDelete = []string{"tflow-p-old"}
	m.deferredDeleteProject = "small"

	msg := sessionCreatedMsg{
		session:  session{Name: "tflow-v-fallback", Temporary: true, Instance: "inst-1"},
		volatile: true,
		fallback: true,
		label:    "scratch",
	}

	updated, followUp := m.Update(msg)
	if followUp != nil {
		t.Fatalf("followUp = %v, want nil", followUp)
	}
	got := updated.(model)
	if fmt.Sprint(killed) != fmt.Sprint([]string{"tflow-v-fallback"}) {
		t.Fatalf("killed = %#v, want fallback session killed on marker sync failure", killed)
	}
	if len(got.deferredDelete) != 0 || got.deferredDeleteProject != "" || got.fallbackSession != "" {
		t.Fatalf("deferred state not cleared: %#v, %q, %q", got.deferredDelete, got.deferredDeleteProject, got.fallbackSession)
	}
	if got.err == nil || !strings.Contains(got.err.Error(), "sync label failed") {
		t.Fatalf("got.err = %v, want sync label failed", got.err)
	}
}
