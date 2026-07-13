package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppStateDefaultsToNoProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	state, err := LoadAppState(path)
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("projects = %#v, want none", state.Projects)
	}
	if len(state.ExpandedProjects) != 0 {
		t.Fatalf("expandedProjects = %#v, want none", state.ExpandedProjects)
	}
}

func TestEnsureStartupStatePreservesEmptyProjectList(t *testing.T) {
	configHome := t.TempDir()
	statePath := filepath.Join(configHome, "tflow", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	data, err := json.Marshal(AppState{
		Projects:         []string{},
		SessionProjects:  map[string]string{},
		SessionTypes:     map[string]string{},
		ProjectDirs:      map[string]string{},
		ExpandedProjects: map[string]bool{},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(statePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	if err := EnsureStartupState(); err != nil {
		t.Fatalf("EnsureStartupState returned error: %v", err)
	}

	state, err := LoadAppState(statePath)
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("projects = %#v, want none", state.Projects)
	}
}
