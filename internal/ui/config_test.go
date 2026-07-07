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

func TestLoadAppConfigDefaultsToNoProjects(t *testing.T) {
	baseDir := t.TempDir()

	cfg, err := loadAppConfigForDir(baseDir)
	if err != nil {
		t.Fatalf("loadAppConfigForDir returned error: %v", err)
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("projects = %#v, want none", cfg.Projects)
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
	if state.CurrentProject != "" {
		t.Fatalf("currentProject = %q, want empty", state.CurrentProject)
	}
}

func TestLoadAppStateMigratesLegacyFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data, err := json.Marshal(legacyAppState{
		Projects:        []string{"alpha"},
		SessionProjects: map[string]string{"alpha_code": "alpha"},
		SessionTypes:    map[string]string{"alpha_code": "terminal"},
	})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	state, err := loadAppState(path)
	if err != nil {
		t.Fatalf("loadAppState returned error: %v", err)
	}
	if state.CurrentProject != "alpha" {
		t.Fatalf("currentProject = %q, want alpha", state.CurrentProject)
	}
	if len(state.Projects) != 1 || len(state.Projects[0].Sessions) != 1 {
		t.Fatalf("unexpected migrated state: %#v", state)
	}
}
