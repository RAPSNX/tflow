package ui

import tea "github.com/charmbracelet/bubbletea"

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, m.closeMenuCmd()
	case tea.KeyEsc:
		return m, m.closeMenuCmd()
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
	case "?":
		m.mode = inputHelp
		m.status = ""
		return m, nil
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
	}

	return m, nil
}
