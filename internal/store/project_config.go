package store

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type ClusterConfig struct {
	Path          string
	ConnectionCmd string
}

type ProjectConfig struct {
	Name        string
	Workdir     string
	Cluster     ClusterConfig
	AgentBinary string
	Protect     bool
}

func NormalizeProjectConfig(cfg ProjectConfig) ProjectConfig {
	cfg.Name = normalizeProjectName(cfg.Name)
	cfg.Workdir = strings.TrimSpace(cfg.Workdir)
	if cfg.Workdir != "" {
		cfg.Workdir = normalizeCWD(cfg.Workdir)
	}
	cfg.Cluster.Path = strings.TrimSpace(cfg.Cluster.Path)
	cfg.Cluster.ConnectionCmd = strings.TrimSpace(cfg.Cluster.ConnectionCmd)
	cfg.AgentBinary = strings.TrimSpace(cfg.AgentBinary)
	return cfg
}

func (cfg ProjectConfig) AgentBinaryValue() string {
	if strings.TrimSpace(cfg.AgentBinary) == "" {
		return "codex"
	}
	return cfg.AgentBinary
}

func ProjectConfigDir(statePath string) string {
	cfg, err := LoadAppConfigForStatePath(statePath)
	if err != nil {
		return filepath.Join(filepath.Dir(statePath), "projects")
	}
	return cfg.ProjectsDir
}

func ProjectConfigPath(statePath, project string) string {
	return filepath.Join(ProjectConfigDir(statePath), normalizeProjectName(project)+".yaml")
}

func LoadProjectConfigs(statePath string, state AppState) (map[string]ProjectConfig, error) {
	configs := map[string]ProjectConfig{}

	for _, project := range state.Projects {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		cfg := configs[project]
		cfg.Name = project
		if dir := strings.TrimSpace(state.ProjectDirs[project]); dir != "" {
			cfg.Workdir = normalizeCWD(dir)
		}
		configs[project] = NormalizeProjectConfig(cfg)
	}

	dir := ProjectConfigDir(statePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return configs, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		cfg, err := ParseProjectConfig(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		cfg = NormalizeProjectConfig(cfg)
		if cfg.Name == "" {
			return nil, fmt.Errorf("parse %s: missing project name", path)
		}
		configs[cfg.Name] = cfg
	}

	return configs, nil
}

func SaveProjectConfigs(statePath string, configs map[string]ProjectConfig) error {
	dir := ProjectConfigDir(statePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		cfg := NormalizeProjectConfig(configs[name])
		if cfg.Name == "" {
			continue
		}
		if err := os.WriteFile(ProjectConfigPath(statePath, cfg.Name), MarshalProjectConfig(cfg), 0o644); err != nil {
			return err
		}
	}

	return nil
}

func RemoveProjectConfigFile(statePath, project string) error {
	err := os.Remove(ProjectConfigPath(statePath, project))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func MarshalProjectConfig(cfg ProjectConfig) []byte {
	cfg = NormalizeProjectConfig(cfg)

	var buf bytes.Buffer
	buf.WriteString("name: ")
	buf.WriteString(YAMLString(cfg.Name))
	buf.WriteByte('\n')
	if cfg.Workdir != "" {
		buf.WriteString("workdir: ")
		buf.WriteString(YAMLString(cfg.Workdir))
		buf.WriteByte('\n')
	}
	if cfg.Protect {
		buf.WriteString("protect: true\n")
	}
	if cfg.AgentBinary != "" {
		buf.WriteString("agent-binary: ")
		buf.WriteString(YAMLString(cfg.AgentBinary))
		buf.WriteByte('\n')
	}
	if cfg.Cluster.Path != "" || cfg.Cluster.ConnectionCmd != "" {
		buf.WriteString("cluster:\n")
		if cfg.Cluster.Path != "" {
			buf.WriteString("  path: ")
			buf.WriteString(YAMLString(cfg.Cluster.Path))
			buf.WriteByte('\n')
		}
		if cfg.Cluster.ConnectionCmd != "" {
			buf.WriteString("  connection-cmd: ")
			buf.WriteString(YAMLString(cfg.Cluster.ConnectionCmd))
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func ParseProjectConfig(data []byte) (ProjectConfig, error) {
	var cfg ProjectConfig
	lines, err := yamlLines(data)
	if err != nil {
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
		case "name":
			parsed, err := ParseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.Name = parsed
		case "workdir":
			parsed, err := ParseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.Workdir = parsed
		case "protect":
			parsed, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return cfg, fmt.Errorf("line %d: invalid bool", i+1)
			}
			cfg.Protect = parsed
		case "agent-binary":
			parsed, err := ParseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.AgentBinary = parsed
		case "cluster":
			value = strings.TrimSpace(value)
			if value != "" {
				parsed, err := ParseYAMLString(value)
				if err != nil {
					return cfg, fmt.Errorf("line %d: %w", i+1, err)
				}
				cfg.Cluster.Path = parsed
				continue
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
					return cfg, fmt.Errorf("invalid cluster line %d", i+1)
				}
				parsed, err := ParseYAMLString(childValue)
				if err != nil {
					return cfg, fmt.Errorf("line %d: %w", i+1, err)
				}
				switch childKey {
				case "path":
					cfg.Cluster.Path = parsed
				case "connection-cmd":
					cfg.Cluster.ConnectionCmd = parsed
				default:
					return cfg, fmt.Errorf("line %d: unknown cluster field %q", i+1, childKey)
				}
			}
		default:
			return cfg, fmt.Errorf("line %d: unknown field %q", i+1, key)
		}
	}

	return NormalizeProjectConfig(cfg), nil
}
