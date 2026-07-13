package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppConfigRoundTrip(t *testing.T) {
	baseDir := t.TempDir()
	original := AppConfig{
		ProjectsDir: filepath.Join(baseDir, "custom-projects"),
		Theme:       "forest",
		Colors: ThemeOverrides{
			Blue: "#336699",
			Red:  "#993333",
		},
	}

	parsed, err := ParseAppConfig(MarshalAppConfig(original))
	if err != nil {
		t.Fatalf("ParseAppConfig returned error: %v", err)
	}

	got := NormalizeAppConfig(baseDir, parsed)
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

	cfg, err := LoadAppConfigForDir(baseDir)
	if err != nil {
		t.Fatalf("LoadAppConfigForDir returned error: %v", err)
	}
	if got, want := cfg.ProjectsDir, filepath.Join(baseDir, "projects"); got != want {
		t.Fatalf("projects-dir = %q, want %q", got, want)
	}
	if cfg.Theme != "catppuccin" {
		t.Fatalf("theme = %q, want catppuccin", cfg.Theme)
	}
}

func TestParseAppConfigAcceptsColorOverrides(t *testing.T) {
	cfg, err := ParseAppConfig([]byte(strings.Join([]string{
		`projects-dir: "/tmp/projects"`,
		`theme: "catppuccin"`,
		`colors:`,
		`  blue: "#123456"`,
		`  badge-text: "#eeeeee"`,
	}, "\n")))
	if err != nil {
		t.Fatalf("ParseAppConfig returned error: %v", err)
	}
	if cfg.Colors.Blue != "#123456" {
		t.Fatalf("blue = %q", cfg.Colors.Blue)
	}
	if cfg.Colors.BadgeText != "#eeeeee" {
		t.Fatalf("badge-text = %q", cfg.Colors.BadgeText)
	}
}

func TestLoadAppConfigUsesExplicitConfigPath(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "config.yaml"), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := LoadAppConfigForStatePath(filepath.Join(baseDir, "state.json"))
	if err != nil {
		t.Fatalf("LoadAppConfigForStatePath returned error: %v", err)
	}
	if got, want := cfg.ProjectsDir, filepath.Join(baseDir, "projects"); got != want {
		t.Fatalf("projects-dir = %q, want %q", got, want)
	}
}
