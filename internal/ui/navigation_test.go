package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNavigateBoundedPersistentContext(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name:    "alpha",
				Workdir: "/tmp/alpha",
				Sessions: []persistentSession{
					{ID: "s1", Label: "First"},
					{ID: "s2", Label: "Second"},
					{ID: "s3", Label: "Third"},
				},
			},
			{
				Name:    "beta",
				Workdir: "/tmp/beta",
				Sessions: []persistentSession{
					{ID: "b1", Label: "Other"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	sessions := []session{
		{Name: "s1", Label: "First"},
		{Name: "s2", Label: "Second"},
		{Name: "s3", Label: "Third"},
		{Name: "b1", Label: "Other"},
	}

	var switchedTo string
	topBarCalls := make(map[string]string)

	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarCalls[name] = content
			return nil
		},
	}

	// s1 -> prev (at start boundary: no-op, no switch)
	t.Setenv(menuCurrentEnv, "s1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from s1: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switched to %q at start boundary, want no-op", switchedTo)
	}
	if _, ok := topBarCalls["s1"]; !ok {
		t.Fatalf("top bar should be refreshed for s1 on boundary halt")
	}

	// s1 -> next (s2)
	t.Setenv(menuCurrentEnv, "s1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from s1: %v", err)
	}
	if switchedTo != "s2" {
		t.Fatalf("switched to %q, want s2", switchedTo)
	}
	if _, ok := topBarCalls["s2"]; !ok {
		t.Fatalf("top bar not set for target s2")
	}
	if len(topBarCalls) != 1 {
		t.Fatalf("top bar set for %d sessions, want 1", len(topBarCalls))
	}

	// s2 -> next (s3)
	t.Setenv(menuCurrentEnv, "s2")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from s2: %v", err)
	}
	if switchedTo != "s3" {
		t.Fatalf("switched to %q, want s3", switchedTo)
	}

	// s3 -> next (at end boundary: no-op, no switch)
	t.Setenv(menuCurrentEnv, "s3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from s3: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switched to %q at end boundary, want no-op", switchedTo)
	}
	if _, ok := topBarCalls["s3"]; !ok {
		t.Fatalf("top bar should be refreshed for s3 on boundary halt")
	}

	// s3 -> prev (s2)
	t.Setenv(menuCurrentEnv, "s3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from s3: %v", err)
	}
	if switchedTo != "s2" {
		t.Fatalf("switched to %q, want s2", switchedTo)
	}
}

func TestNavigateBoundedVolatileContext(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := saveAppState(statePath, appState{}); err != nil {
		t.Fatal(err)
	}

	sessions := []session{
		{Name: "tflow-v-1", Temporary: true, Instance: "inst-1", Label: "One"},
		{Name: "tflow-v-2", Temporary: true, Instance: "inst-1", Label: "Two"},
		{Name: "tflow-v-3", Temporary: true, Instance: "inst-1", Label: "Three"},
		{Name: "tflow-v-other", Temporary: true, Instance: "inst-2", Label: "OtherInst"},
	}

	var switchedTo string
	topBarCalls := make(map[string]string)

	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarCalls[name] = content
			return nil
		},
	}

	t.Setenv(menuInstanceEnv, "inst-1")

	// tflow-v-1 -> prev (at start boundary: no-op)
	t.Setenv(menuCurrentEnv, "tflow-v-1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from tflow-v-1: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switched to %q at start boundary, want no-op", switchedTo)
	}

	// tflow-v-1 -> next (tflow-v-2)
	t.Setenv(menuCurrentEnv, "tflow-v-1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from tflow-v-1: %v", err)
	}
	if switchedTo != "tflow-v-2" {
		t.Fatalf("switched to %q, want tflow-v-2", switchedTo)
	}
	if _, ok := topBarCalls["tflow-v-2"]; !ok {
		t.Fatalf("top bar not set for target tflow-v-2")
	}

	// tflow-v-3 -> next (at end boundary: no-op)
	t.Setenv(menuCurrentEnv, "tflow-v-3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from tflow-v-3: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switched to %q at end boundary, want no-op", switchedTo)
	}

	// tflow-v-3 -> prev (tflow-v-2)
	t.Setenv(menuCurrentEnv, "tflow-v-3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from tflow-v-3: %v", err)
	}
	if switchedTo != "tflow-v-2" {
		t.Fatalf("switched to %q, want tflow-v-2", switchedTo)
	}
}

