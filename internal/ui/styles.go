package ui

import "charm.land/lipgloss/v2"

type themePalette struct {
	BaseBG    string
	Surface0  string
	Surface1  string
	Text      string
	Subtext   string
	Blue      string
	Teal      string
	Yellow    string
	Green     string
	Red       string
	Mantle    string
	Crust     string
	BadgeText string
	Mauve     string
}

var (
	baseBG         = lipgloss.Color("#24273A")
	surface0       = lipgloss.Color("#363A4F")
	surface1       = lipgloss.Color("#494D64")
	textColor      = lipgloss.Color("#CAD3F5")
	subtextColor   = lipgloss.Color("#A5ADCB")
	blueColor      = lipgloss.Color("#8AADF4")
	tealColor      = lipgloss.Color("#8BD5CA")
	yellowColor    = lipgloss.Color("#EED49F")
	greenColor     = lipgloss.Color("#A6DA95")
	redColor       = lipgloss.Color("#ED8796")
	mantleColor    = lipgloss.Color("#1E2030")
	crustColor     = lipgloss.Color("#181926")
	badgeTextColor = lipgloss.Color("#24273A")
	mauveColor     = lipgloss.Color("#C6A0F6")

	appStyle               lipgloss.Style
	titleStyle             lipgloss.Style
	brandBadgeStyle        lipgloss.Style
	mutedStyle             lipgloss.Style
	panelStyle             lipgloss.Style
	sessionStyle           lipgloss.Style
	selectedSessionStyle   lipgloss.Style
	sessionListHeaderStyle lipgloss.Style
	footerStyle            lipgloss.Style
	warningStatusStyle     lipgloss.Style
	errorStatusStyle       lipgloss.Style
	inputStyle             lipgloss.Style
	dialogInputStyle       lipgloss.Style
	codeChipStyle          lipgloss.Style
	gitChipStyle           lipgloss.Style
	agentChipStyle         lipgloss.Style
	liveChipStyle          lipgloss.Style
	attentionBadgeStyle    lipgloss.Style
)

func init() {
	applyTheme(catppuccinMacchiatoPalette())
}

func catppuccinMacchiatoPalette() themePalette {
	return themePalette{
		BaseBG:    "#24273a",
		Surface0:  "#363a4f",
		Surface1:  "#494d64",
		Text:      "#cad3f5",
		Subtext:   "#a5adcb",
		Blue:      "#8aadf4",
		Teal:      "#8bd5ca",
		Yellow:    "#eed49f",
		Green:     "#a6da95",
		Red:       "#ed8796",
		Mantle:    "#1e2030",
		Crust:     "#181926",
		BadgeText: "#24273a",
		Mauve:     "#c6a0f6",
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
	greenColor = lipgloss.Color(p.Green)
	redColor = lipgloss.Color(p.Red)
	mantleColor = lipgloss.Color(p.Mantle)
	crustColor = lipgloss.Color(p.Crust)
	badgeTextColor = lipgloss.Color(p.BadgeText)
	mauveColor = lipgloss.Color(p.Mauve)

	appStyle = lipgloss.NewStyle().
		Padding(1).
		Foreground(textColor).
		Background(baseBG)

	// Background(baseBG) so titleStyle stays safely coloured when it's
	// joined on the same line right after another pre-rendered segment
	// that ends in its own reset.
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(textColor).
		Background(baseBG)

	// The badge is a filled pill -- a Catppuccin accent background with
	// dark text on top, matching the same treatment the type chips use
	// elsewhere in the app -- so it actually reads as a badge, not just
	// coloured text.
	brandBadgeStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(badgeTextColor).
		Background(blueColor).
		Padding(0, 1)

	mutedStyle = lipgloss.NewStyle().
		Foreground(subtextColor)

	// panelStyle frames the session list in one thin border of its own, on
	// the popup's own background rather than a separate fill colour -- the
	// badge sits outside it, unboxed, above. BorderBackground is a distinct
	// property from Background in lipgloss -- it colours the border glyphs
	// themselves, which Background alone leaves with no background at all
	// (see (Style).styleBorder in charm.land/lipgloss/v2, which only emits
	// a background code for the border when BorderBackground is set).
	panelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(surface1).
		BorderBackground(baseBG).
		Background(baseBG).
		Padding(1, 3)

	sessionStyle = lipgloss.NewStyle().
		Foreground(textColor).
		Padding(0, 2)

	// The selected row is marked by colour alone -- bold mauve text --
	// rather than a background block, so every row (selected or not) stays
	// on the same single background colour throughout the list.
	selectedSessionStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(mauveColor).
		Padding(0, 2)

	// No horizontal padding of its own -- both it and each row are placed
	// by JoinVertical from their own block's start, so this stays aligned
	// with every row's own left edge without needing to match padding.
	sessionListHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(subtextColor)

	footerStyle = lipgloss.NewStyle().
		Background(baseBG).
		Padding(0, 1, 0, 1)

	// Background(baseBG) so statusView's own Width() fill (the status line
	// is rendered directly through this style, at the popup's full width,
	// before ever reaching footerStyle) is safely, freshly coloured rather
	// than left plain -- see fillLines' doc comment in view.go for why that
	// distinction matters.
	warningStatusStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(yellowColor).
		Background(baseBG)

	errorStatusStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(redColor).
		Background(baseBG)

	inputStyle = lipgloss.NewStyle().
		Foreground(textColor)

	// Plain, unboxed input text on baseBG -- no nested border -- matching
	// the sidebar popup's own no-nested-boxes design. renderPanelInputField
	// adds a leading marker to distinguish it as an input line.
	dialogInputStyle = lipgloss.NewStyle().Foreground(mauveColor).Background(baseBG).Padding(0, 1)
	// Session type chips are colour alone, matching the top bar's own icon
	// treatment (Palette.sessionTypeIcon in internal/tmux/types.go) -- no
	// *filled* background of their own (no contrasting block behind the
	// glyph), but Background(baseBG) still explicitly re-asserts the
	// popup's single background after whatever came before, since a
	// foreground-only style's own trailing reset would otherwise leave
	// nothing to carry it forward to the next segment.
	codeChipStyle = lipgloss.NewStyle().Bold(true).Foreground(blueColor).Background(baseBG)
	gitChipStyle = lipgloss.NewStyle().Bold(true).Foreground(tealColor).Background(baseBG)
	agentChipStyle = lipgloss.NewStyle().Bold(true).Foreground(yellowColor).Background(baseBG)
	liveChipStyle = lipgloss.NewStyle().Bold(true).Foreground(greenColor).Background(baseBG)
	attentionBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(redColor).Background(baseBG)
}

// sessionTypeChip renders the symbol-only, colour-only type chip for a
// session row: blue ">_" for terminal sessions (including legacy untyped
// records), teal "⎇" for git, or yellow "✦" for agent -- no background of
// its own, matching the top bar's own icon treatment. The type name is
// never spelled out -- the symbol is the only identity shown. Its colour is
// the one exception the live session makes: a live (currently attached)
// session always renders its chip in green regardless of type, since that
// colour is otherwise unused, so live status reads unambiguously from the
// icon alone without a separate badge. Selection and attention states never
// replace the chip -- callers append them alongside it, not instead of it.
func sessionTypeChip(sessionType string, live bool) string {
	symbol := ">_"
	style := codeChipStyle
	switch sessionType {
	case sessionTypeGit:
		symbol = "⎇"
		style = gitChipStyle
	case sessionTypeAgent:
		symbol = "✦"
		style = agentChipStyle
	}
	if live {
		style = liveChipStyle
	}
	return style.Render(symbol)
}
