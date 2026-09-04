package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNavigateWraparoundPersistentContext(t *testing.T) {
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

	// s3 -> next (wraparound to s1)
	t.Setenv(menuCurrentEnv, "s3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from s3: %v", err)
	}
	if switchedTo != "s1" {
		t.Fatalf("switched to %q, want s1 (wraparound)", switchedTo)
	}
	if _, ok := topBarCalls["s1"]; !ok {
		t.Fatalf("top bar not set for target s1")
	}

	// s1 -> prev (wraparound to s3)
	t.Setenv(menuCurrentEnv, "s1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from s1: %v", err)
	}
	if switchedTo != "s3" {
		t.Fatalf("switched to %q, want s3 (wraparound)", switchedTo)
	}
	if _, ok := topBarCalls["s3"]; !ok {
		t.Fatalf("top bar not set for target s3")
	}
}

func TestNavigateWraparoundVolatileContext(t *testing.T) {
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

	// tflow-v-3 -> next (wraparound to tflow-v-1, skipping other instances)
	t.Setenv(menuCurrentEnv, "tflow-v-3")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, 1); err != nil {
		t.Fatalf("navigate next from tflow-v-3: %v", err)
	}
	if switchedTo != "tflow-v-1" {
		t.Fatalf("switched to %q, want tflow-v-1", switchedTo)
	}

	// tflow-v-1 -> prev (wraparound to tflow-v-3)
	t.Setenv(menuCurrentEnv, "tflow-v-1")
	switchedTo = ""
	topBarCalls = make(map[string]string)
	if err := navigateWithManager(fake, -1); err != nil {
		t.Fatalf("navigate prev from tflow-v-1: %v", err)
	}
	if switchedTo != "tflow-v-3" {
		t.Fatalf("switched to %q, want tflow-v-3", switchedTo)
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
