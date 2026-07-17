package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type projectEditField int

const (
	projectEditWorkdir projectEditField = iota
	projectEditProtect
	projectEditAgentBinary
	projectEditClusterPath
	projectEditClusterConnection
)

func (m model) editProject() (tea.Model, tea.Cmd) {
	project := m.contextProject()
	if project == "" {
		m.err = fmt.Errorf("no project selected")
		m.status = "No project selected."
		return m, nil
	}

	m.mode = inputEditProject
	m.projectEditConfig = m.projectConfig(project)
	m.projectEditField = projectEditWorkdir
	m.projectEditConfig.Name = project
	m.input.Focus()
	m.loadProjectEditField()
	return m, nil
}

func (m *model) loadProjectEditField() {
	switch m.projectEditField {
	case projectEditWorkdir:
		m.input.Prompt = "workdir: "
		m.input.SetValue(m.projectEditConfig.Workdir)
		m.status = "Edit the project workdir. Leave empty to use the current directory."
	case projectEditProtect:
		m.input.Prompt = "protect: "
		m.input.SetValue(strconv.FormatBool(m.projectEditConfig.Protect))
		m.status = "Set project protection to true or false."
	case projectEditAgentBinary:
		m.input.Prompt = "agent: "
		m.input.SetValue(m.projectEditConfig.AgentBinary)
		m.status = "Set the agent binary. Leave empty to use codex."
	case projectEditClusterPath:
		m.input.Prompt = "cluster path: "
		m.input.SetValue(m.projectEditConfig.Cluster.Path)
		m.status = "Set the cluster kubeconfig path. Leave empty to skip."
	case projectEditClusterConnection:
		m.input.Prompt = "cluster cmd: "
		m.input.SetValue(m.projectEditConfig.Cluster.ConnectionCmd)
		m.status = "Set the cluster connection command. Leave empty to skip."
	}
	m.input.CursorEnd()
}

func (m *model) cancelProjectEdit() (tea.Model, tea.Cmd) {
	m.mode = inputNone
	m.projectEditConfig = projectConfig{}
	m.projectEditField = projectEditWorkdir
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	m.err = nil
	m.status = "Project settings cancelled."
	return m, nil
}

func (m *model) commitProjectEditField() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())

	switch m.projectEditField {
	case projectEditWorkdir:
		m.projectEditConfig.Workdir = value
	case projectEditProtect:
		if value == "" {
			m.projectEditConfig.Protect = false
			break
		}
		parsed, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			m.err = fmt.Errorf("protect must be true or false")
			m.status = "Protect must be true or false."
			return m, nil
		}
		m.projectEditConfig.Protect = parsed
	case projectEditAgentBinary:
		m.projectEditConfig.AgentBinary = value
	case projectEditClusterPath:
		m.projectEditConfig.Cluster.Path = value
	case projectEditClusterConnection:
		m.projectEditConfig.Cluster.ConnectionCmd = value
		cfg := normalizeProjectConfig(m.projectEditConfig)
		m.setProjectConfig(cfg)
		m.mode = inputNone
		m.projectEditConfig = projectConfig{}
		m.projectEditField = projectEditWorkdir
		m.input.Blur()
		m.input.Prompt = ""
		m.input.SetValue("")
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

	m.projectEditField++
	m.err = nil
	m.loadProjectEditField()
	return m, nil
}
