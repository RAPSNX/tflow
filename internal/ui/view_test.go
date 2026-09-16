package ui

import (
	"charm.land/lipgloss/v2"
	"errors"
	"regexp"
	"strings"
	"testing"
)

// TestRenderMenuBarShowsBrandSectionAndSessionList guards the layout: the
// tflow badge as plain text on its own top line (a distinct mark, not a
// list entry), with the session list -- headed "Sessions" -- stacked below
// it, indented, and framed in exactly one thin border of its own.
func TestRenderMenuBarShowsBrandSectionAndSessionList(t *testing.T) {
	m := newMenu().(model)
	m.width = 60
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	rendered := m.renderMenuBar(60)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(rendered, "")
	lines := strings.Split(plain, "\n")
	if !strings.Contains(plain, "Sessions") {
		t.Fatalf("menu bar missing the session list header: %q", plain)
	}
	if len(lines) == 0 || !strings.Contains(lines[0], "TFLOW") {
		t.Fatalf("menu bar should show the brand on its own first line: %q", plain)
	}

	borderLine := -1
	for i, line := range lines {
		if strings.Contains(line, "╭") {
			borderLine = i
			break
		}
	}
	if borderLine <= 0 {
		t.Fatalf("session list border should start on a line below the brand: %q", plain)
	}
	borders := 0
	for _, line := range lines {
		borders += strings.Count(line, "╭")
	}
	if borders != 1 {
		t.Fatalf("menu bar should have exactly one bordered section, the session list: %q", plain)
	}
	if !strings.HasPrefix(lines[borderLine], " ") {
		t.Fatalf("session list border should be indented, not flush with the brand's left edge: %q", plain)
	}

	devIndex := strings.Index(plain, "dev")
	if devIndex == -1 {
		t.Fatalf("menu bar missing session: %q", plain)
	}
}

// TestRenderMenuHasNoUnstyledBackgroundGaps guards against a regression
// where composing pre-rendered styled strings with lipgloss.JoinVertical/
// lipgloss.Place -- both of which pad shorter lines with plain, uncoloured
// space, and neither of which recolors a nested style's own embedded reset
// -- left gaps that never picked up the popup's own background: behind the
// badge, behind each row's own label (after its type chip's reset), the
// session list's border glyphs (Background and BorderBackground are
// distinct lipgloss properties), and the strip to the right of the list's
// border. A single visible background colour code elsewhere in a line
// isn't enough to prove this: it must actually be the *active* SGR state
// covering every visible character, tracked here by walking each line's
// escape sequences in order rather than just checking which colours
// appear anywhere in it.
func TestRenderMenuHasNoUnstyledBackgroundGaps(t *testing.T) {
	cases := []struct {
		name string
		mod  func(*model)
	}{
		{"plain", func(m *model) {}},
		{"help and status", func(m *model) {
			m.showHelp = true
			m.status = "Type a project prefix to switch."
		}},
		{"error status", func(m *model) {
			m.err = errors.New("boom")
			m.status = "boom"
		}},
		{"empty context", func(m *model) {
			m.sessions = nil
		}},
		{"attention, git, and agent rows", func(m *model) {
			m.sessions = append(m.sessions,
				session{Name: "gitty", Attention: true},
				session{Name: "agenty"},
			)
			m.sessionProjects["gitty"] = defaultProjectName
			m.sessionProjects["agenty"] = defaultProjectName
			m.sessionTypes = map[string]string{"gitty": sessionTypeGit, "agenty": sessionTypeAgent}
		}},
		{"switch-project picker", func(m *model) {
			m.mode = inputSwitchProject
			m.projects = append(m.projects, "storage")
		}},
		{"switch-project picker, no matches", func(m *model) {
			m.mode = inputSwitchProject
			m.input.SetValue("nonexistent")
		}},
		{"move-session picker", func(m *model) {
			m.mode = inputMoveSession
			m.moveTarget = moveTarget{session: "dev"}
			m.projects = append(m.projects, "storage")
		}},
		{"create session panel", func(m *model) {
			m.mode = inputCreateSession
		}},
		{"rename panel", func(m *model) {
			m.mode = inputRename
			m.renameTarget = renameTarget{session: "dev"}
		}},
		{"delete confirmation panel (red text)", func(m *model) {
			m.mode = inputConfirmDelete
			m.deleteTarget = deleteTarget{session: "dev"}
		}},
		{"quit confirmation panel", func(m *model) {
			m.mode = inputConfirmQuit
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMenu().(model)
			m.width = 60
			m.height = 24
			m.projects = []string{defaultProjectName}
			m.sessions = []session{{Name: "dev"}, {Name: "feature-x"}}
			m.sessionProjects = map[string]string{"dev": defaultProjectName, "feature-x": defaultProjectName}
			m.selectedProject = defaultProjectName
			m.selectedSession = "dev"
			m.currentSession = "dev"
			tc.mod(&m)

			for i, gap := range unstyledBackgroundGaps(m.View()) {
				t.Fatalf("line %d has a gap with no active background colour (falls through to the terminal's own default): %q", i, gap)
			}
		})
	}
}

