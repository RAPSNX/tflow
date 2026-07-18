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
	lines := []string{sectionTitleStyle.Render("Sessions"), ""}
	sessions := m.contextSessions()
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
		content = currentBadgeStyle.Render("live") + " " + content
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

func (m model) renderDialogCard(badge, title, context, body, primary string, destructive bool) string {
	width := max(24, min(44, m.width-6))
	badgeStyle := dialogHeaderBadgeStyle
	primaryStyle := keycapStyle
	if destructive {
		badgeStyle = destructiveBadgeStyle
		primaryStyle = destructiveKeycapStyle
	}
	header := lipgloss.JoinHorizontal(lipgloss.Left, badgeStyle.Render(strings.ToUpper(badge)), " ", titleStyle.Render(title))
	divider := dialogDividerStyle.Render(strings.Repeat("─", max(16, width-8)))
	footer := lipgloss.JoinHorizontal(lipgloss.Left,
		primaryStyle.Render("Enter"), " ", mutedStyle.Render(primary), "   ",
		countBadgeStyle.Render("Esc"), " ", mutedStyle.Render("Cancel"),
	)
	lines := []string{header, divider, mutedStyle.Render(context)}
	if body != "" {
		lines = append(lines, "", body)
	}
	lines = append(lines, "", footer)
	box := overlayStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return m.renderDialog(box)
}

func (m model) renderInputField() string {
	return dialogInputStyle.Width(max(18, min(38, m.width-12))).Render(inputStyle.Render(m.input.View()))
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
	return m.renderDialogCard("switch", "Switch Project", "Search projects. Up and Down select a match.", body, "Switch", false)
}

func (m model) renderInputOverlay(title string) string {
	badge, primary := "create", "Create"
	if title == "Project Settings" {
		badge, primary = "settings", "Save"
	}
	context := "project: " + fallbackText(m.contextProject(), "none")
	return m.renderDialogCard(badge, title, context, m.renderInputField(), primary, false)
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
	return m.renderDialogCard("delete", "Confirm Delete", message, "", "Delete", true)
}

func (m model) renderProjectSwitchConfirmOverlay() string {
	context := "Switch from the current volatile session to project " + fallbackText(m.switchProjectTarget, "none") + "?"
	return m.renderDialogCard("confirm", "Confirm Project Switch", context, "", "Switch", false)
}

func (m model) renderQuitConfirmOverlay() string {
	return m.renderDialogCard("confirm", "Confirm Quit", "Remove this tflow instance volatile sessions and detach?", "", "Quit", false)
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
	return m.renderDialogCard("rename", title, "current: "+fallbackText(current, "none"), m.renderInputField(), "Save", false)
}
