package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type appConfig struct {
	ProjectsDir string
	Theme       string
	Colors      themeOverrides
}

type themeOverrides struct {
	BaseBG       string
	Surface0     string
	Surface1     string
	Text         string
	Subtext      string
	Blue         string
	Teal         string
	Yellow       string
	Red          string
	Mantle       string
	Crust        string
	BadgeText    string
	SelectedText string
}

func defaultAppConfig(baseDir string) appConfig {
	return appConfig{
		ProjectsDir: filepath.Join(baseDir, "projects"),
		Theme:       "catppuccin",
	}
}

func normalizeAppConfig(baseDir string, cfg appConfig) appConfig {
	cfg.Theme = strings.TrimSpace(strings.ToLower(cfg.Theme))
	if cfg.Theme == "" {
		cfg.Theme = "catppuccin"
	}
	cfg.ProjectsDir = strings.TrimSpace(cfg.ProjectsDir)
	if cfg.ProjectsDir == "" {
		cfg.ProjectsDir = filepath.Join(baseDir, "projects")
	}
	cfg.ProjectsDir = normalizeCWD(cfg.ProjectsDir)
	cfg.Colors = normalizeThemeOverrides(cfg.Colors)
	return cfg
}

func normalizeThemeOverrides(overrides themeOverrides) themeOverrides {
	normalize := func(value string) string {
		value = strings.TrimSpace(value)
		if value == "" {
			return ""
		}
		return strings.ToLower(value)
	}
	overrides.BaseBG = normalize(overrides.BaseBG)
	overrides.Surface0 = normalize(overrides.Surface0)
	overrides.Surface1 = normalize(overrides.Surface1)
	overrides.Text = normalize(overrides.Text)
	overrides.Subtext = normalize(overrides.Subtext)
	overrides.Blue = normalize(overrides.Blue)
	overrides.Teal = normalize(overrides.Teal)
	overrides.Yellow = normalize(overrides.Yellow)
	overrides.Red = normalize(overrides.Red)
	overrides.Mantle = normalize(overrides.Mantle)
	overrides.Crust = normalize(overrides.Crust)
	overrides.BadgeText = normalize(overrides.BadgeText)
	overrides.SelectedText = normalize(overrides.SelectedText)
	return overrides
}

func appConfigDir() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(dir, "tflow")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".config", "tflow")
	}
	return filepath.Join(".", ".tflow")
}

func appConfigPath() string {
	return filepath.Join(appConfigDir(), "config.yaml")
}

func loadAppConfig() (appConfig, error) {
	return loadAppConfigForDir(appConfigDir())
}

func loadAppConfigForStatePath(statePath string) (appConfig, error) {
	return loadAppConfigForDir(filepath.Dir(statePath))
}

func loadAppConfigForDir(baseDir string) (appConfig, error) {
	path := filepath.Join(baseDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultAppConfig(baseDir), nil
		}
		return appConfig{}, err
	}
	cfg, err := parseAppConfig(data)
	if err != nil {
		return appConfig{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return normalizeAppConfig(baseDir, cfg), nil
}

func saveDefaultAppConfig() error {
	baseDir := appConfigDir()
	path := appConfigPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	cfg := defaultAppConfig(baseDir)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, marshalAppConfig(cfg), 0o644)
}

func marshalAppConfig(cfg appConfig) []byte {
	cfg = normalizeAppConfig(appConfigDir(), cfg)
	var buf bytes.Buffer
	buf.WriteString("projects-dir: ")
	buf.WriteString(yamlString(cfg.ProjectsDir))
	buf.WriteByte('\n')
	buf.WriteString("theme: ")
	buf.WriteString(yamlString(cfg.Theme))
	buf.WriteByte('\n')

	colorLines := map[string]string{
		"base-bg":       cfg.Colors.BaseBG,
		"surface-0":     cfg.Colors.Surface0,
		"surface-1":     cfg.Colors.Surface1,
		"text":          cfg.Colors.Text,
		"subtext":       cfg.Colors.Subtext,
		"blue":          cfg.Colors.Blue,
		"teal":          cfg.Colors.Teal,
		"yellow":        cfg.Colors.Yellow,
		"red":           cfg.Colors.Red,
		"mantle":        cfg.Colors.Mantle,
		"crust":         cfg.Colors.Crust,
		"badge-text":    cfg.Colors.BadgeText,
		"selected-text": cfg.Colors.SelectedText,
	}
	keys := make([]string, 0, len(colorLines))
	for key, value := range colorLines {
		if value != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		buf.WriteString("colors:\n")
		for _, key := range keys {
			buf.WriteString("  ")
			buf.WriteString(key)
			buf.WriteString(": ")
			buf.WriteString(yamlString(colorLines[key]))
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func parseAppConfig(data []byte) (appConfig, error) {
	var cfg appConfig
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return cfg, err
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if shouldSkipYAMLLine(line) {
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			return cfg, fmt.Errorf("unexpected indentation on line %d", i+1)
		}
		key, value, ok := splitYAMLField(line)
		if !ok {
			return cfg, fmt.Errorf("invalid line %d", i+1)
		}

		switch key {
		case "projects-dir":
			parsed, err := parseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.ProjectsDir = parsed
		case "theme":
			parsed, err := parseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.Theme = parsed
		case "colors":
			if strings.TrimSpace(value) != "" {
				return cfg, fmt.Errorf("line %d: colors must be a map", i+1)
			}
			for i+1 < len(lines) {
				next := lines[i+1]
				if shouldSkipYAMLLine(next) {
					i++
					continue
				}
				if !strings.HasPrefix(next, "  ") || strings.HasPrefix(next, "   ") {
					break
				}
				i++
				child := strings.TrimPrefix(lines[i], "  ")
				childKey, childValue, ok := splitYAMLField(child)
				if !ok {
					return cfg, fmt.Errorf("invalid colors line %d", i+1)
				}
				parsed, err := parseYAMLString(childValue)
				if err != nil {
					return cfg, fmt.Errorf("line %d: %w", i+1, err)
				}
				switch childKey {
				case "base-bg":
					cfg.Colors.BaseBG = parsed
				case "surface-0":
					cfg.Colors.Surface0 = parsed
				case "surface-1":
					cfg.Colors.Surface1 = parsed
				case "text":
					cfg.Colors.Text = parsed
				case "subtext":
					cfg.Colors.Subtext = parsed
				case "blue":
					cfg.Colors.Blue = parsed
				case "teal":
					cfg.Colors.Teal = parsed
				case "yellow":
					cfg.Colors.Yellow = parsed
				case "red":
					cfg.Colors.Red = parsed
				case "mantle":
					cfg.Colors.Mantle = parsed
				case "crust":
					cfg.Colors.Crust = parsed
				case "badge-text":
					cfg.Colors.BadgeText = parsed
				case "selected-text":
					cfg.Colors.SelectedText = parsed
				default:
					return cfg, fmt.Errorf("line %d: unknown color field %q", i+1, childKey)
				}
			}
		default:
			return cfg, fmt.Errorf("line %d: unknown field %q", i+1, key)
		}
	}

	return cfg, nil
}
