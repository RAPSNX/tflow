package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	return appStyle.Width(m.width).Height(m.height).Render(m.renderMenu())
}

// fillLines pads every given line (which may itself be multi-line) to the
// width of the widest one, through a style with both Width() and
// Background(baseBG) of its own, and returns them ready to hand straight to
// lipgloss.JoinVertical/JoinHorizontal.
//
// This matters because lipgloss.JoinVertical, JoinHorizontal, and
// lipgloss.Place (without a WithWhitespaceStyle option) all pad shorter
// content with *plain, uncoloured* space -- and most of what's joined here
// already ends in its own embedded reset (from badge/chip/row styles
// rendering themselves first). Once plain padding is baked into a composed
// line next to a reset, no later outer Background() wrap can recolor it: an
// outer style's SGR prefix applies once at the very start of a line and is
// cancelled by any reset already inside it. Style.Width()'s own padding,
// done here instead before anything is joined, is safe because it's
// appended fresh after the (already-reset) content, not dependent on
// whatever an outer wrap does afterward.
func fillLines(lines ...string) []string {
	width := 0
	for _, line := range lines {
		if w := lipgloss.Width(line); w > width {
			width = w
		}
	}
	style := lipgloss.NewStyle().Background(baseBG).Width(width)
	filled := make([]string, len(lines))
	for i, line := range lines {
		filled[i] = style.Render(line)
	}
	return filled
}

func (m model) renderMenu() string {
	// appStyle wraps this in its own Padding(1) (2 rows, 2 columns) before
	// applying Width/Height, so every width computed here must already
	// exclude that, or appStyle's own Height enforcement re-crops the
	// placement afterward.
	areaWidth := max(28, m.width-2)
	sections := []string{m.renderMenuBar(areaWidth)}
	if m.showHelp {
		sections = append(sections, "", m.renderHelp())
	}
	body := lipgloss.JoinVertical(lipgloss.Left, fillLines(sections...)...)
	footer := m.renderFooter(areaWidth)
	footerHeight := 0
	if footer != "" {
		footerHeight = lipgloss.Height(footer)
	}
	// The badge+session-list component (and the help panel below it, when
	// shown) is centered in the popup's remaining space above the footer,
	// rather than pinned to the top-left corner. lipgloss.Place fills any
	// gap it introduces with *plain, uncoloured* space by default -- pass
	// WithWhitespaceStyle so that gap (most visibly, the strip to the right
	// of the session list's border) carries the popup's own background
	// instead of falling through to the terminal's native one.
	fill := lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Background(baseBG))
	areaHeight := max(lipgloss.Height(body), m.height-2-footerHeight)
	centered := lipgloss.Place(areaWidth, areaHeight, lipgloss.Center, lipgloss.Center, body, fill)
	if footer == "" {
		return centered
	}
	// footer is already rendered at areaWidth, matching centered, so this
	// join needs no padding of its own on either block.
	return lipgloss.JoinVertical(lipgloss.Left, centered, footer)
}

// sessionLabelWidth is the assumed maximum session label length the list
// box is sized for -- fixed, not derived from the actual longest label or
// the popup's available width, so the list stays a narrow, predictable
// strip instead of stretching to match whatever room the popup has.
const sessionLabelWidth = 32

// listIndent shifts the session list box right, under and offset from the
// plain badge line above it, rather than sitting flush at its left edge.
const listIndent = 4

// listBorderOverhead is how much horizontal room the list's own border
// consumes (1 column each side), so renderMenuBar can size its interior to
// fit within the width it's given rather than overflowing once the border
// wraps around it.
const listBorderOverhead = 2

