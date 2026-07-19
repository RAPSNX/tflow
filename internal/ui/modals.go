package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case inputCreateSession:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Session creation cancelled."
			return m, nil
		case tea.KeyEnter:
			label := sanitizeSessionName(m.input.Value())
			if label == "" {
				m.status = "Session name is empty."
				return m, nil
			}
			project := m.contextProject()
			if m.hasSessionLabel(project, label, "") {
				m.status = "Session name already exists in this project."
				return m, nil
			}
			name := label
			if project != "" {
				name = projectSessionName(project, label)
			}
			if _, ok := m.findSession(name); ok {
				m.status = "Session name already exists."
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			dir := m.createSessionDir()
			return m, func() tea.Msg {
				s, err := m.tmux.CreateSession(name, dir, "")
				if err != nil {
					return sessionCreatedMsg{err: err}
				}
				if project == "" {
					if err := m.tmux.SetSessionTemporary(s.Name, true, m.instanceID); err != nil {
						_ = m.tmux.KillSession(s.Name)
						return sessionCreatedMsg{err: fmt.Errorf("mark volatile session: %w", err)}
					}
					return sessionCreatedMsg{session: s, volatile: true, label: label}
				}
				return sessionCreatedMsg{session: s, project: project, label: label}
			}
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputCreateProject:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Project creation cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitProjectCreate()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputSwitchProject:
		switch msg.Type {
		case tea.KeyDown:
			m.shiftProjectSwitch(1)
			return m, nil
		case tea.KeyUp:
			m.shiftProjectSwitch(-1)
			return m, nil
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.input.SetValue("")
			m.status = "Project switch cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitProjectSwitch()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		m.projectSwitchIndex = 0
		return m, cmd
	case inputConfirmDelete:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.deleteTarget = deleteTarget{}
			m.status = "Delete cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmDelete()
		}
	case inputConfirmProjectSwitch:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.switchProjectTarget = ""
			m.status = "Project switch cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmProjectSwitch()
		}
	case inputConfirmQuit:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.status = "Quit cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmQuit()
		}
	case inputEditProject:
		switch msg.Type {
		case tea.KeyEsc:
			return m.cancelProjectEdit()
		case tea.KeyEnter:
			return m.commitProjectEditField()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputRename:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Rename cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitRename()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	}
	return m, nil
}
