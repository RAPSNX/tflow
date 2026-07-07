package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type clusterConfig struct {
	Path          string
	ConnectionCmd string
}

type projectConfig struct {
	Name        string
	Workdir     string
	Cluster     clusterConfig
	AgentBinary string
	Protect     bool
}

func normalizeProjectConfig(cfg projectConfig) projectConfig {
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

func (cfg projectConfig) isZero() bool {
	return cfg.Name == "" && cfg.Workdir == "" && cfg.Cluster.Path == "" && cfg.Cluster.ConnectionCmd == "" && cfg.AgentBinary == "" && !cfg.Protect
}

func (cfg projectConfig) clusterConfigured() bool {
	return cfg.Cluster.Path != "" || cfg.Cluster.ConnectionCmd != ""
}

func (cfg projectConfig) agentBinary() string {
	return strings.TrimSpace(cfg.AgentBinary)
}

func loadProjectConfigs(statePath string, state appState) (map[string]projectConfig, error) {
	cfg, err := loadAppConfigForStatePath(statePath)
	if err != nil {
		return nil, err
	}
	configs := map[string]projectConfig{}
	for _, project := range cfg.Projects {
		project = normalizeProjectConfig(project)
		if project.Name == "" {
			continue
		}
		configs[project.Name] = project
	}
	return configs, nil
}

func saveProjectConfigs(statePath string, configs map[string]projectConfig) error {
	baseDir := filepath.Dir(statePath)
	cfg, err := loadAppConfigForStatePath(statePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		cfg = defaultAppConfig(baseDir)
	}

	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)

	cfg.Projects = cfg.Projects[:0]
	for _, name := range names {
		project := normalizeProjectConfig(configs[name])
		if project.Name == "" {
			continue
		}
		cfg.Projects = append(cfg.Projects, project)
	}
	return saveAppConfigForDir(baseDir, cfg)
}

func removeProjectConfigFile(statePath, project string) error {
	configs, err := loadProjectConfigs(statePath, appState{})
	if err != nil {
		return err
	}
	delete(configs, normalizeProjectName(project))
	return saveProjectConfigs(statePath, configs)
}

func marshalProjectConfig(cfg projectConfig) []byte {
	cfg = normalizeProjectConfig(cfg)

	var buf bytes.Buffer
	buf.WriteString("name: ")
	buf.WriteString(yamlString(cfg.Name))
	buf.WriteByte('\n')
	if cfg.Workdir != "" {
		buf.WriteString("workdir: ")
		buf.WriteString(yamlString(cfg.Workdir))
		buf.WriteByte('\n')
	}
	if cfg.Protect {
		buf.WriteString("protect: true\n")
	}
	if cfg.AgentBinary != "" {
		buf.WriteString("agent-cmd: ")
		buf.WriteString(yamlString(cfg.AgentBinary))
		buf.WriteByte('\n')
	}
	if cfg.Cluster.Path != "" || cfg.Cluster.ConnectionCmd != "" {
		buf.WriteString("cluster:\n")
		if cfg.Cluster.Path != "" {
			buf.WriteString("  path: ")
			buf.WriteString(yamlString(cfg.Cluster.Path))
			buf.WriteByte('\n')
		}
		if cfg.Cluster.ConnectionCmd != "" {
			buf.WriteString("  connection-cmd: ")
			buf.WriteString(yamlString(cfg.Cluster.ConnectionCmd))
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func parseProjectConfig(data []byte) (projectConfig, error) {
	var cfg projectConfig

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
		case "name":
			parsed, err := parseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.Name = parsed
		case "workdir":
			parsed, err := parseYAMLString(value)
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
		case "agent-cmd", "agent-binary":
			parsed, err := parseYAMLString(value)
			if err != nil {
				return cfg, fmt.Errorf("line %d: %w", i+1, err)
			}
			cfg.AgentBinary = parsed
		case "cluster":
			value = strings.TrimSpace(value)
			if value != "" {
				parsed, err := parseYAMLString(value)
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
				parsed, err := parseYAMLString(childValue)
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

	return normalizeProjectConfig(cfg), nil
}

func yamlString(value string) string {
	return strconv.Quote(value)
}

func parseYAMLString(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, "\"") {
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string")
		}
		return parsed, nil
	}
	if strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'") && len(value) >= 2 {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'"), nil
	}
	return value, nil
}

func splitYAMLField(line string) (string, string, bool) {
	index := strings.IndexByte(line, ':')
	if index < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:index]), line[index+1:], true
}

func shouldSkipYAMLLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	return trimmed == "" || strings.HasPrefix(trimmed, "#")
}
