package ui

import "charm.land/lipgloss/v2"

var (
	baseBG            = lipgloss.Color("#1E1E2E")
	surface0          = lipgloss.Color("#313244")
	surface1          = lipgloss.Color("#45475A")
	textColor         = lipgloss.Color("#CDD6F4")
	subtextColor      = lipgloss.Color("#A6ADC8")
	blueColor         = lipgloss.Color("#89B4FA")
	tealColor         = lipgloss.Color("#94E2D5")
	yellowColor       = lipgloss.Color("#F9E2AF")
	redColor          = lipgloss.Color("#F38BA8")
	mantleColor       = lipgloss.Color("#181825")
	crustColor        = lipgloss.Color("#11111B")
	badgeTextColor    = lipgloss.Color("#1E1E2E")
	selectedTextColor = lipgloss.Color("#11111B")

	appStyle = lipgloss.NewStyle().
			Padding(1).
			Foreground(textColor).
			Background(baseBG)

	headerStyle = lipgloss.NewStyle().
			Background(baseBG).
			Padding(0, 0, 1, 0)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	brandBadgeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(badgeTextColor).
			Background(blueColor).
			Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	hintStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface1).
			Background(mantleColor).
			Padding(1)

	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(textColor)

	statsValueStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor)

	statsLabelStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	projectStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Padding(0, 1)

	selectedProjectStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(selectedTextColor).
				Background(blueColor).
				Padding(0, 1)

	sessionStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Padding(0, 1)

	selectedSessionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(selectedTextColor).
				Background(tealColor).
				Padding(0, 1)

	currentBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(yellowColor).
				Padding(0, 0)

	countBadgeStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(surface0).
			Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
			Background(baseBG).
			Padding(0, 1, 0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(subtextColor)

	errorStatusStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(redColor)

	inputStyle = lipgloss.NewStyle().
			Foreground(textColor)

	overlayStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(surface1).
			Background(crustColor).
			Foreground(textColor).
			Padding(1, 2)
)