// TestRenderDialogsHaveNoUnstyledBackgroundGaps guards the same defect as
// TestRenderMenuHasNoUnstyledBackgroundGaps (see its comment), applied to
// the six dialog modes folded into the popup's own panel
// (renderTextInputPanel/renderConfirmPanel via renderPanelContent): they
// share the same badge/border shell and fillLines composition as the
// session list and pickers, so the same background-gap regression applies.
func TestRenderDialogsHaveNoUnstyledBackgroundGaps(t *testing.T) {
	cases := []struct {
		name string
		mod  func(m *model)
	}{
		{"create session", func(m *model) { m.mode = inputCreateSession }},
		{"create project", func(m *model) { m.mode = inputCreateProject }},
		{"rename", func(m *model) {
			m.mode = inputRename
			m.renameTarget = renameTarget{session: "small--otter"}
			m.sessionLabels = map[string]string{"small--otter": "otter"}
		}},
		{"delete session", func(m *model) {
			m.mode = inputConfirmDelete
			m.deleteTarget = deleteTarget{session: "small--otter"}
			m.sessionLabels = map[string]string{"small--otter": "otter"}
		}},
		{"delete project", func(m *model) {
			m.mode = inputConfirmDelete
			m.deleteTarget = deleteTarget{project: "small"}
		}},
		{"quit confirm", func(m *model) { m.mode = inputConfirmQuit }},
		{"project switch confirm", func(m *model) {
			m.mode = inputConfirmProjectSwitch
			m.switchProjectTarget = "small"
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := newMenu().(model)
			m.width = 64
			m.height = 24
			m.input.Prompt = ""
			m.projects = []string{"small"}
			m.sessions = []session{{Name: "small--otter"}}
			m.sessionProjects = map[string]string{"small--otter": "small"}
			tc.mod(&m)

			for i, gap := range unstyledBackgroundGaps(m.View()) {
				t.Fatalf("line %d has a gap with no active background colour (falls through to the terminal's own default): %q", i, gap)
			}
		})
	}
}

// unstyledBackgroundGaps walks each line of s as a terminal would: tracking
// the currently active SGR background as escape sequences are encountered
// in order (a bare reset, "\x1b[m", clears it; a "48;2;r;g;b" sequence sets
// it; anything else, e.g. a foreground-only code, leaves it as is, since
// SGR attributes are independent). It returns the plain (ANSI-stripped)
// text of every line that has a run of visible characters with no active
// background at all.
func unstyledBackgroundGaps(s string) []string {
	tokenRe := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	bgSet := regexp.MustCompile(`48;2;\d+;\d+;\d+`)
	plainRe := regexp.MustCompile(`\x1b\[[0-9;]*m`)

	var gaps []string
	for _, line := range strings.Split(s, "\n") {
		idx := 0
		activeBG := false
		hasGap := false
		for _, loc := range tokenRe.FindAllStringIndex(line, -1) {
			if line[idx:loc[0]] != "" && !activeBG {
				hasGap = true
			}
			idx = loc[1]
			code := line[loc[0]:loc[1]]
			switch {
			case code == "\x1b[m" || code == "\x1b[0m":
				activeBG = false
			case bgSet.MatchString(code):
				activeBG = true
			}
		}
		if idx < len(line) && !activeBG {
			hasGap = true
		}
		if hasGap {
			gaps = append(gaps, plainRe.ReplaceAllString(line, ""))
		}
	}
	return gaps
}

