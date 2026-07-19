package ui

import tea "github.com/charmbracelet/bubbletea"

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(16, min(28, m.width-8))
		return m, nil
	case tea.KeyMsg:
		if m.mode == inputCreatingSession {
			return m, nil
		}
		if msg.Type == tea.KeyCtrlF || msg.Type == tea.KeyCtrlC {
			return m, m.closeMenuCmd()
		}
		if msg.Type == tea.KeyCtrlQ {
			return m.beginQuit()
		}
		if m.mode != inputNone {
			return m.updateModal(msg)
		}
		return m.updateNormal(msg)
	}

	return m.updateMessage(msg)
}
