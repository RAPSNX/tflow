package store

import (
	"bytes"
	"fmt"
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

func LoadProjectConfigs(statePath string, state AppState) (map[string]ProjectConfig, error) {
	_ = statePath
	state = NormalizeAppState(state)

	configs := make(map[string]ProjectConfig, len(state.Projects))
	for _, project := range state.Projects {
		cfg := NormalizeProjectConfig(state.ProjectConfigs[project])
		cfg.Name = project
		configs[project] = cfg
	}
	return configs, nil
}

func SaveProjectConfigs(statePath string, configs map[string]ProjectConfig) error {
	_ = statePath
	_ = configs
	return nil
}

func RemoveProjectConfigFile(statePath, project string) error {
	_ = statePath
	_ = project
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