func TestRenderSessionRowUsesDisplayLabel(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.selectedSession = "small--code"
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionRow(0, 0, m.sessions[0]), "")
	if !strings.Contains(plain, "code") || strings.Contains(plain, "small--code") {
		t.Fatalf("session row = %q", plain)
	}
}

func TestRenderSessionRowShowsTypeChipAcrossStates(t *testing.T) {
	strip := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	m := newMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.sessions = []session{{Name: "s1"}, {Name: "s2"}, {Name: "s3"}}
	m.sessionProjects = map[string]string{"s1": "small", "s2": "small", "s3": "small"}
	m.sessionLabels = map[string]string{"s1": "code", "s2": "git", "s3": "agent"}
	m.sessionTypes = map[string]string{"s2": sessionTypeGit, "s3": sessionTypeAgent}

	cases := []struct {
		index   int
		want    string
		exclude []string
	}{
		{0, ">_", []string{"⎇", "✦", "CODE", "GIT", "AGENT"}},
		{1, "⎇", []string{">_", "✦", "CODE", "GIT", "AGENT"}},
		{2, "✦", []string{">_", "⎇", "CODE", "GIT", "AGENT"}},
	}
	for _, tc := range cases {
		plain := strip.ReplaceAllString(m.renderSessionRow(tc.index, tc.index, m.sessions[tc.index]), "")
		if !strings.Contains(plain, tc.want) {
			t.Fatalf("row %d missing chip %q: %q", tc.index, tc.want, plain)
		}
		for _, excluded := range tc.exclude {
			if strings.Contains(plain, excluded) {
				t.Fatalf("row %d unexpectedly contains %q: %q", tc.index, excluded, plain)
			}
		}
	}

	// Selection and attention states never replace the chip; the live
	// session marks itself only by the chip's own colour, not a separate
	// badge.
	m.currentSession = "s2"
	liveRow := m.renderSessionRow(1, 1, m.sessions[1])
	if !strings.Contains(liveRow, liveChipStyle.Render("⎇")) {
		t.Fatalf("selected+live git row should use the live chip colour: %q", liveRow)
	}
	plainLive := strip.ReplaceAllString(liveRow, "")
	if !strings.Contains(plainLive, "⎇") {
		t.Fatalf("selected+live git row lost its chip: %q", plainLive)
	}
	if strings.Contains(plainLive, "live") {
		t.Fatalf("live row should not render a separate text badge: %q", plainLive)
	}
}

// TestRenderSessionRowKeepsSelectedHighlightPastTheChip guards against a
// regression where the chip's own pre-rendered ANSI reset erased the
// selected row's background/foreground for everything after it (the
// attention mark, the live badge, and the label).
func TestRenderSessionRowKeepsSelectedHighlightPastTheChip(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.sessions = []session{{Name: "s1", Attention: true}}
	m.sessionProjects = map[string]string{"s1": "small"}
	m.sessionLabels = map[string]string{"s1": "flagged"}
	m.currentSession = "s1"

	row := m.renderSessionRow(0, 0, m.sessions[0])
	wantLabel := selectedSessionStyle.Background(baseBG).Padding(0).Render(" flagged")
	if !strings.Contains(row, wantLabel) {
		t.Fatalf("selected row lost its highlight before the label: %q", row)
	}
}

