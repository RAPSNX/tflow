package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) sessionLabel(name string) string {
	if m.sessionLabels != nil {
		if label := strings.TrimSpace(m.sessionLabels[name]); label != "" {
			return label
		}
	}
	return name
}

func (m *model) setSessionLabel(name, label string) {
	if m.sessionLabels == nil {
		m.sessionLabels = map[string]string{}
	}
	m.sessionLabels[name] = sanitizeSessionName(label)
}

func (m *model) clearSessionMetadata(names ...string) bool {
	changed := false
	for _, name := range names {
		if _, ok := m.sessionProjects[name]; ok {
			delete(m.sessionProjects, name)
			changed = true
		}
		if _, ok := m.sessionLabels[name]; ok {
			delete(m.sessionLabels, name)
			changed = true
		}
	}
	return changed
}

func (m model) hasSessionLabel(project, label string, exceptName string) bool {
	project = normalizeProjectName(project)
	label = sanitizeSessionName(label)
	for _, s := range m.sessions {
		if s.Name == exceptName || s.Temporary {
			continue
		}
		if normalizeProjectName(m.sessionProjects[s.Name]) == project && m.sessionLabel(s.Name) == label {
			return true
		}
	}
	return false
}

func (m model) projectConfig(project string) projectConfig {
	project = normalizeProjectName(project)
	if cfg, ok := m.projectConfigs[project]; ok {
		return normalizeProjectConfig(cfg)
	}
	return projectConfig{Name: project}
}

func (m *model) setProjectConfig(cfg projectConfig) {
	cfg = normalizeProjectConfig(cfg)
	if cfg.Name == "" {
		return
	}
	if m.projectConfigs == nil {
		m.projectConfigs = map[string]projectConfig{}
	}
	m.projectConfigs[cfg.Name] = cfg
}

func (m model) projectDir(project string) string {
	return strings.TrimSpace(m.projectConfig(project).Workdir)
}

func (m model) createSessionDir() string {
	if dir := m.projectDir(m.contextProject()); dir != "" {
		return dir
	}
	return m.cwd
}

func (m *model) startSessionCreate() (tea.Model, tea.Cmd) {
	m.mode = inputCreateSession
	m.input.Prompt = "session: "
	m.input.SetValue("")
	m.input.Focus()
	m.status = fmt.Sprintf("Create a new terminal session in %s.", fallbackText(m.contextProject(), "current directory"))
	return m, nil
}
