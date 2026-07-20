package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

func TestPrepareStartupReconcilesStateWithOneSessionList(t *testing.T) {
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
			return []session{{Name: "tflow-p-keep"}}, nil
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
	if len(state.Projects) != 1 || len(state.Projects[0].Sessions) != 1 || state.Projects[0].Sessions[0].ID != "tflow-p-keep" {
		t.Fatalf("reconciled state = %#v", state)
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
			// disappeared). This command is deliberately not tied to ctx via
			// exec.CommandContext, so its failure is its own, not one caused
			// by the cancellation killing it.
			cancel()
			return exec.Command("sh", "-c", "exit 7"), nil
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

func TestProjectCreationKeepsVolatileSidebarContext(t *testing.T) {
	t.Skip("superseded by background worker coverage")
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.cwd = "/tmp/workspace"
	m.sessions = []session{{Name: "scratch-temp", Temporary: true, Instance: "instance-1"}}
	m.instanceID = "instance-1"
	m.mode = inputCreateProject
	m.input.SetValue("small")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	msg := cmd().(projectCreatedMsg)
	pending := *(updated.(*model))
	updated, _ = pending.Update(msg)
	got := updated.(model)
	if got.selectedProject != "" || got.selectedSession != "" {
		t.Fatalf("project creation changed volatile context: project %q session %q", got.selectedProject, got.selectedSession)
	}
}

func TestProjectCreationKeepsExistingProjectContext(t *testing.T) {
	m := newModel(fakeTmuxController{}, "existing--code").(model)
	m.statePath = t.TempDir() + "/store.json"
	m.projects = []string{"existing"}
	m.sessions = []session{{Name: "existing--code"}}
	m.selectedProject = "existing"
	m.selectedSession = "existing--code"
	m.sessionProjects = map[string]string{"existing--code": "existing"}
	m.sessionLabels = map[string]string{"existing--code": "code"}

	updated, _ := m.Update(projectCreatedMsg{
		config:  projectConfig{Name: "new", Workdir: "/tmp/new"},
		session: session{Name: "new--code"},
	})
	got := updated.(model)
	if got.selectedProject != "existing" || got.selectedSession != "existing--code" {
		t.Fatalf("project creation changed context: project %q session %q", got.selectedProject, got.selectedSession)
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
		syncSessionProjects: func(map[string]string) error {
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
