package ui

import (
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
