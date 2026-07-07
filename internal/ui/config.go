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
	Projects    []projectConfig
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
		Theme: "catppuccin",
	}
}

func normalizeAppConfig(baseDir string, cfg appConfig) appConfig {
	cfg.Theme = strings.TrimSpace(strings.ToLower(cfg.Theme))
	if cfg.Theme == "" {
		cfg.Theme = "catppuccin"
	}
	cfg.ProjectsDir = strings.TrimSpace(cfg.ProjectsDir)
	if cfg.ProjectsDir != "" {
		cfg.ProjectsDir = normalizeCWD(cfg.ProjectsDir)
	}
	cfg.Colors = normalizeThemeOverrides(cfg.Colors)

	seen := map[string]struct{}{}
	projects := make([]projectConfig, 0, len(cfg.Projects))
	for _, project := range cfg.Projects {
		project = normalizeProjectConfig(project)
		if project.Name == "" {
			continue
		}
		if _, ok := seen[project.Name]; ok {
			continue
		}
		seen[project.Name] = struct{}{}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})
	cfg.Projects = projects
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

func saveAppConfigForDir(baseDir string, cfg appConfig) error {
	cfg = normalizeAppConfig(baseDir, cfg)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(baseDir, "config.yaml"), marshalAppConfigForDir(baseDir, cfg), 0o644)
}

func saveDefaultAppConfig() error {
	baseDir := appConfigDir()
	path := appConfigPath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	return saveAppConfigForDir(baseDir, defaultAppConfig(baseDir))
}

func marshalAppConfig(cfg appConfig) []byte {
	return marshalAppConfigForDir(appConfigDir(), cfg)
}

func marshalAppConfigForDir(baseDir string, cfg appConfig) []byte {
	cfg = normalizeAppConfig(baseDir, cfg)
	var buf bytes.Buffer
	if len(cfg.Projects) > 0 {
		buf.WriteString("projects:\n")
		for _, project := range cfg.Projects {
			buf.WriteString("  - name: ")
			buf.WriteString(yamlString(project.Name))
			buf.WriteByte('\n')
			if project.Workdir != "" {
				buf.WriteString("    workdir: ")
				buf.WriteString(yamlString(project.Workdir))
				buf.WriteByte('\n')
			}
			if project.AgentBinary != "" {
				buf.WriteString("    agent-cmd: ")
				buf.WriteString(yamlString(project.AgentBinary))
				buf.WriteByte('\n')
			}
		}
	}
	if cfg.Theme != "" && cfg.Theme != "catppuccin" {
		buf.WriteString("theme: ")
		buf.WriteString(yamlString(cfg.Theme))
		buf.WriteByte('\n')
	}
	if cfg.ProjectsDir != "" {
		buf.WriteString("projects-dir: ")
		buf.WriteString(yamlString(cfg.ProjectsDir))
		buf.WriteByte('\n')
	}

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
		case "projects":
			if strings.TrimSpace(value) != "" {
				return cfg, fmt.Errorf("line %d: projects must be a list", i+1)
			}
			projects, next, err := parseAppConfigProjects(lines, i)
			if err != nil {
				return cfg, err
			}
			cfg.Projects = append(cfg.Projects, projects...)
			i = next
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

func parseAppConfigProjects(lines []string, index int) ([]projectConfig, int, error) {
	projects := []projectConfig{}
	for index+1 < len(lines) {
		next := lines[index+1]
		if shouldSkipYAMLLine(next) {
			index++
			continue
		}
		if !strings.HasPrefix(next, "  - ") {
			break
		}
		index++
		cfg := projectConfig{}
		item := strings.TrimSpace(strings.TrimPrefix(lines[index], "  - "))
		if item != "" {
			if err := parseAppConfigProjectField(&cfg, item, index+1); err != nil {
				return nil, index, err
			}
		}
		for index+1 < len(lines) {
			child := lines[index+1]
			if shouldSkipYAMLLine(child) {
				index++
				continue
			}
			if strings.HasPrefix(child, "  - ") || !strings.HasPrefix(child, "    ") || strings.HasPrefix(child, "     ") {
				break
			}
			index++
			if err := parseAppConfigProjectField(&cfg, strings.TrimPrefix(lines[index], "    "), index+1); err != nil {
				return nil, index, err
			}
		}
		cfg = normalizeProjectConfig(cfg)
		if cfg.Name == "" {
			return nil, index, fmt.Errorf("line %d: missing project name", index+1)
		}
		projects = append(projects, cfg)
	}
	return projects, index, nil
}

func parseAppConfigProjectField(cfg *projectConfig, line string, lineNumber int) error {
	key, value, ok := splitYAMLField(line)
	if !ok {
		return fmt.Errorf("invalid projects line %d", lineNumber)
	}
	parsed, err := parseYAMLString(value)
	if err != nil {
		return fmt.Errorf("line %d: %w", lineNumber, err)
	}
	switch key {
	case "name":
		cfg.Name = parsed
	case "workdir":
		cfg.Workdir = parsed
	case "agent-cmd", "agent-binary":
		cfg.AgentBinary = parsed
	default:
		return fmt.Errorf("line %d: unknown project field %q", lineNumber, key)
	}
	return nil
}