// renderMenuBar lays the popup out as the tflow badge -- a filled pill, so
// it reads as a static logo mark, not a list-like section -- on its own
// line, with the session list beneath it, offset to the right: a
// "Sessions" header followed by every contextual session stacked one per
// row, framed in one thin border of its own.
func (m model) renderMenuBar(width int) string {
	badge := brandBadgeStyle.Render("TFLOW")
	// 2 (marker, "▎ " or blank) + 2 (widest type icon, ">_") + 1 (space) +
	// sessionLabelWidth (label), plus this row's own padding (2 each side,
	// sessionStyle/selectedSessionStyle) and the panel's own padding (3
	// each side, panelStyle).
	listWidth := 2 + 2 + 1 + sessionLabelWidth + 2*2 + 3*2
	available := max(20, width-listIndent-listBorderOverhead)
	list := panelStyle.Width(min(listWidth, available)).Render(m.renderPanelContent())
	// Background(baseBG) so its own left-indent padding is filled with the
	// popup's colour rather than left plain.
	indentedList := lipgloss.NewStyle().Background(baseBG).Padding(0, 0, 0, listIndent).Render(list)
	return lipgloss.JoinVertical(lipgloss.Left, fillLines(badge, "", indentedList)...)
}

// renderPanelContent picks what fills the session list's bordered box:
// normally the session list itself, but the switch-project and
// move-session pickers replace it with a live-filtered project list in the
// same popup shell (same badge, same border, same fixed width) instead of
// opening a separate dialog -- they already rendered a list reusing the
// sidebar's own row styles even as standalone dialogs, so folding them in
// here is just moving where that list lives, not changing how it works.
func (m model) renderPanelContent() string {
	switch m.mode {
	case inputSwitchProject:
		return m.renderProjectPickerPanel("Switch Project", m.matchingProjects(m.input.Value()), m.projectSwitchIndex)
	case inputMoveSession:
		header := "Move " + fallbackText(m.sessionLabel(m.moveTarget.session), "session") + " to…"
		return m.renderProjectPickerPanel(header, m.matchingMoveProjects(m.input.Value()), m.moveProjectIndex)
	case inputCreateSession:
		return m.renderTextInputPanel("Create Session")
	case inputCreateProject:
		return m.renderTextInputPanel("Create Project")
	case inputRename:
		title := "Rename Session"
		if m.renameTarget.project != "" {
			title = "Rename Project"
		}
		return m.renderTextInputPanel(title)
	case inputConfirmDelete:
		title, message := m.deleteConfirmation()
		return m.renderConfirmPanel(title, message, true)
	case inputConfirmProjectSwitch:
		return m.renderConfirmPanel("Switch Project", "Switch to "+fallbackText(m.switchProjectTarget, "none")+"?", false)
	case inputConfirmQuit:
		return m.renderConfirmPanel("Quit", "Remove this instance’s volatile sessions and quit?", false)
	default:
		return m.renderSessionPanel()
	}
}

// deleteConfirmation mirrors beginDelete's own target-resolution logic to
// produce the header/message pair for the delete confirmation panel: a sole
// session belonging to a single-session project is reframed as deleting the
// project (matching the consequence that will actually happen).
func (m model) deleteConfirmation() (title, message string) {
	title, message = "Delete Selection", "Delete selection?"
	switch {
	case m.deleteTarget.project != "":
		title = "Delete Project"
		message = "Delete project " + m.deleteTarget.project + " and all its sessions?"
	case m.deleteTarget.session != "":
		title = "Delete Session"
		message = "Delete " + m.sessionLabel(m.deleteTarget.session) + "?"
	}
	if m.deleteTarget.session != "" {
		project := normalizeProjectName(m.sessionProjects[m.deleteTarget.session])
		if project != "" && len(m.projectSessions(project)) == 1 {
			title = "Delete Project"
			message = "Delete project " + project + " and its session?"
		}
	}
	return title, message
}

// renderTextInputPanel is the free-text counterpart to
// renderProjectPickerPanel: a header, a blank line, then the shared input
// field -- no row list, since these modes (create/rename) collect a single
// value rather than picking from a set.
func (m model) renderTextInputPanel(header string) string {
	rows := []string{sessionListHeaderStyle.Render(header), "", m.renderPanelInputField()}
	return lipgloss.JoinVertical(lipgloss.Left, fillLines(rows...)...)
}

// renderConfirmPanel renders a header, a blank line, then the confirmation
// message wrapped to the panel's fixed width -- lipgloss wraps automatically
// once Width is set. destructive renders the message in plain red text
// (never a filled badge, consistent with the rest of this design) as the
// only accent marking a delete confirmation apart from a plain one.
func (m model) renderConfirmPanel(header, message string, destructive bool) string {
	width := sessionLabelWidth + 2*2
	msgStyle := mutedStyle.Background(baseBG).Width(width)
	if destructive {
		msgStyle = msgStyle.Foreground(redColor)
	}
	rows := []string{sessionListHeaderStyle.Render(header), "", msgStyle.Render(message)}
	return lipgloss.JoinVertical(lipgloss.Left, fillLines(rows...)...)
}

