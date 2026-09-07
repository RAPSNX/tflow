package ui

import (
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
