package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	m.projectEditConfig.Name = project
	m.input.Prompt = "workdir: "
	m.input.SetValue(m.projectEditConfig.Workdir)
	m.input.Focus()
	m.input.CursorEnd()
	m.status = "Edit the project workdir. Leave empty to use the current directory."
	return m, nil
}

func (m *model) cancelProjectEdit() (tea.Model, tea.Cmd) {
	m.mode = inputNone
	m.projectEditConfig = projectConfig{}
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	m.err = nil
	m.status = "Project settings cancelled."
	return m, nil
}

func (m *model) commitProjectEditField() (tea.Model, tea.Cmd) {
	m.projectEditConfig.Workdir = strings.TrimSpace(m.input.Value())
	m.setProjectConfig(m.projectEditConfig)
	m.mode = inputNone
	m.projectEditConfig = projectConfig{}
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
