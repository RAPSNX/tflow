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

	appStyle             lipgloss.Style
	headerStyle          lipgloss.Style
	titleStyle           lipgloss.Style
	subtitleStyle        lipgloss.Style
	brandBadgeStyle      lipgloss.Style
	mutedStyle           lipgloss.Style
	hintStyle            lipgloss.Style
	panelStyle           lipgloss.Style
	sectionTitleStyle    lipgloss.Style
	statsValueStyle      lipgloss.Style
	statsLabelStyle      lipgloss.Style
	projectStyle         lipgloss.Style
	selectedProjectStyle lipgloss.Style
	sessionStyle         lipgloss.Style
	selectedSessionStyle lipgloss.Style
	currentBadgeStyle    lipgloss.Style
	countBadgeStyle      lipgloss.Style
	footerStyle          lipgloss.Style
	statusStyle          lipgloss.Style
	errorStatusStyle     lipgloss.Style
	inputStyle           lipgloss.Style
	overlayStyle         lipgloss.Style
)

func init() {
	applyTheme(themePaletteForName("catppuccin"))
}

func themePaletteForName(name string) themePalette {
	switch name {
	case "forest":
		return themePalette{
			BaseBG:       "#18251a",
			Surface0:     "#35523a",
			Surface1:     "#466b4b",
			Text:         "#edf6ee",
			Subtext:      "#b7cbb9",
			Blue:         "#8fc2a0",
			Teal:         "#7fd1bf",
			Yellow:       "#f0d37a",
			Red:          "#e59c8e",
			Mantle:       "#132016",
			Crust:        "#0d170f",
			BadgeText:    "#132016",
			SelectedText: "#0d170f",
		}
	default:
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
}

func themeFromConfig(cfg appConfig) themePalette {
	palette := themePaletteForName(cfg.Theme)
	applyOverride := func(current, override string) string {
		if override == "" {
			return current
		}
		return override
	}
	palette.BaseBG = applyOverride(palette.BaseBG, cfg.Colors.BaseBG)
	palette.Surface0 = applyOverride(palette.Surface0, cfg.Colors.Surface0)
	palette.Surface1 = applyOverride(palette.Surface1, cfg.Colors.Surface1)
	palette.Text = applyOverride(palette.Text, cfg.Colors.Text)
	palette.Subtext = applyOverride(palette.Subtext, cfg.Colors.Subtext)
	palette.Blue = applyOverride(palette.Blue, cfg.Colors.Blue)
	palette.Teal = applyOverride(palette.Teal, cfg.Colors.Teal)
	palette.Yellow = applyOverride(palette.Yellow, cfg.Colors.Yellow)
	palette.Red = applyOverride(palette.Red, cfg.Colors.Red)
	palette.Mantle = applyOverride(palette.Mantle, cfg.Colors.Mantle)
	palette.Crust = applyOverride(palette.Crust, cfg.Colors.Crust)
	palette.BadgeText = applyOverride(palette.BadgeText, cfg.Colors.BadgeText)
	palette.SelectedText = applyOverride(palette.SelectedText, cfg.Colors.SelectedText)
	return palette
}
