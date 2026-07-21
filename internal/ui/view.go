package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
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
	case inputMoveSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderSessionMoveOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	innerWidth := max(28, m.width-4)
	sections := []string{m.renderHeader(innerWidth), m.renderSessionPanel(innerWidth)}
	if m.showHelp {
		sections = append(sections, "", m.renderHelp())
	}
	body := lipgloss.JoinVertical(lipgloss.Left, sections...)
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
	lines := []string{sectionTitleStyle.Render("Sessions"), ""}
	sessions := m.contextSessions()
	selectedIndex := sessionIndex(sessions, m.selectedSession)
	if len(sessions) == 0 {
		lines = append(lines, mutedStyle.Render("No sessions in this context"))
	} else {
		for index, session := range sessions {
			lines = append(lines, m.renderSessionRow(index, selectedIndex, session))
		}
	}
	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderSessionRow(index, selectedIndex int, s session) string {
	label := m.sessionLabel(s.Name)
	project := normalizeProjectName(m.sessionProjects[s.Name])
	selected := index == selectedIndex
	style := m.rowStyle(selected, project)
	content := label
	if s.Name == m.currentSession {
		content = currentBadgeStyle.Render("live") + " " + label
		if selected {
			// The live badge resets terminal styles after rendering. Reapply the
			// selected row style so the label stays highlighted beside the badge.
			content = currentBadgeStyle.Render("live") + selectedSessionStyle.Padding(0).Render(" "+label)
		}
	}
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
		"m       Move session to project",
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

func (m model) dialogCardWidth() int {
	return min(44, max(1, m.width-4))
}

func (m model) dialogContentWidth() int {
	return max(1, m.dialogCardWidth()-overlayStyle.GetHorizontalFrameSize())
}

func (m model) renderDialogCard(badge, title, context, body string, destructive bool) string {
	width := m.dialogCardWidth()
	contentWidth := m.dialogContentWidth()
	badgeStyle := dialogHeaderBadgeStyle
	primaryStyle := keycapStyle
	if destructive {
		badgeStyle = destructiveBadgeStyle
		primaryStyle = destructiveKeycapStyle
	}
	header := lipgloss.JoinHorizontal(lipgloss.Left, badgeStyle.Render(strings.ToUpper(badge)), " ", titleStyle.Render(title))
	divider := dialogDividerStyle.Render(strings.Repeat("─", contentWidth))
	footer := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center,
		lipgloss.JoinHorizontal(lipgloss.Left, primaryStyle.Render("Enter"), " ", countBadgeStyle.Render("Esc")),
	)
	lines := []string{header, divider}
	if context != "" {
		lines = append(lines, mutedStyle.Render(context))
	}
	if body != "" {
		if context != "" {
			lines = append(lines, "")
		}
		lines = append(lines, body)
	}
	lines = append(lines, "", footer)
	box := overlayStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderInputField() string {
	width := min(38, m.dialogContentWidth())
	return dialogInputStyle.Width(width).Render(inputStyle.Render(m.input.View()))
}

func (m model) renderProjectSwitchOverlay() string {
	matches := m.matchingProjects(m.input.Value())
	list := []string{}
	if len(matches) == 0 {
		list = append(list, mutedStyle.Render("No matching projects."))
	} else {
		for index, project := range matches {
			style := sessionStyle
			if index == m.projectSwitchIndex {
				style = selectedSessionStyle
			}
			list = append(list, style.Render(project))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, m.renderInputField(), "", lipgloss.JoinVertical(lipgloss.Left, list...))
	return m.renderDialogCard("switch", "Project", "Choose a project.", body, false)
}

func (m model) renderInputOverlay(title string) string {
	badge := "create"
	switch title {
	case "New Session":
		title = "Session"
	case "New Project":
		title = "Project"
	case "Project Settings":
		badge, title = "settings", "Project"
	}
	return m.renderDialogCard(badge, title, "", m.renderInputField(), false)
}

func (m model) renderDeleteOverlay() string {
	title, message := "Selection", "Delete selection?"
	switch {
	case m.deleteTarget.project != "":
		title = "Project"
		message = "Delete project " + m.deleteTarget.project + " and all its sessions?"
	case m.deleteTarget.session != "":
		title = "Session"
		message = "Delete " + m.sessionLabel(m.deleteTarget.session) + "?"
	}
	if m.deleteTarget.session != "" {
		project := normalizeProjectName(m.sessionProjects[m.deleteTarget.session])
		if project != "" && len(m.projectSessions(project)) == 1 {
			title = "Project"
			message = "Delete project " + project + " and its session?"
		}
	}
	return m.renderDialogCard("delete", title, message, "", true)
}

func (m model) renderProjectSwitchConfirmOverlay() string {
	context := "Switch to " + fallbackText(m.switchProjectTarget, "none") + "?"
	return m.renderDialogCard("confirm", "Project Switch", context, "", false)
}

func (m model) renderQuitConfirmOverlay() string {
	return m.renderDialogCard("confirm", "Quit", "Remove this instance’s volatile sessions and quit?", "", false)
}

func (m model) renderSessionMoveOverlay() string {
	matches := m.matchingMoveProjects(m.input.Value())
	list := []string{}
	if len(matches) == 0 {
		list = append(list, mutedStyle.Render("No matching projects."))
	} else {
		for index, project := range matches {
			style := sessionStyle
			if index == m.moveProjectIndex {
				style = selectedSessionStyle
			}
			list = append(list, style.Render(project))
		}
	}
	body := lipgloss.JoinVertical(lipgloss.Left, m.renderInputField(), "", lipgloss.JoinVertical(lipgloss.Left, list...))
	context := "Move " + fallbackText(m.sessionLabel(m.moveTarget.session), "session") + " to…"
	return m.renderDialogCard("move", "Session", context, body, false)
}

func (m model) renderRenameOverlay() string {
	title := "Session"
	current := ""
	switch {
	case m.renameTarget.project != "":
		title = "Project"
		current = m.renameTarget.project
	case m.renameTarget.session != "":
		current = m.sessionLabel(m.renameTarget.session)
	}
	return m.renderDialogCard("rename", title, "Current: "+fallbackText(current, "none"), m.renderInputField(), false)
}
