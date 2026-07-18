package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func normalizeSessionTypes(raw map[string]string) map[string]sessionType {
	normalized := map[string]sessionType{}
	for name, value := range raw {
		normalized[name] = normalizeSessionType(value)
	}
	return normalized
}

func normalizeSessionType(value string) sessionType {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case string(sessionTypeK9s):
		return sessionTypeK9s
	case string(sessionTypeAgent):
		return sessionTypeAgent
	default:
		return sessionTypeTerminal
	}
}

func sessionTypeFromKind(kind sessionKind) sessionType {
	switch kind {
	case sessionKindK9s:
		return sessionTypeK9s
	case sessionKindAgent:
		return sessionTypeAgent
	default:
		return sessionTypeTerminal
	}
}

func (m model) sessionType(name string) sessionType {
	if m.sessionTypes == nil {
		return sessionTypeTerminal
	}
	if value, ok := m.sessionTypes[name]; ok {
		return value
	}
	return sessionTypeTerminal
}

func (m *model) setSessionType(name string, value sessionType) {
	if m.sessionTypes == nil {
		m.sessionTypes = map[string]sessionType{}
	}
	m.sessionTypes[name] = value
}

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
		if _, ok := m.sessionTypes[name]; ok {
			delete(m.sessionTypes, name)
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

func (m model) sessionTypeBadge(value sessionType) string {
	switch value {
	case sessionTypeK9s:
		return countBadgeStyle.Render("[k9s]")
	case sessionTypeAgent:
		return countBadgeStyle.Render("[agent]")
	default:
		return ""
	}
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

func (m *model) startSessionCreate(kind sessionKind) (tea.Model, tea.Cmd) {
	m.mode = inputCreateSession
	m.createSessionKind = kind
	m.input.Prompt = "session: "
	scope := fallbackText(m.contextProject(), "current directory")
	switch kind {
	case sessionKindK9s:
		m.input.SetValue("k9s")
		m.status = fmt.Sprintf("Create a new k9s session in %s.", scope)
	case sessionKindAgent:
		m.input.SetValue("agent")
		m.status = fmt.Sprintf("Create a new agent session in %s.", scope)
	default:
		m.input.SetValue("")
		m.status = fmt.Sprintf("Create a new terminal session in %s.", scope)
	}
	m.input.Focus()
	m.input.CursorEnd()
	return m, nil
}

func (m model) sessionStartupCommand(kind sessionKind) (string, error) {
	cfg := m.projectConfig(m.contextProject())
	switch kind {
	case sessionKindTerminal:
		return "", nil
	case sessionKindK9s:
		if cfg.Cluster.Path != "" {
			return "export KUBECONFIG=" + shellQuote(cfg.Cluster.Path) + "; exec k9s", nil
		}
		if cfg.Cluster.ConnectionCmd != "" {
			return cfg.Cluster.ConnectionCmd + " && exec k9s", nil
		}
		return "", fmt.Errorf("project %s has no cluster configured", cfg.Name)
	case sessionKindAgent:
		return "exec " + shellQuote(cfg.AgentBinaryValue()), nil
	default:
		return "", nil
	}
}
