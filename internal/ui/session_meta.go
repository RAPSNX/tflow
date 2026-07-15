package ui

import (
	"fmt"
	"os"
	"os/exec"
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

func (m model) editProject() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	if project == "" {
		m.err = fmt.Errorf("no project selected")
		m.status = "No project selected."
		return m, nil
	}
	cfg := m.projectConfig(project)
	path, err := writeProjectEditFile(cfg)
	if err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	cmd, err := projectEditorCommand(path)
	if err != nil {
		_ = os.Remove(path)
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		defer os.Remove(path)
		if err != nil {
			return projectEditedMsg{oldName: project, err: err}
		}
		edited, readErr := os.ReadFile(path)
		if readErr != nil {
			return projectEditedMsg{oldName: project, err: readErr}
		}
		parsed, parseErr := parseProjectConfig(edited)
		if parseErr != nil {
			return projectEditedMsg{oldName: project, err: parseErr}
		}
		return projectEditedMsg{oldName: project, config: parsed}
	})
}

func writeProjectEditFile(cfg projectConfig) (string, error) {
	file, err := os.CreateTemp("", "tflow-project-*.yaml")
	if err != nil {
		return "", err
	}
	path := file.Name()
	if _, err := file.Write(marshalProjectConfig(cfg)); err != nil {
		file.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func projectEditorCommand(path string) (*exec.Cmd, error) {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	args := strings.Fields(editor)
	if len(args) == 0 {
		return nil, fmt.Errorf("EDITOR is empty")
	}
	args = append(args, path)
	return exec.Command(args[0], args[1:]...), nil
}

func (m model) applyProjectEdit(oldName string, cfg projectConfig) (tea.Model, tea.Cmd) {
	cfg = normalizeProjectConfig(cfg)
	if cfg.Name == "" {
		m.err = fmt.Errorf("project name is empty")
		m.status = "Project name is empty."
		return m, nil
	}
	if oldName != cfg.Name && containsString(m.projects, cfg.Name) {
		m.err = fmt.Errorf("project already exists")
		m.status = "Project already exists."
		return m, nil
	}
	if oldName != cfg.Name {
		for name, project := range m.sessionProjects {
			if normalizeProjectName(project) == oldName {
				m.sessionProjects[name] = cfg.Name
			}
		}
		m.projects = replaceProject(m.projects, oldName, cfg.Name)
		delete(m.projectConfigs, oldName)
		if m.selectedProject == oldName {
			m.selectedProject = cfg.Name
		}
		if err := removeProjectConfigFile(m.statePath, oldName); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
	}
	m.setProjectConfig(cfg)
	if err := m.saveState(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	if err := m.syncTmuxSessionProjects(); err != nil {
		m.err = err
		m.status = err.Error()
		return m, nil
	}
	m.err = nil
	m.status = ""
	return m, nil
}

func (m model) executeCommand(command string) (tea.Model, tea.Cmd) {
	switch command {
	case "", ":":
		m.err = nil
		m.status = ""
		return m, nil
	case "q", "q!":
		return m, m.closeMenuCmd()
	case "qa", "qa!":
		return m, m.quitAllCmd()
	default:
		m.err = fmt.Errorf("unknown command")
		m.status = fmt.Sprintf("Unknown command: %s", command)
		return m, nil
	}
}