func TestRenderSessionRowShowsAttentionIndependentOfTypeAndSelection(t *testing.T) {
	strip := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	m := newMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.sessions = []session{{Name: "s1", Attention: true}, {Name: "s2"}}
	m.sessionProjects = map[string]string{"s1": "small", "s2": "small"}
	m.sessionLabels = map[string]string{"s1": "flagged", "s2": "quiet"}
	m.sessionTypes = map[string]string{"s1": sessionTypeGit}

	flagged := strip.ReplaceAllString(m.renderSessionRow(0, 0, m.sessions[0]), "")
	if !strings.Contains(flagged, "!") || !strings.Contains(flagged, "⎇") {
		t.Fatalf("flagged git row missing attention mark or type chip: %q", flagged)
	}

	quiet := strip.ReplaceAllString(m.renderSessionRow(1, 1, m.sessions[1]), "")
	if strings.Contains(quiet, "!") {
		t.Fatalf("unflagged row unexpectedly shows attention mark: %q", quiet)
	}

	// Attention survives alongside live status too, indicated only by the
	// chip's colour, not a separate badge.
	m.currentSession = "s1"
	liveRow := m.renderSessionRow(0, 0, m.sessions[0])
	if !strings.Contains(liveRow, liveChipStyle.Render("⎇")) {
		t.Fatalf("flagged+live row should use the live chip colour: %q", liveRow)
	}
	plainLive := strip.ReplaceAllString(liveRow, "")
	if !strings.Contains(plainLive, "!") {
		t.Fatalf("flagged+live row lost its attention mark: %q", plainLive)
	}
}

func TestRenderSessionPanelShowsFlatSessionsOnly(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{
		"dev": defaultProjectName,
		"api": "small",
	}
	m.currentSession = "dev"
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"

	view := m.renderSessionPanel()
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	if !strings.Contains(plain, "dev") {
		t.Fatalf("renderSessionPanel missing %q in %q", "dev", plain)
	}
	for _, unwanted := range []string{"Projects", "small", "[-]", "[agent]", "[k9s]"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderSessionPanel unexpectedly contained %q in %q", unwanted, plain)
		}
	}
}

// TestRenderSessionPanelSeparatesHeaderFromRows guards the blank line
// between the "Sessions" header and the first row -- without it the list
// reads as crowding straight underneath its own label.
func TestRenderSessionPanelSeparatesHeaderFromRows(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.selectedProject = defaultProjectName
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedSession = "dev"

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionPanel(), "")
	lines := strings.Split(plain, "\n")
	headerLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Sessions") {
			headerLine = i
			break
		}
	}
	if headerLine == -1 {
		t.Fatalf("renderSessionPanel missing header in %q", plain)
	}
	if headerLine+1 >= len(lines) || strings.TrimSpace(lines[headerLine+1]) != "" {
		t.Fatalf("renderSessionPanel should have a blank line right after the header: %q", plain)
	}
}

func TestRenderSessionPanelUsesCurrentProjectContext(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{
		"dev": defaultProjectName,
		"api": "small",
	}
	m.selectedProject = "small"
	m.selectedSession = "api"

	view := m.renderSessionPanel()
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	if !strings.Contains(plain, "api") {
		t.Fatalf("renderSessionPanel missing selected-project session in %q", plain)
	}
	if strings.Contains(plain, "dev") {
		t.Fatalf("renderSessionPanel leaked other-context session in %q", plain)
	}
	if strings.Contains(plain, "Projects") {
		t.Fatalf("renderSessionPanel unexpectedly contained grouped project UI in %q", plain)
	}
}

func TestRenderMenuIncludesBrandSessionPanelAndStatusArea(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.height = 16
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.status = "Type a project prefix to switch."

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderMenu(), "")
	for _, want := range []string{"TFLOW", "dev", "Type a project prefix to switch."} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderMenu missing %q in %q", want, plain)
		}
	}
}

func TestRenderFooterShowsOnlyInlineStatusByDefault(t *testing.T) {
	m := newMenu().(model)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderFooter(60), "")
	if strings.TrimSpace(plain) != "" {
		t.Fatalf("default footer = %q, want empty", plain)
	}
	m.status = "Saved."
	plain = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderFooter(60), "")
	if !strings.Contains(plain, "Saved.") {
		t.Fatalf("status footer = %q", plain)
	}
}

func TestRenderHelpListsOneShortcutPerRow(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.height = 24
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHelp(), "")
	for _, want := range []string{"Ctrl+F, f", "h / l (command sidebar)", "Ctrl+Q", "Ctrl+C", "Esc", "?", "j", "k", "Enter", "n", "N", "p", "r", "R", "d", "D", "e"} {
		count := 0
		for _, line := range strings.Split(plain, "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), want) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("shortcut %q appears %d times in %q", want, count, plain)
		}
	}
}

