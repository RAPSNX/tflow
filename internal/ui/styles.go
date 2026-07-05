package ui

import "charm.land/lipgloss/v2"

var (
	appStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Foreground(lipgloss.Color("#E7E5E4"))

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAF9"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8A29E"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8A29E"))

	sidebarStyle = lipgloss.NewStyle().
			Padding(0, 0)

	selectedItemStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAF9"))

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D6D3D1"))

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A8A29E"))

	errorStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FCA5A5"))

	inputStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAF9"))

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("#57534E")).
			Foreground(lipgloss.Color("#E7E5E4")).
			Padding(1, 2)
)
