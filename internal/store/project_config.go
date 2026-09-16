package store

import "strings"

type ProjectConfig struct {
	Name        string
	Workdir     string
	AgentBinary string
}

func NormalizeProjectConfig(cfg ProjectConfig) ProjectConfig {
	cfg.Name = NormalizeProjectName(cfg.Name)
	cfg.Workdir = strings.TrimSpace(cfg.Workdir)
	if cfg.Workdir != "" {
		cfg.Workdir = NormalizeCWD(cfg.Workdir)
	}
	cfg.AgentBinary = strings.TrimSpace(cfg.AgentBinary)
	return cfg
}