func TestRenderMenuShowsHelpInlineBelowSessions(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.height = 24
	m.showHelp = true
	m.sessions = []session{{Name: "dev"}}
	m.currentSession = "dev"
	m.selectedSession = "dev"

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	menuBar := strings.Index(plain, "TFLOW")
	help := strings.Index(plain, "Shortcuts")
	if menuBar < 0 || help < 0 || help <= menuBar {
		t.Fatalf("help was not rendered below the menu bar: %q", plain)
	}
	lines := strings.Split(plain, "\n")
	for index, line := range lines {
		if strings.Contains(line, "Shortcuts") {
			if index == 0 || strings.TrimSpace(lines[index-1]) != "" {
				t.Fatalf("help is missing a gap below sessions: %q", plain)
			}
			break
		}
	}
}

// TestConfirmationPanelsShowMessageWithoutKeycapFooter guards the item-B
// merge for the three confirm-only modes: no in-panel "Enter"/"Esc" keycap
// footer or "Cancel" action text -- action hints now live only in the
// bottom status line (m.status/statusView), matching the picker panels'
// precedent, not a per-dialog footer.
func TestConfirmationPanelsShowMessageWithoutKeycapFooter(t *testing.T) {
	m := newMenu().(model)
	m.width = 48
	m.height = 24
	m.switchProjectTarget = "small"

	for name, mode := range map[string]inputMode{
		"delete":         inputConfirmDelete,
		"project switch": inputConfirmProjectSwitch,
		"quit":           inputConfirmQuit,
	} {
		t.Run(name, func(t *testing.T) {
			m.mode = mode
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
			if strings.Contains(plain, "Cancel") {
				t.Fatalf("confirmation retained action text in %q", plain)
			}
			if strings.Contains(plain, "Enter") || strings.Contains(plain, "Esc") {
				t.Fatalf("confirmation panel should not carry its own keycap footer in %q", plain)
			}
		})
	}
}

// TestPanelRendersAtNarrowViewport guards the item-B merge's basic
// robustness at a narrow viewport: the merged panel now goes through the
// same fixed-width session-list shell every other mode uses (the popup's
// own list is deliberately a fixed width rather than a responsive one, see
// sessionLabelWidth's doc comment), so it isn't expected to shrink to fit
// an arbitrarily narrow m.width -- only to render without panicking and
// still show its header and input field.
func TestPanelRendersAtNarrowViewport(t *testing.T) {
	m := newMenu().(model)
	m.width = 28
	m.height = 16
	m.input.Prompt = ""
	m.mode = inputCreateSession

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Create Session") {
		t.Fatalf("panel missing its header at a narrow viewport: %q", plain)
	}
	if !strings.Contains(plain, "❯") {
		t.Fatalf("panel missing its input field at a narrow viewport: %q", plain)
	}
}

// TestSwitchProjectPickerListsMatchingProjectsInPopup guards the item-4
// merge: switching projects no longer opens a separate dialog card -- it
// replaces the sidebar popup's own panel content (same badge, same border)
// with a live-filtered project list, reached through the normal m.View()
// path (m.mode == inputSwitchProject falls through to the default,
// renderMenu, case in View's mode switch).
func TestSwitchProjectPickerListsMatchingProjectsInPopup(t *testing.T) {
	m := newMenu().(model)
	m.width = 60
	m.height = 24
	m.projects = []string{"small", "storage"}
	m.mode = inputSwitchProject
	m.input.Prompt = ""

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Switch Project") {
		t.Fatalf("switch-project picker missing its header in %q", plain)
	}
	lines := strings.Split(plain, "\n")
	foundSmall := false
	foundStorage := false
	for _, line := range lines {
		if strings.Contains(line, "small") {
			foundSmall = true
		}
		if strings.Contains(line, "storage") {
			foundStorage = true
		}
	}
	if !foundSmall || !foundStorage {
		t.Fatalf("switch-project picker missing the project list in %q", plain)
	}
}

