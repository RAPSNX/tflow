package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	switch m.mode {
	case inputNew:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	case inputCreateSession:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Session"))
	case inputCreateProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderInputOverlay("New Project"))
	case inputMoveProject:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	case inputSetProjectDir:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderProjectDirOverlay())
	case inputConfirmDelete:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderDeleteOverlay())
	case inputRename:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderRenameOverlay())
	default:
		return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
	}
}

func (m model) renderMenu() string {
	innerWidth := max(28, m.width-4)
	header := m.renderHeader(innerWidth)
	tree := m.renderTreePanel(innerWidth)
	footer := m.renderFooter(innerWidth)
	return lipgloss.JoinVertical(lipgloss.Left, header, tree, footer)
}

func (m model) renderRow(index int, row treeRow) string {
	selected := index == m.selectedRowIndex()
	switch row.kind {
	case rowProject:
		return m.renderProjectRow(row, selected)
	case rowSession:
		return m.renderSessionRow(row, selected)
	default:
		return ""
	}
}

func (m model) rowLabel(row treeRow) string {
	switch row.kind {
	case rowProject:
		prefix := "v"
		if !m.expandedProjects[row.project] {
			prefix = ">"
		}
		return fmt.Sprintf("%s %s", prefix, row.project)
	case rowSession:
		marker := "  "
		if row.session == m.currentSession {
			marker = "* "
		}
		return strings.Repeat(" ", row.depth*2) + marker + row.session
	default:
		return ""
	}
}

func (m model) renderHeader(width int) string {
	left := brandBadgeStyle.Render("TFLOW")
	right := mutedStyle.Render(fmt.Sprintf("%d projects  %d sessions", len(m.projects), m.visibleSessionCount()))
	gap := max(1, width-lipgloss.Width(left)-lipgloss.Width(right))
	return headerStyle.Width(width).Render(left + strings.Repeat(" ", gap) + right)
}

func (m model) renderTreePanel(width int) string {
	lines := []string{
		sectionTitleStyle.Render("Projects"),
	}

	rows := m.treeRows()
	if len(rows) == 0 {
		lines = append(lines, "", mutedStyle.Render("No projects or sessions"))
	} else {
		lines = append(lines, "")
		for index, row := range rows {
			lines = append(lines, m.renderRow(index, row))
		}
	}

	return panelStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderProjectRow(row treeRow, selected bool) string {
	toggle := "[-]"
	if !m.expandedProjects[row.project] {
		toggle = "[+]"
	}
	return m.renderLabeledRow(fmt.Sprintf("%s %s", toggle, m.projectLabel(row.project)), "", selected, true, row.project)
}

func (m model) renderSessionRow(row treeRow, selected bool) string {
	parts := []string{"  "}
	if sessionType := m.sessionType(row.session); sessionType != sessionTypeTerminal {
		parts = append(parts, m.sessionTypeBadge(sessionType), " ")
	}
	parts = append(parts, row.session)
	content := strings.Join(parts, "")
	if row.session == m.currentSession {
		badge := "[live]"
		if !selected {
			badge = currentBadgeStyle.Render(badge)
		}
		content = "  " + badge + " " + strings.TrimLeft(content, " ")
	}
	return m.renderInlineRow(content, selected, false, row.project)
}

func (m model) renderLabeledRow(left, right string, selected, project bool, projectName string) string {
	style := m.rowStyle(selected, project, projectName)

	contentWidth := max(16, m.width-12)
	gap := contentWidth - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap)
	if right != "" {
		content += right
	}
	return style.Width(contentWidth).Render(content)
}

func (m model) renderInlineRow(content string, selected, project bool, projectName string) string {
	style := m.rowStyle(selected, project, projectName)
	return style.Width(max(16, m.width-12)).Render(content)
}

func (m model) renderFooter(width int) string {
	lines := []string{}
	if m.mode == inputNew {
		lines = append(lines, hintStyle.Render("new: [p] project  [t] terminal  [k] k9s  [c] agent  [a] add temp  [esc] cancel"))
	}
	if m.mode == inputMoveProject {
		lines = append(lines, hintStyle.Render("move: type a project prefix and the matching initials stay highlighted"))
	}
	if m.mode == inputCommand {
		lines = append(lines, inputStyle.Render(m.input.View()), hintStyle.Render("Enter runs. Esc cancels."))
	}
	if status := m.statusView(); status != "" {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, status)
	}
	if len(lines) == 0 {
		return ""
	}
	return footerStyle.Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m model) renderInputOverlay(title string) string {
	lines := []string{
		titleStyle.Render(title),
		mutedStyle.Render("project: " + m.contextProject()),
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
	switch m.deleteRow.kind {
	case rowProject:
		target = "project " + m.deleteRow.project
	case rowSession:
		target = "session " + m.deleteRow.session
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

func (m model) renderMoveOverlay() string {
	lines := []string{
		titleStyle.Render("Move Session"),
		mutedStyle.Render("prefix: " + fallbackText(m.moveQuery, "type letters")),
		"",
	}
	for _, project := range m.matchingProjects(m.moveQuery) {
		lines = append(lines, sessionStyle.Render("  "+project))
	}
	if len(m.matchingProjects(m.moveQuery)) == 0 {
		lines = append(lines, mutedStyle.Render("No matching project."))
	}
	lines = append(lines, "", hintStyle.Render("Type until one project matches. Enter confirms."))
	box := overlayStyle.Width(max(24, min(36, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderProjectDirOverlay() string {
	lines := []string{
		titleStyle.Render("Project Directory"),
		mutedStyle.Render("project: " + m.contextProject()),
		"",
		inputStyle.Render(m.input.View()),
		"",
		hintStyle.Render("Enter saves. Esc cancels."),
	}
	box := overlayStyle.Width(max(24, min(44, m.width-6))).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func (m model) renderRenameOverlay() string {
	title := "Rename"
	current := ""
	switch m.renameRow.kind {
	case rowProject:
		title = "Rename Project"
		current = m.renameRow.project
	case rowSession:
		title = "Rename Section"
		current = m.renameRow.session
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