func TestNavigatePrefersCurrentSessionInstanceOverAmbientEnvironment(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{}); err != nil {
		t.Fatal(err)
	}

	var switchedTo string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "tflow-v-live-1", Temporary: true, Instance: "live", Label: "one"},
				{Name: "tflow-v-live-2", Temporary: true, Instance: "live", Label: "two"},
				{Name: "tflow-v-stale", Temporary: true, Instance: "stale", Label: "stale"},
			}, nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "tflow-v-live-1")
	t.Setenv(menuInstanceEnv, "stale")

	if err := navigateWithManager(manager, 1); err != nil {
		t.Fatalf("navigate next: %v", err)
	}
	if switchedTo != "tflow-v-live-2" {
		t.Fatalf("switched to %q, want current session instance sibling", switchedTo)
	}
}

func TestNavigateRequiresCurrentVolatileSessionInstance(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{}); err != nil {
		t.Fatal(err)
	}

	var switchedTo string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "tflow-v-unmarked", Temporary: true, Label: "current"},
				{Name: "tflow-v-foreign", Temporary: true, Instance: "foreign", Label: "foreign"},
			}, nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "tflow-v-unmarked")
	t.Setenv(menuInstanceEnv, "foreign")

	if err := navigateWithManager(manager, 1); err != nil {
		t.Fatalf("navigate next: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switched to %q without proven current-session ownership", switchedTo)
	}
}

func TestNavigateTwoSessionsBoundedBackAndForth(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name:    "pair",
				Workdir: "/tmp/pair",
				Sessions: []persistentSession{
					{ID: "p1", Label: "First"},
					{ID: "p2", Label: "Second"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	sessions := []session{
		{Name: "p1", Label: "First"},
		{Name: "p2", Label: "Second"},
	}

	var switchedTo string
	topBarCalls := make(map[string]string)

	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarCalls[name] = content
			return nil
		},
	}

	// At p1 (first session):
	// h (prev) halts at start boundary -> no-op
	t.Setenv(menuCurrentEnv, "p1")
	switchedTo = ""
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev at p1: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switchedTo = %q, want empty (boundary halt)", switchedTo)
	}

	// l (next) advances from p1 to p2
	t.Setenv(menuCurrentEnv, "p1")
	switchedTo = ""
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from p1: %v", err)
	}
	if switchedTo != "p2" {
		t.Fatalf("switchedTo = %q, want p2", switchedTo)
	}

	// At p2 (second session):
	// l (next) halts at end boundary -> no-op
	t.Setenv(menuCurrentEnv, "p2")
	switchedTo = ""
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next at p2: %v", err)
	}
	if switchedTo != "" {
		t.Fatalf("switchedTo = %q, want empty (boundary halt)", switchedTo)
	}

	// h (prev) goes back from p2 to p1
	t.Setenv(menuCurrentEnv, "p2")
	switchedTo = ""
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from p2: %v", err)
	}
	if switchedTo != "p1" {
		t.Fatalf("switchedTo = %q, want p1", switchedTo)
	}
}

