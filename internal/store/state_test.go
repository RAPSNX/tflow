package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppStateDefaultsToEmptyStoreWhenFileMissing(t *testing.T) {
	state, err := LoadAppState(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 0 || len(state.ProjectConfigs) != 0 || len(state.SessionProjects) != 0 || len(state.SessionLabels) != 0 {
		t.Fatalf("state = %#v, want empty", state)
	}
}

func TestSaveAndLoadAppStateRoundTripsCanonicalSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	want := AppState{
		Projects:        []string{"small", "garden"},
		SessionProjects: map[string]string{"small--otter": "small"},
		SessionLabels:   map[string]string{"small--otter": "otter"},
		ProjectConfigs: map[string]ProjectConfig{
			"small":  {Name: "small", Workdir: "/tmp/project-small"},
			"garden": {Name: "garden", Workdir: "/tmp/project-garden"},
		},
	}
	if err := SaveAppState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got.Projects, ",") != "small,garden" || got.ProjectConfigs["small"].Workdir != "/tmp/project-small" || got.SessionLabels["small--otter"] != "otter" {
		t.Fatalf("round trip = %#v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain := string(data)
	for _, want := range []string{"project_order", "projects", "session_projects", "session_labels"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("missing canonical field %s in %s", want, plain)
		}
	}
	for _, removed := range []string{"session_types", "protect", "agent_binary", "cluster"} {
		if strings.Contains(plain, removed) {
			t.Fatalf("removed field %q in %s", removed, plain)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadAppStateRejectsUnknownAndRemovedFields(t *testing.T) {
	tests := []struct {
		name  string
		state string
		field string
	}{
		{"unknown top-level", `{"project_order":[],"projects":{},"session_projects":{},"session_labels":{},"unexpected":true}`, "unexpected"},
		{"session types", `{"project_order":[],"projects":{},"session_projects":{},"session_labels":{},"session_types":{}}`, "session_types"},
		{"protect", `{"project_order":["small"],"projects":{"small":{"workdir":"/tmp","protect":true}},"session_projects":{},"session_labels":{}}`, "protect"},
		{"agent binary", `{"project_order":["small"],"projects":{"small":{"workdir":"/tmp","agent_binary":"codex"}},"session_projects":{},"session_labels":{}}`, "agent_binary"},
		{"cluster", `{"project_order":["small"],"projects":{"small":{"workdir":"/tmp","cluster":{}}},"session_projects":{},"session_labels":{}}`, "cluster"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "store.json")
			if err := os.WriteFile(path, []byte(test.state), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := LoadAppState(path)
			if err == nil || !strings.Contains(err.Error(), test.field) {
				t.Fatalf("error = %v, want offending field %q", err, test.field)
			}
		})
	}
}

func TestLoadAppStateDoesNotReadLegacyConfigState(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	legacyPath := filepath.Join(configHome, "tflow", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"project_order":["legacy"],"projects":{"legacy":{"workdir":"/tmp"}},"session_projects":{},"session_labels":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadAppState(AppStatePath())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("legacy config state was loaded: %#v", state)
	}
}

func TestNormalizeProjectListMatchesFormerCallSiteBehavior(t *testing.T) {
	got := NormalizeProjectList([]string{" Small ", "default", "Alpha/One", "small", "alpha.one", "", "alpha-one"})
	want := []string{"small", "default", "alpha-one"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("NormalizeProjectList = %#v, want %#v", got, want)
	}
}

func TestNormalizeCWDFallsBackToHomeWhenCurrentDirectoryIsUnavailable(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	home := t.TempDir()
	t.Setenv("HOME", home)
	missingDir := filepath.Join(t.TempDir(), "missing")
	if err := os.Mkdir(missingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(missingDir); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{"", "."} {
		if got := NormalizeCWD(cwd); got != home {
			t.Fatalf("NormalizeCWD(%q) = %q, want %q", cwd, got, home)
		}
	}
}