// TestMoveSessionPickerListsMatchingProjectsInPopup is
// TestSwitchProjectPickerListsMatchingProjectsInPopup's counterpart for the
// move-session picker.
func TestMoveSessionPickerListsMatchingProjectsInPopup(t *testing.T) {
	m := newMenu().(model)
	m.width = 60
	m.height = 24
	m.projects = []string{"small", "storage"}
	m.sessions = []session{{Name: "small--otter"}}
	m.sessionLabels = map[string]string{"small--otter": "otter"}
	m.sessionProjects = map[string]string{"small--otter": "small"}
	m.mode = inputMoveSession
	m.moveTarget = moveTarget{session: "small--otter"}
	m.input.Prompt = ""

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Move otter to…") {
		t.Fatalf("move-session picker missing its header in %q", plain)
	}
	if !strings.Contains(plain, "storage") {
		t.Fatalf("move-session picker missing the target project list in %q", plain)
	}
	if strings.Contains(plain, "small") {
		t.Fatalf("move-session picker should exclude the session's current project: %q", plain)
	}
}

// TestRenderBadgesUseColourOnlyStyles guards the "single background colour"
// design for the session list specifically: the chips carry no
// *contrasting* fill of their own -- they distinguish themselves by
// foreground colour alone -- but each does carry the popup's own
// Background(baseBG), re-asserting it explicitly rather than relying on
// whatever an outer wrap happens to leave active (see fillLines' doc
// comment in view.go for why that distinction matters). The brand badge is
// the one deliberate exception: a filled Catppuccin-accent pill, so it
// reads as an actual badge. selectedSessionStyle itself stays
// background-less at the package level -- rowStyle and
// renderProjectPickerPanel each layer baseBG onto it locally for the
// popup only.
func TestRenderBadgesUseColourOnlyStyles(t *testing.T) {
	for name, style := range map[string]lipgloss.Style{
		"codeChipStyle":       codeChipStyle,
		"gitChipStyle":        gitChipStyle,
		"agentChipStyle":      agentChipStyle,
		"liveChipStyle":       liveChipStyle,
		"attentionBadgeStyle": attentionBadgeStyle,
	} {
		if style.GetBackground() != baseBG {
			t.Fatalf("%s background = %v, want the popup's single background %v", name, style.GetBackground(), baseBG)
		}
	}
	if _, ok := selectedSessionStyle.GetBackground().(lipgloss.NoColor); !ok {
		t.Fatalf("selectedSessionStyle should have no background at the package level (dialogs render through it directly): got %v", selectedSessionStyle.GetBackground())
	}
	if brandBadgeStyle.GetBackground() != blueColor {
		t.Fatalf("brand badge background = %v, want the filled pill colour %v", brandBadgeStyle.GetBackground(), blueColor)
	}
	if brandBadgeStyle.GetForeground() != badgeTextColor {
		t.Fatalf("brand badge foreground = %v, want %v", brandBadgeStyle.GetForeground(), badgeTextColor)
	}
	if liveChipStyle.GetForeground() != greenColor {
		t.Fatalf("live chip foreground = %v, want %v", liveChipStyle.GetForeground(), greenColor)
	}
	if liveChipStyle.GetForeground() == selectedSessionStyle.GetForeground() {
		t.Fatal("live chip and selected row use the same foreground colour")
	}
	m := newMenu().(model)
	m.width = 48
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": "small", "api": "small"}
	m.sessionLabels = map[string]string{"dev": "dev", "api": "api"}
	m.selectedProject = "small"
	m.currentSession = "dev"
	m.selectedSession = "dev"
	view := m.renderSessionPanel()
	if !strings.Contains(view, liveChipStyle.Render(">_")) {
		t.Fatalf("live session should render its chip with the live colour: %q", view)
	}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	if strings.Contains(plain, "live") {
		t.Fatalf("session panel should not render a separate live text badge: %q", plain)
	}
	selectedLabel := selectedSessionStyle.Background(baseBG).Padding(0).Render(" dev")
	if row := m.renderSessionRow(0, 0, m.sessions[0]); !strings.Contains(row, selectedLabel) {
		t.Fatalf("active selected row does not keep its highlight around the label: %q", row)
	}
}

