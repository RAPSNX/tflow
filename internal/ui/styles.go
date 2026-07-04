package ui

import "charm.land/lipgloss/v2"

const appPadding = 4

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Background(lipgloss.Color("#0B0F14")).
			Foreground(lipgloss.Color("#E5EEF7"))

	headerStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, true, false).
			BorderForeground(lipgloss.Color("#2A3442")).
			PaddingBottom(1)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F4B942"))

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7F8EA3"))

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#90A0B6"))

	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2A3442")).
			Background(lipgloss.Color("#111823")).
			Padding(0, 1)

	mainStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2A3442")).
			Background(lipgloss.Color("#0F1520")).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#111827")).
				Background(lipgloss.Color("#F4B942"))

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5EEF7"))

	statusStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("#2A3442")).
			Foreground(lipgloss.Color("#C7D2DF"))

	errorStatusStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder(), true, false, false, false).
				BorderForeground(lipgloss.Color("#7F1D1D")).
				Foreground(lipgloss.Color("#FCA5A5"))

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#2A3442")).
			Background(lipgloss.Color("#131A23")).
			Foreground(lipgloss.Color("#F8FAFC")).
			Padding(0, 1)

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F4B942")).
			Background(lipgloss.Color("#111823")).
			Foreground(lipgloss.Color("#E5EEF7")).
			Padding(1, 2)
)
