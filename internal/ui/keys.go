package ui

import tea "github.com/charmbracelet/bubbletea"

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, m.closeMenuCmd()
	case tea.KeyEsc:
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		return m, m.closeMenuCmd()
	}

	if msg.String() == "?" {
		m.showHelp = !m.showHelp
		return m, nil
	}
	if m.showHelp {
		m.showHelp = false
	}

	switch msg.String() {
	case "j":
		m.shiftSession(1)
		return m, nil
	case "k":
		m.shiftSession(-1)
		return m, nil
	case "enter":
		return m.switchSelectedSession()
	case "n":
		return m.startSessionCreate()
	case "N":
		return m.beginProjectCreate()
	case "p":
		return m.beginProjectSwitch()
	case "d":
		return m.beginDelete()
	case "D":
		return m.beginProjectDelete()
	case "r":
		return m.beginRename()
	case "R":
		return m.beginProjectRename()
	case "e":
		return m.editProject()
	case "m":
		return m.beginSessionMove()
	}

	return m, nil
}