// renderProjectPickerPanel renders a header, a live text input, a blank
// line, then every matching project as its own row -- reusing
// sessionStyle/selectedSessionStyle exactly like renderSessionRow does, so
// a picker row looks like a session row minus the type chip.
func (m model) renderProjectPickerPanel(header string, matches []string, selectedIndex int) string {
	rows := []string{sessionListHeaderStyle.Render(header), "", m.renderPanelInputField(), ""}
	if len(matches) == 0 {
		rows = append(rows, mutedStyle.Background(baseBG).Render("No matching projects."))
	} else {
		for index, project := range matches {
			style := sessionStyle.Background(baseBG)
			if index == selectedIndex {
				style = selectedSessionStyle.Background(baseBG)
			}
			rows = append(rows, style.Render(project))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, fillLines(rows...)...)
}

// renderPanelInputField renders the shared single-line text input used by
// every free-text mode (the project/move pickers' filter, and the
// create/rename panels' value field) -- plain, unboxed text (no nested
// border, per the popup's no-nested-boxes design), sized to the fixed list
// width.
func (m model) renderPanelInputField() string {
	width := sessionLabelWidth + 2*2
	return dialogInputStyle.Width(width).Render("❯ " + inputStyle.Render(m.input.View()))
}

// renderSessionPanel renders a "Sessions" header followed by every
// contextual session as its own row, stacked top to bottom in selection
// order, or a muted placeholder when the context is empty. A blank line
// separates the header from the rows so the list doesn't read as crowding
// straight underneath its own label.
func (m model) renderSessionPanel() string {
	sessions := m.contextSessions()
	selectedIndex := sessionIndex(sessions, m.selectedSession)
	header := sessionListHeaderStyle.Render("Sessions")

	var rows []string
	if len(sessions) == 0 {
		rows = []string{header, "", mutedStyle.Render("No sessions in this context")}
	} else {
		rows = make([]string, len(sessions)+2)
		rows[0] = header
		rows[1] = ""
		for index, s := range sessions {
			rows[index+2] = m.renderSessionRow(index, selectedIndex, s)
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, fillLines(rows...)...)
}

func (m model) renderSessionRow(index, selectedIndex int, s session) string {
	label := m.sessionLabel(s.Name)
	selected := index == selectedIndex
	live := s.Name == m.currentSession
	style := m.rowStyle(selected)
	plain := style.Padding(0)

	// A marker bar precedes the selected row (blank, same width, on every
	// other row so the chip stays aligned regardless of selection).
	// Rendered through plain (Background(baseBG)) even when blank, rather
	// than a raw string, so it doesn't reintroduce an unstyled background
	// gap at the very start of the row.
	markerGlyph := "  "
	if selected {
		markerGlyph = "▎ "
	}
	marker := plain.Render(markerGlyph)

	// The chip is pre-rendered with its own colour and ends in a terminal
	// reset, so the row style must be reapplied around every remaining
	// plain-text segment -- otherwise the row's foreground is lost for
	// everything after it. The live session is marked only by the chip's
	// own colour (see sessionTypeChip), not a separate badge, so each row
	// reads as "<marker><icon> <name>".
	segments := []string{marker, sessionTypeChip(m.sessionType(s.Name), live), plain.Render(" " + label)}
	if s.Attention {
		// Independent of type, selection, and live status: never replaces
		// the chip, and renders regardless of the other states.
		segments = append(segments, plain.Render(" "), attentionBadgeStyle.Render("!"))
	}

	// Sized to its own content, not stretched full-width -- rows stack top
	// to bottom inside the session list's own bordered box.
	return style.Render(strings.Join(segments, ""))
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
		"Ctrl+F, f     Toggle command sidebar",
		"h / l (command sidebar)  Navigate and close",
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
	return lipgloss.JoinVertical(lipgloss.Left, fillLines(rows...)...)
}
