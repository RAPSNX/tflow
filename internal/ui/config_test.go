package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppConfigRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	original := appConfig{
		ProjectsDir: filepath.Join(baseDir, "custom-projects"),
		Theme:       "forest",
		Colors: themeOverrides{
			Blue: "#336699",
			Red:  "#993333",
		},
	}

	parsed, err := parseAppConfig(marshalAppConfig(original))
	if err != nil {
		t.Fatalf("parseAppConfig returned error: %v", err)
	}

	got := normalizeAppConfig(baseDir, parsed)
	if got.ProjectsDir != original.ProjectsDir {
		t.Fatalf("projects-dir = %q, want %q", got.ProjectsDir, original.ProjectsDir)
	}
	if got.Theme != original.Theme {
		t.Fatalf("theme = %q, want %q", got.Theme, original.Theme)
	}
	if got.Colors.Blue != "#336699" {
		t.Fatalf("blue = %q", got.Colors.Blue)
	}
}

func TestLoadAppConfigDefaultsProjectsDir(t *testing.T) {
	baseDir := t.TempDir()

	cfg, err := loadAppConfigForDir(baseDir)
	if err != nil {
		t.Fatalf("loadAppConfigForDir returned error: %v", err)
	}
	if got, want := cfg.ProjectsDir, filepath.Join(baseDir, "projects"); got != want {
		t.Fatalf("projects-dir = %q, want %q", got, want)
	}
	if cfg.Theme != "catppuccin" {
		t.Fatalf("theme = %q, want catppuccin", cfg.Theme)
	}
}

func TestParseAppConfigAcceptsColorOverrides(t *testing.T) {
	cfg, err := parseAppConfig([]byte(strings.Join([]string{
		`projects-dir: "/tmp/projects"`,
		`theme: "catppuccin"`,
		`colors:`,
		`  blue: "#123456"`,
		`  badge-text: "#eeeeee"`,
	}, "\n")))
	if err != nil {
		t.Fatalf("parseAppConfig returned error: %v", err)
	}
	if cfg.Colors.Blue != "#123456" {
		t.Fatalf("blue = %q", cfg.Colors.Blue)
	}
	if cfg.Colors.BadgeText != "#eeeeee" {
		t.Fatalf("badge-text = %q", cfg.Colors.BadgeText)
	}
}

func TestLoadAppStateDefaultsToNoProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")

	state, err := loadAppState(path)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
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
	data, err := json.Marshal(appState{
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

	if err := ensureStartupState(); err != nil {
		t.Fatalf("ensureStartupState returned error: %v", err)
	}

	state, err := loadAppState(statePath)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("projects = %#v, want none", state.Projects)
	}
}
