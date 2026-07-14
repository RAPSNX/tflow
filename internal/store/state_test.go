package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppStateDefaultsToEmptyStoreWhenFileMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")

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

func TestEnsureStartupStateCreatesEmptyStoreAtStatePath(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	setXDGEnv(t, stateHome, configHome)

	if err := EnsureStartupState(); err != nil {
		t.Fatalf("EnsureStartupState returned error: %v", err)
	}

	path := AppStatePath()
	if path != filepath.Join(stateHome, "tflow", "store.json") {
		t.Fatalf("AppStatePath = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}

	state, err := LoadAppState(path)
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("projects = %#v, want none", state.Projects)
	}
}

func TestLoadAppStateFailsOnInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	_, err := LoadAppState(path)
	if err == nil {
		t.Fatal("LoadAppState returned nil error")
	}
}

func TestLoadAppStateSupportsLegacyStringListState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data, err := json.Marshal(legacyStringListState{
		Projects:         []string{"small", "default", "small"},
		SessionProjects:  map[string]string{"dev": "small"},
		SessionTypes:     map[string]string{"dev": "agent"},
		ProjectDirs:      map[string]string{"small": "/tmp/project"},
		ExpandedProjects: map[string]bool{"small": false},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	state, err := LoadAppState(path)
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	wantProjects := []string{"small", "default"}
	if strings.Join(state.Projects, ",") != strings.Join(wantProjects, ",") {
		t.Fatalf("projects = %#v, want %#v", state.Projects, wantProjects)
	}
	if got := state.SessionTypes["dev"]; got != "agent" {
		t.Fatalf("sessionTypes[dev] = %q, want agent", got)
	}
	if got := state.ProjectDirs["small"]; got != "/tmp/project" {
		t.Fatalf("projectDirs[small] = %q", got)
	}
	if state.ExpandedProjects["small"] {
		t.Fatal("expandedProjects[small] = true, want false")
	}
}

func TestEnsureStartupStateMigratesLegacySessionSnapshot(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	setXDGEnv(t, stateHome, configHome)

	legacyPath := filepath.Join(configHome, "tflow", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	legacy := []byte(`{
  "current_project": "bug-iowait",
  "projects": [
    {
      "name": "atze",
      "persistent": true,
      "sessions": [
        {"name": "a", "tmux_name": "atze_a", "type": "terminal", "cwd": "/home/user"},
        {"name": "code", "tmux_name": "atze_code", "type": "terminal", "cwd": "/home/user"}
      ]
    },
    {
      "name": "bug-iowait",
      "persistent": true,
      "sessions": [
        {"name": "agent", "tmux_name": "bug-iowait_agent", "type": "agent", "cwd": "/home/user"},
        {"name": "code", "tmux_name": "bug-iowait_code", "type": "terminal", "cwd": "/home/user"}
      ]
    }
  ]
}`)
	if err := os.WriteFile(legacyPath, legacy, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if err := EnsureStartupState(); err != nil {
		t.Fatalf("EnsureStartupState returned error: %v", err)
	}

	state, err := LoadAppState(AppStatePath())
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	wantProjects := []string{"atze", "bug-iowait"}
	if strings.Join(state.Projects, ",") != strings.Join(wantProjects, ",") {
		t.Fatalf("projects = %#v, want %#v", state.Projects, wantProjects)
	}
	if got := state.SessionProjects["atze_a"]; got != "atze" {
		t.Fatalf("sessionProjects[atze_a] = %q", got)
	}
	if got := state.SessionProjects["atze_code"]; got != "atze" {
		t.Fatalf("sessionProjects[atze_code] = %q", got)
	}
	if got := state.SessionProjects["bug-iowait_agent"]; got != "bug-iowait" {
		t.Fatalf("sessionProjects[bug-iowait_agent] = %q", got)
	}
	if got := state.SessionProjects["bug-iowait_code"]; got != "bug-iowait" {
		t.Fatalf("sessionProjects[bug-iowait_code] = %q", got)
	}
	if got := state.SessionTypes["bug-iowait_agent"]; got != "agent" {
		t.Fatalf("sessionTypes[bug-iowait_agent] = %q", got)
	}
	if _, ok := state.SessionProjects["agent"]; ok {
		t.Fatal("sessionProjects unexpectedly used display session name key")
	}

	data, err := os.ReadFile(AppStatePath())
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	plain := string(data)
	if !strings.Contains(plain, `"project_order"`) {
		t.Fatalf("store.json missing project_order: %s", plain)
	}
	if !strings.Contains(plain, `"projects": {`) {
		t.Fatalf("store.json missing object-based projects map: %s", plain)
	}
}

func TestLoadAppStateMigratesLegacySessionSnapshotWithoutTmuxNameFallbacksToSessionName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	legacy := []byte(`{
  "projects": [
    {
      "name": "garden",
      "sessions": [
        {"name": "agent", "type": "agent"},
        {"name": "code", "tmux_name": "garden_code", "type": "terminal"}
      ]
    }
  ]
}`)
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	state, err := LoadAppState(path)
	if err != nil {
		t.Fatalf("LoadAppState returned error: %v", err)
	}
	if got := state.SessionProjects["agent"]; got != "garden" {
		t.Fatalf("sessionProjects[agent] = %q", got)
	}
	if got := state.SessionProjects["garden_code"]; got != "garden" {
		t.Fatalf("sessionProjects[garden_code] = %q", got)
	}
	if got := state.SessionTypes["agent"]; got != "agent" {
		t.Fatalf("sessionTypes[agent] = %q", got)
	}
}

func setXDGEnv(t *testing.T, stateHome, configHome string) {
	t.Helper()

	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)
}
