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
	case inputNew, inputSwitchProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
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
	case inputRename:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderRenameOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	innerWidth := max(28, m.width-4)
	header := m.renderHeader(innerWidth)
	sessions := m.renderSessionPanel(innerWidth)
	footer := m.renderFooter(innerWidth)
	return lipgloss.JoinVertical(lipgloss.Left, header, sessions, footer)
}

func (m model) renderHeader(width int) string {
	left := brandBadgeStyle.Render("TFLOW")
	project := countBadgeStyle.Render("project " + fallbackText(m.currentProject(), ""))
	session := countBadgeStyle.Render("session " + fallbackText(m.sessionDisplayName(m.currentSession), ""))
	right := lipgloss.JoinHorizontal(lipgloss.Left, project, " ", session)
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return headerStyle.Width(width).Render(left + strings.Repeat(" ", gap) + right)
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
	parts := []string{}
	if sessionType := m.sessionType(s.Name); sessionType != sessionTypeTerminal {
		parts = append(parts, m.sessionTypeBadge(sessionType), " ")
	}
	parts = append(parts, m.sessionDisplayName(s.Name))
	content := strings.Join(parts, "")
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
	lines := []string{}
	switch m.mode {
	case inputNew:
		lines = append(lines, hintStyle.Render("new: [p] project  [t] terminal  [k] k9s  [c] agent  [esc] cancel"))
	case inputSwitchProject:
		lines = append(lines, inputStyle.Render(m.input.View()), hintStyle.Render("Enter switches. Esc cancels."))
		matches := m.matchingProjects(m.input.Value())
		if len(matches) == 0 {
			lines = append(lines, mutedStyle.Render("No matching projects."))
		} else {
			for _, project := range matches {
				lines = append(lines, sessionStyle.Render("  "+project))
			}
		}
	default:
		lines = append(lines, hintStyle.Render("[j/k] move  [enter] switch  [n] new  [p] project  [r] rename  [d] delete  [e] edit project"))
	}
	if status := m.statusView(); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}
	return footerStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
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
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderDeleteOverlay() string {
	target := "selection"
	switch {
	case m.deleteTarget.project != "":
		target = "project " + m.deleteTarget.project
	case m.deleteTarget.session != "":
		target = "session " + m.deleteTarget.session
	}
	lines := []string{
		titleStyle.Render("Confirm Delete"),
		mutedStyle.Render("Delete " + target + "?"),
		"",
		hintStyle.Render("Enter, d, or y confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(42, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderProjectSwitchConfirmOverlay() string {
	lines := []string{
		titleStyle.Render("Confirm Project Switch"),
		mutedStyle.Render("Switch from the current volatile session to project " + fallbackText(m.switchProjectTarget, "none") + "?"),
		"",
		hintStyle.Render("Enter or y confirms. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(48, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
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
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}
