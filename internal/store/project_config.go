package store

import "strings"

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
