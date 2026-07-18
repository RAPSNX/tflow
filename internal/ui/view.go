package ui

import (
	"charm.land/lipgloss/v2"
	"strings"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case inputHelp:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderHelp())
	case inputSwitchProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderProjectSwitchOverlay())
	case inputCreateSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Session"))
	case inputCreateProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Project"))
	case inputEditProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("Project Settings"))
	case inputConfirmDelete:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderDeleteOverlay())
	case inputConfirmProjectSwitch:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderProjectSwitchConfirmOverlay())
	case inputConfirmQuit:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderQuitConfirmOverlay())
	case inputRename:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderRenameOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	innerWidth := max(28, m.width-4)
	body := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(innerWidth), m.renderSessionPanel(innerWidth))
	footer := m.renderFooter(innerWidth)
	if footer == "" {
		return body
	}
	spacer := strings.Repeat("\n", max(0, m.height-lipgloss.Height(body)-lipgloss.Height(footer)))
	return lipgloss.JoinVertical(lipgloss.Left, body, spacer, footer)
}

func (m model) renderHeader(width int) string {
	return headerStyle.Width(width).Render(lipgloss.PlaceHorizontal(width, lipgloss.Center, brandBadgeStyle.Render("TFLOW")))
}

func (m model) renderSessionPanel(width int) string {
	lines := []string{
		sectionTitleStyle.Render("Sessions"),
	}

	sessions := m.contextSessions()
	lines = append(lines, "")
	if len(sessions) == 0 {
		lines = append(lines, mutedStyle.Render("No sessions in this context"))
	} else {
		for index, session := range sessions {
			lines = append(lines, m.renderSessionRow(index, session))
		}
	}

	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderSessionRow(index int, s session) string {
	content := m.sessionLabel(s.Name)
	if s.Name == m.currentSession {
		badge := "[live]"
		if index != m.selectedSessionIndex() {
			badge = currentBadgeStyle.Render(badge)
		}
		content = badge + " " + content
	}
	project := normalizeProjectName(m.sessionProjects[s.Name])
	style := m.rowStyle(index == m.selectedSessionIndex(), project)
	return style.Width(max(16, m.width-12)).Render(content)
}

func (m model) renderFooter(width int) string {
	status := m.statusView()
	if status == "" {
		return ""
	}
	return footerStyle.Width(width).Render(status)
}

func (m model) renderHelp() string {
	rows := []string{
		titleStyle.Render("Shortcuts"),
		"Ctrl+F  Toggle sidebar",
		"Ctrl+Q  Quit tflow",
		"Ctrl+C  Close sidebar",
		"Esc     Return to sessions",
		"?       Show shortcuts",
		"j       Select next session",
		"k       Select previous session",
		"Enter   Switch to session",
		"n       Create session",
		"N       Create project",
		"p       Switch project",
		"r       Rename session",
		"R       Rename project",
		"d       Delete session",
		"D       Delete project",
		"e       Edit project settings",
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) renderDialog(box string) string {
	status := m.statusView()
	height := m.height
	if status != "" {
		height--
	}
	dialog := lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, box)
	if status == "" {
		return dialog
	}
	return lipgloss.JoinVertical(lipgloss.Left, dialog, footerStyle.Width(m.width).Render(status))
}

func (m model) renderProjectSwitchOverlay() string {
	lines := []string{
		titleStyle.Render("Switch Project"),
		"",
		inputStyle.Render(m.input.View()),
		"",
	}
	matches := m.matchingProjects(m.input.Value())
	if len(matches) == 0 {
		lines = append(lines, mutedStyle.Render("No matching projects."))
	} else {
		for index, project := range matches {
			style := sessionStyle
			if index == m.projectSwitchIndex {
				style = selectedSessionStyle
			}
			lines = append(lines, style.Render(project))
		}
	}
	lines = append(lines, "", hintStyle.Render("j/k selects. Enter switches. Esc cancels."))
	box := overlayStyle.Width(max(24, min(42, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderInputOverlay(title string) string {
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("project: " + fallbackText(m.contextProject(), "none")),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderDeleteOverlay() string {
	target := "selection"
	switch {
	case m.deleteTarget.project != "":
		target = "project " + m.deleteTarget.project
	case m.deleteTarget.session != "":
		target = "session " + m.deleteTarget.session
	}
	message := "Delete " + target + "?"
	if m.deleteTarget.session != "" {
		project := normalizeProjectName(m.sessionProjects[m.deleteTarget.session])
		if project != "" && len(m.projectSessions(project)) == 1 {
			message = "This will delete the whole project " + project + "."
		}
	}
	lines := []string{
		titleStyle.Render("Confirm Delete"),
		mutedStyle.Render(message),
		"",
		hintStyle.Render("Enter confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(42, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderProjectSwitchConfirmOverlay() string {
	lines := []string{
		titleStyle.Render("Confirm Project Switch"),
		mutedStyle.Render("Switch from the current volatile session to project " + fallbackText(m.switchProjectTarget, "none") + "?"),
		"",
		hintStyle.Render("Enter confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(48, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderQuitConfirmOverlay() string {
	lines := []string{
		titleStyle.Render("Confirm Quit"),
		mutedStyle.Render("Remove this instance's volatile sessions and detach?"),
		"",
		hintStyle.Render("Enter confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(48, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderRenameOverlay() string {
	title := "Rename"
	current := ""
	switch {
	case m.renameTarget.project != "":
		title = "Rename Project"
		current = m.renameTarget.project
	case m.renameTarget.session != "":
		title = "Rename Session"
		current = m.renameTarget.session
	}
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("current: " + fallbackText(current, "none")),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}
