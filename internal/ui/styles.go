package ui

import "charm.land/lipgloss/v2"

type themePalette struct {
	BaseBG       string
	Surface0     string
	Surface1     string
	Text         string
	Subtext      string
	Blue         string
	Teal         string
	Yellow       string
	Red          string
	Mantle       string
	Crust        string
	BadgeText    string
	SelectedText string
}

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

	appStyle               lipgloss.Style
	headerStyle            lipgloss.Style
	titleStyle             lipgloss.Style
	brandBadgeStyle        lipgloss.Style
	mutedStyle             lipgloss.Style
	panelStyle             lipgloss.Style
	sectionTitleStyle      lipgloss.Style
	sessionStyle           lipgloss.Style
	selectedSessionStyle   lipgloss.Style
	currentBadgeStyle      lipgloss.Style
	countBadgeStyle        lipgloss.Style
	footerStyle            lipgloss.Style
	warningStatusStyle     lipgloss.Style
	errorStatusStyle       lipgloss.Style
	inputStyle             lipgloss.Style
	overlayStyle           lipgloss.Style
	dialogHeaderBadgeStyle lipgloss.Style
	destructiveBadgeStyle  lipgloss.Style
	dialogDividerStyle     lipgloss.Style
	dialogInputStyle       lipgloss.Style
	keycapStyle            lipgloss.Style
	destructiveKeycapStyle lipgloss.Style
	codeChipStyle          lipgloss.Style
	gitChipStyle           lipgloss.Style
	agentChipStyle         lipgloss.Style
	attentionBadgeStyle    lipgloss.Style
)

func init() {
	applyTheme(catppuccinPalette())
}

func catppuccinPalette() themePalette {
	return themePalette{
		BaseBG:       "#1e1e2e",
		Surface0:     "#313244",
		Surface1:     "#45475a",
		Text:         "#cdd6f4",
		Subtext:      "#a6adc8",
		Blue:         "#89b4fa",
		Teal:         "#94e2d5",
		Yellow:       "#f9e2af",
		Red:          "#f38ba8",
		Mantle:       "#181825",
		Crust:        "#11111b",
		BadgeText:    "#1e1e2e",
		SelectedText: "#11111b",
	}
}

func applyTheme(p themePalette) {
	baseBG = lipgloss.Color(p.BaseBG)
	surface0 = lipgloss.Color(p.Surface0)
	surface1 = lipgloss.Color(p.Surface1)
	textColor = lipgloss.Color(p.Text)
	subtextColor = lipgloss.Color(p.Subtext)
	blueColor = lipgloss.Color(p.Blue)
	tealColor = lipgloss.Color(p.Teal)
	yellowColor = lipgloss.Color(p.Yellow)
	redColor = lipgloss.Color(p.Red)
	mantleColor = lipgloss.Color(p.Mantle)
	crustColor = lipgloss.Color(p.Crust)
	badgeTextColor = lipgloss.Color(p.BadgeText)
	selectedTextColor = lipgloss.Color(p.SelectedText)

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

	brandBadgeStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(badgeTextColor).
		Background(blueColor).
		Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(surface1).
		Background(mantleColor).
		Padding(1)

	sectionTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor)

	sessionStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Padding(0, 1)

	selectedSessionStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(selectedTextColor).
		Background(blueColor).
		Padding(0, 1)

	currentBadgeStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(badgeTextColor).
		Background(tealColor).
		Padding(0, 1)

	countBadgeStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Background(surface0).
		Padding(0, 1)

	footerStyle = lipgloss.NewStyle().
		Background(baseBG).
		Padding(0, 1, 0, 1)

	warningStatusStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(yellowColor)

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

	dialogHeaderBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(blueColor).Padding(0, 1)
	destructiveBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(redColor).Padding(0, 1)
	dialogDividerStyle = lipgloss.NewStyle().Foreground(surface1)
	dialogInputStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(tealColor).Padding(0, 1)
	keycapStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(tealColor).Padding(0, 1)
	destructiveKeycapStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(redColor).Padding(0, 1)
	codeChipStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(blueColor).Padding(0, 1)
	gitChipStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(tealColor).Padding(0, 1)
	agentChipStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(yellowColor).Padding(0, 1)
	attentionBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(badgeTextColor).Background(redColor).Padding(0, 1)
}

// sessionTypeChip renders the full worded type chip for a sidebar row: blue
// ">_ CODE" for terminal sessions (including legacy untyped records), teal
// "⏇ GIT", or yellow "✦ AGENT". Selection, live, and attention states never
// replace it -- callers append it alongside those, not instead of it.
func sessionTypeChip(sessionType string) string {
	switch sessionType {
	case sessionTypeGit:
		return gitChipStyle.Render("⎇ GIT")
	case sessionTypeAgent:
		return agentChipStyle.Render("✦ AGENT")
	default:
		return codeChipStyle.Render(">_ CODE")
	}
}