// TestPanelsShareStaticBadgeAndModeHeader guards the item-B merge's design:
// every one of the six folded-in modes keeps the same static "TFLOW" badge
// (never a per-mode badge like the old CREATE/RENAME/DELETE/CONFIRM cards
// had) and instead carries its identity in the panel's own header text,
// exactly like the two already-merged pickers.
func TestPanelsShareStaticBadgeAndModeHeader(t *testing.T) {
	m := newMenu().(model)
	m.width = 64
	m.height = 24
	m.input.Prompt = ""
	m.selectedProject = "small"
	m.sessionLabels = map[string]string{"small--otter": "otter"}
	m.renameTarget = renameTarget{session: "small--otter"}
	m.deleteTarget = deleteTarget{session: "small--otter"}
	m.switchProjectTarget = "small"
	tests := []struct {
		name   string
		mode   inputMode
		header string
		copy   string
	}{
		{"create session", inputCreateSession, "Create Session", ""},
		{"create project", inputCreateProject, "Create Project", ""},
		{"rename", inputRename, "Rename Session", ""},
		{"delete", inputConfirmDelete, "Delete Session", "Delete otter?"},
		{"project switch confirm", inputConfirmProjectSwitch, "Switch Project", "Switch to small?"},
		{"quit", inputConfirmQuit, "Quit", "Remove this instance’s volatile sessions and quit?"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.mode = test.mode
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
			for _, want := range []string{"TFLOW", test.header} {
				if !strings.Contains(plain, want) {
					t.Fatalf("panel missing %q in %q", want, plain)
				}
			}
			if test.copy != "" && !strings.Contains(strings.Join(strings.Fields(strings.ReplaceAll(plain, "│", "")), " "), test.copy) {
				t.Fatalf("panel missing copy %q in %q", test.copy, plain)
			}
			if strings.Contains(plain, "Cancel") {
				t.Fatalf("panel retained action text in %q", plain)
			}
		})
	}
}

func TestDeleteConfirmationExplainsProjectConsequences(t *testing.T) {
	m := newMenu().(model)
	m.width = 64
	m.height = 24
	m.mode = inputConfirmDelete
	m.sessionProjects = map[string]string{"small--otter": "small", "small--fox": "small"}
	m.sessionLabels = map[string]string{"small--otter": "otter", "small--fox": "fox"}
	tests := []struct {
		name     string
		sessions []session
		target   deleteTarget
		want     string
	}{
		{"last session", []session{{Name: "small--otter"}}, deleteTarget{session: "small--otter"}, "Delete project small and its session?"},
		{"project", []session{{Name: "small--otter"}, {Name: "small--fox"}}, deleteTarget{project: "small"}, "Delete project small and all its sessions?"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			m.sessions, m.deleteTarget = test.sessions, test.target
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
			plain = strings.Join(strings.Fields(strings.ReplaceAll(plain, "│", "")), " ")
			if !strings.Contains(plain, test.want) {
				t.Fatalf("delete confirmation = %q, want %q", plain, test.want)
			}
		})
	}
}

// TestDeleteConfirmationUsesRedTextNotAFill guards this design's rule that
// delete's destructive cue is plain red *text*, never a filled badge --
// renderConfirmPanel's message style must carry redColor as its foreground
// with no background of its own beyond the popup's shared baseBG, unlike
// the old destructiveBadgeStyle's solid red fill.
func TestDeleteConfirmationUsesRedTextNotAFill(t *testing.T) {
	m := newMenu().(model)
	m.width = 64
	m.height = 24
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: "small--otter"}
	m.sessionProjects = map[string]string{"small--otter": "small"}
	m.sessionLabels = map[string]string{"small--otter": "otter"}

	_, message := m.deleteConfirmation()
	got := m.renderPanelContent()
	width := sessionLabelWidth + 2*2
	wantStyled := mutedStyle.Background(baseBG).Width(width).Foreground(redColor).Render(message)
	if !strings.Contains(got, wantStyled) {
		t.Fatalf("delete confirmation message should render in red text: got %q, want it to contain %q", got, wantStyled)
	}
}

func TestConfirmationPanelKeepsStatusVisibleInFooter(t *testing.T) {
	m := newMenu().(model)
	m.width = 64
	m.height = 20
	m.mode = inputCreateProject
	m.status = "Saved project settings."
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	if !strings.Contains(plain, "Saved project settings.") {
		t.Fatalf("panel status missing from %q", plain)
	}
	if strings.LastIndex(plain, "Saved project settings.") < strings.LastIndex(plain, "Create Project") {
		t.Fatalf("status was not rendered below the panel: %q", plain)
	}
}