func TestNavigateSingleSessionNoOp(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "solo",
				Sessions: []persistentSession{
					{ID: "solo-1", Label: "Single"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	sessions := []session{
		{Name: "solo-1", Label: "Single"},
	}

	switchCalled := false
	topBarCalls := make(map[string]string)

	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		switchClient: func(name string) error {
			switchCalled = true
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarCalls[name] = content
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "solo-1")
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate 1-session: %v", err)
	}
	if switchCalled {
		t.Fatal("switchClient called for 1-session context")
	}
	if _, ok := topBarCalls["solo-1"]; !ok {
		t.Fatal("top bar was not refreshed for solo-1")
	}
}

func TestNavigateLazyMaterialization(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name:    "alpha",
				Workdir: "/project/alpha",
				Sessions: []persistentSession{
					{ID: "s1", Label: "Running"},
					{ID: "s2", Label: "Lazy"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	// Only s1 is running in tmux
	sessions := []session{
		{Name: "s1", Label: "Running"},
	}

	var createdName, createdDir string
	var projectSet, labelSet string
	var switchedTo string
	topBarCalls := make(map[string]string)

	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			createdName = name
			createdDir = cwd
			return session{Name: name}, nil
		},
		setSessionProject: func(name, project string) error {
			projectSet = project
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelSet = label
			return nil
		},
		switchClient: func(name string) error {
			switchedTo = name
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			topBarCalls[name] = content
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "s1")
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from s1: %v", err)
	}

	if createdName != "s2" {
		t.Fatalf("created session = %q, want s2", createdName)
	}
	if createdDir != "/project/alpha" {
		t.Fatalf("created dir = %q, want /project/alpha", createdDir)
	}
	if projectSet != "alpha" {
		t.Fatalf("project marker = %q, want alpha", projectSet)
	}
	if labelSet != "Lazy" {
		t.Fatalf("label marker = %q, want Lazy", labelSet)
	}
	if switchedTo != "s2" {
		t.Fatalf("switched to %q, want s2", switchedTo)
	}
	if _, ok := topBarCalls["s2"]; !ok {
		t.Fatal("top bar not set on materialized s2")
	}
}

func TestNavigateDoesNotPerformDeadSessionCleanup(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	statePath := appStatePath()
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}

	state := appState{
		Projects: []storedProject{
			{
				Name: "alpha",
				Sessions: []persistentSession{
					{ID: "s1", Label: "DeadPanes"},
					{ID: "s2", Label: "Other"},
				},
			},
		},
	}
	if err := saveAppState(statePath, state); err != nil {
		t.Fatal(err)
	}

	sessions := []session{
		{Name: "s1", Label: "DeadPanes"},
		{Name: "s2", Label: "Other"},
	}

	killedSessions := []string{}
	fake := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return sessions, nil
		},
		sessionPanesAllDead: func(name string) (bool, error) {
			return true, nil
		},
		killSession: func(name string) error {
			killedSessions = append(killedSessions, name)
			return nil
		},
		switchClient: func(name string) error {
			return nil
		},
		setSessionTopBar: func(name, content string) error {
			return nil
		},
	}

	t.Setenv(menuCurrentEnv, "s1")
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next: %v", err)
	}
	if len(killedSessions) > 0 {
		t.Fatalf("navigation must not trigger dead-session cleanup, killed: %v", killedSessions)
	}
}

func TestNavigateLocksBeforeReloadingPersistentMembership(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	initial := appState{Projects: []storedProject{{
		Name: "alpha",
		Sessions: []persistentSession{
			{ID: "s1", Label: "Current"},
			{ID: "s2", Label: "Deleted concurrently"},
		},
	}}}
	if err := saveAppState(path, initial); err != nil {
		t.Fatal(err)
	}

	originalLock := lockAppState
	lockCalls := 0
	lockAppState = func(statePath string) (func() error, error) {
		lockCalls++
		latest := appState{Projects: []storedProject{{
			Name:     "alpha",
			Sessions: []persistentSession{{ID: "s1", Label: "Current"}},
		}}}
		if err := saveAppState(statePath, latest); err != nil {
			return nil, err
		}
		return func() error { return nil }, nil
	}
	t.Cleanup(func() { lockAppState = originalLock })

	created, switched := 0, 0
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "s1", Label: "Current"}}, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			created++
			return session{Name: name}, nil
		},
		switchClient: func(name string) error {
			switched++
			return nil
		},
	}
	t.Setenv(menuCurrentEnv, "s1")
	if err := navigateWithManager(manager, 1); err != nil {
		t.Fatalf("navigate next: %v", err)
	}
	if lockCalls != 1 {
		t.Fatalf("state lock calls = %d, want 1", lockCalls)
	}
	if created != 0 || switched != 0 {
		t.Fatalf("created = %d, switched = %d; stale deleted target must remain untouched", created, switched)
	}
}

func TestNavigateHoldsStateLockThroughLazyMaterialization(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	if err := saveAppState(path, appState{Projects: []storedProject{{
		Name: "alpha", Workdir: "/project/alpha",
		Sessions: []persistentSession{
			{ID: "s1", Label: "Current"},
			{ID: "s2", Label: "Lazy"},
		},
	}}}); err != nil {
		t.Fatal(err)
	}

	originalLock := lockAppState
	locked := false
	lockAppState = func(string) (func() error, error) {
		locked = true
		return func() error {
			locked = false
			return nil
		}, nil
	}
	t.Cleanup(func() { lockAppState = originalLock })

	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			if !locked {
				t.Fatal("state lock was not held while listing tmux sessions")
			}
			return []session{{Name: "s1", Label: "Current"}}, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			if !locked {
				t.Fatal("state lock was released before lazy materialization")
			}
			return session{Name: name}, nil
		},
	}
	t.Setenv(menuCurrentEnv, "s1")
	if err := navigateWithManager(manager, 1); err != nil {
		t.Fatalf("navigate next: %v", err)
	}
	if locked {
		t.Fatal("state lock was not released after navigation")
	}
}

func TestNavigateCleansUpLazySessionWhenMarkerSetupFails(t *testing.T) {
	tests := []struct {
		name        string
		failProject bool
	}{
		{name: "project marker", failProject: true},
		{name: "label marker"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			path := appStatePath()
			if err := saveAppState(path, appState{Projects: []storedProject{{
				Name: "alpha", Workdir: "/project/alpha",
				Sessions: []persistentSession{
					{ID: "s1", Label: "Current"},
					{ID: "s2", Label: "Lazy"},
				},
			}}}); err != nil {
				t.Fatal(err)
			}

			setupErr := errors.New("marker setup failed")
			var killed []string
			switched := 0
			manager := fakeTmuxController{
				listSessions: func() ([]session, error) {
					return []session{{Name: "s1", Label: "Current"}}, nil
				},
				createSession: func(name, cwd, command string) (session, error) {
					return session{Name: name}, nil
				},
				setSessionProject: func(name, project string) error {
					if tc.failProject {
						return setupErr
					}
					return nil
				},
				setSessionLabel: func(name, label string) error {
					if !tc.failProject {
						return setupErr
					}
					return nil
				},
				killSession: func(name string) error {
					killed = append(killed, name)
					return nil
				},
				switchClient: func(name string) error {
					switched++
					return nil
				},
			}
			t.Setenv(menuCurrentEnv, "s1")
			err := navigateWithManager(manager, 1)
			if !errors.Is(err, setupErr) {
				t.Fatalf("navigate error = %v, want marker setup error", err)
			}
			if len(killed) != 1 || killed[0] != "s2" {
				t.Fatalf("killed sessions = %#v, want s2", killed)
			}
			if switched != 0 {
				t.Fatalf("switch calls = %d, want 0 after marker failure", switched)
			}
		})
	}
}

func TestRunMenuExitActionNavigatesCommandSidebarAction(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(menuCurrentEnv, "s1")

	var switched string
	menu := model{exitAction: menuExitNavigate, exitNavigateDirection: 1}
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{
				{Name: "s1", Temporary: true, Instance: "instance-1"},
				{Name: "s2", Temporary: true, Instance: "instance-1"},
			}, nil
		},
		switchClient: func(name string) error {
			switched = name
			return nil
		},
	}

	if err := runMenuExitAction(manager, menu); err != nil {
		t.Fatalf("runMenuExitAction: %v", err)
	}
	if switched != "s2" {
		t.Fatalf("switched to %q, want s2", switched)
	}
}
