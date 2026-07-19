package ui

import (
	"charm.land/lipgloss/v2"
	"regexp"
	"strings"
	"testing"
)

func TestRenderHeaderCentersBrandWithoutPopupMetadata(t *testing.T) {
	m := NewMenu().(model)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHeader(40), "")
	firstLine := strings.Split(plain, "\n")[0]
	if got, want := strings.Index(firstLine, "TFLOW"), 17; got != want {
		t.Fatalf("TFLOW offset = %d, want %d in %q", got, want, firstLine)
	}
	for _, unwanted := range []string{"project", "session"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("header unexpectedly contains %q in %q", unwanted, plain)
		}
	}
}

func TestRenderSessionRowUsesDisplayLabel(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.selectedProject = "small"
	m.selectedSession = "small--code"
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionRow(0, m.sessions[0]), "")
	if !strings.Contains(plain, "code") || strings.Contains(plain, "small--code") {
		t.Fatalf("session row = %q", plain)
	}
}

func TestRenderSessionPanelShowsFlatSessionsOnly(t *testing.T) {
	m := NewMenu().(model)
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

	view := m.renderSessionPanel(40)
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(view, "")
	for _, want := range []string{"Sessions", "live", "dev"} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderSessionPanel missing %q in %q", want, plain)
		}
	}
	for _, unwanted := range []string{"Projects", "small", "[-]", "[agent]", "[k9s]"} {
		if strings.Contains(plain, unwanted) {
			t.Fatalf("renderSessionPanel unexpectedly contained %q in %q", unwanted, plain)
		}
	}
}

func TestRenderSessionPanelUsesCurrentProjectContext(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{
		"dev": defaultProjectName,
		"api": "small",
	}
	m.selectedProject = "small"
	m.selectedSession = "api"

	view := m.renderSessionPanel(40)
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
	m := NewMenu().(model)
	m.width = 48
	m.height = 16
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.status = "Type a project prefix to switch."

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderMenu(), "")
	for _, want := range []string{"TFLOW", "Sessions", "Type a project prefix to switch."} {
		if !strings.Contains(plain, want) {
			t.Fatalf("renderMenu missing %q in %q", want, plain)
		}
	}
}

func TestRenderFooterShowsOnlyInlineStatusByDefault(t *testing.T) {
	m := NewMenu().(model)
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
	m := NewMenu().(model)
	m.width = 48
	m.height = 24
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderHelp(), "")
	for _, want := range []string{"Ctrl+F", "Ctrl+Q", "Ctrl+C", "Esc", "?", "j", "k", "Enter", "n", "N", "p", "r", "R", "d", "D", "e"} {
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
	m := NewMenu().(model)
	m.width = 48
	m.height = 24
	m.showHelp = true
	m.sessions = []session{{Name: "dev"}}
	m.currentSession = "dev"
	m.selectedSession = "dev"

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.View(), "")
	sessions := strings.Index(plain, "Sessions")
	help := strings.Index(plain, "Shortcuts")
	if sessions < 0 || help < 0 || help <= sessions {
		t.Fatalf("help was not rendered below sessions: %q", plain)
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

func TestConfirmationOverlaysAdvertiseAcceptedKeys(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.height = 24

	for name, render := range map[string]func() string{
		"delete":         m.renderDeleteOverlay,
		"project switch": m.renderProjectSwitchConfirmOverlay,
		"quit":           m.renderQuitConfirmOverlay,
	} {
		t.Run(name, func(t *testing.T) {
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(render(), "")
			if strings.Contains(plain, "Cancel") {
				t.Fatalf("confirmation retained action text in %q", plain)
			}
			footerFound := false
			for _, line := range strings.Split(plain, "\n") {
				if strings.Contains(line, "Enter") {
					footerFound = true
					if !strings.Contains(line, "Esc") {
						t.Fatalf("keycaps are not on one line in %q", plain)
					}
				}
			}
			if !footerFound {
				t.Fatalf("confirmation missing keycap footer in %q", plain)
			}
		})
	}
}

func TestDialogFitsNarrowViewportAndCentersKeycaps(t *testing.T) {
	m := NewMenu().(model)
	m.width = 28
	m.height = 16
	m.input.Prompt = ""

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderInputOverlay("New Session"), "")
	for _, line := range strings.Split(plain, "\n") {
		if lipgloss.Width(line) > m.width {
			t.Fatalf("dialog line width = %d, want at most %d: %q", lipgloss.Width(line), m.width, line)
		}
	}
	for _, line := range strings.Split(plain, "\n") {
		if !strings.Contains(line, "Enter") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(line), "│"), "│")
		left := len(inner) - len(strings.TrimLeft(inner, " "))
		right := len(inner) - len(strings.TrimRight(inner, " "))
		if difference := left - right; difference < -1 || difference > 1 {
			t.Fatalf("keycaps are not centered: %q", line)
		}
		return
	}
	t.Fatal("dialog keycaps not rendered")
}

func TestRenderProjectSwitchDialogListsMatchingProjects(t *testing.T) {
	m := NewMenu().(model)
	m.width = 48
	m.projects = []string{"small", "storage"}
	m.mode = inputSwitchProject
	m.input.Prompt = ""

	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderProjectSwitchOverlay(), "")
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
		t.Fatalf("renderFooter missing newline-separated project list in %q", plain)
	}
}

func TestRenderBadgesUseFilledContrastingStyles(t *testing.T) {
	if brandBadgeStyle.GetBackground() != blueColor {
		t.Fatalf("brand badge background = %v, want %v", brandBadgeStyle.GetBackground(), blueColor)
	}
	if currentBadgeStyle.GetBackground() != tealColor {
		t.Fatalf("live badge background = %v, want %v", currentBadgeStyle.GetBackground(), tealColor)
	}
	if currentBadgeStyle.GetBackground() == selectedSessionStyle.GetBackground() {
		t.Fatal("live badge and selected row use the same background")
	}
	m := NewMenu().(model)
	m.width = 48
	m.sessions = []session{{Name: "dev"}, {Name: "api"}}
	m.sessionProjects = map[string]string{"dev": "small", "api": "small"}
	m.sessionLabels = map[string]string{"dev": "dev", "api": "api"}
	m.selectedProject = "small"
	m.currentSession = "dev"
	m.selectedSession = "dev"
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderSessionPanel(40), "")
	if strings.Count(plain, "live") != 1 || !strings.Contains(plain, "live  dev") {
		t.Fatalf("active row = %q", plain)
	}
	selectedLabel := selectedSessionStyle.Copy().Padding(0).Render(" dev")
	if row := m.renderSessionRow(0, m.sessions[0]); !strings.Contains(row, selectedLabel) {
		t.Fatalf("active selected row does not restore its highlight after the live badge: %q", row)
	}
}

func TestDialogsUseSharedStructuredCardLayout(t *testing.T) {
	m := NewMenu().(model)
	m.width = 64
	m.height = 24
	m.input.Prompt = ""
	m.selectedProject = "small"
	m.sessionLabels = map[string]string{"small--otter": "otter"}
	m.renameTarget = renameTarget{session: "small--otter"}
	m.deleteTarget = deleteTarget{session: "small--otter"}
	m.switchProjectTarget = "small"
	tests := []struct {
		name     string
		render   func() string
		badge    string
		title    string
		copy     string
		unwanted []string
	}{
		{"create session", func() string { return m.renderInputOverlay("New Session") }, "CREATE", "Session", "", []string{"project: small"}},
		{"create project", func() string { return m.renderInputOverlay("New Project") }, "CREATE", "Project", "", []string{"project:"}},
		{"settings", func() string { return m.renderInputOverlay("Project Settings") }, "SETTINGS", "Project", "", nil},
		{"rename", m.renderRenameOverlay, "RENAME", "Session", "Current: otter", nil},
		{"switch", m.renderProjectSwitchOverlay, "SWITCH", "Project", "Choose a project.", nil},
		{"delete", m.renderDeleteOverlay, "DELETE", "Session", "Delete otter?", nil},
		{"confirm", m.renderProjectSwitchConfirmOverlay, "CONFIRM", "Project Switch", "Switch to small?", nil},
		{"quit", m.renderQuitConfirmOverlay, "CONFIRM", "Quit", "Remove this instance’s volatile sessions and quit?", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(test.render(), "")
			for _, want := range []string{test.badge, test.title, "──", "Enter", "Esc"} {
				if !strings.Contains(plain, want) {
					t.Fatalf("dialog missing %q in %q", want, plain)
				}
			}
			if test.copy != "" && !strings.Contains(strings.Join(strings.Fields(strings.ReplaceAll(plain, "│", "")), " "), test.copy) {
				t.Fatalf("dialog missing copy %q in %q", test.copy, plain)
			}
			if strings.Contains(plain, "Cancel") {
				t.Fatalf("dialog retained action text in %q", plain)
			}
			for _, unwanted := range test.unwanted {
				if strings.Contains(plain, unwanted) {
					t.Fatalf("dialog unexpectedly contains %q in %q", unwanted, plain)
				}
			}
		})
	}
}

func TestDeleteDialogExplainsProjectConsequences(t *testing.T) {
	m := NewMenu().(model)
	m.width = 64
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
			plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderDeleteOverlay(), "")
			plain = strings.Join(strings.Fields(strings.ReplaceAll(plain, "│", "")), " ")
			if !strings.Contains(plain, test.want) {
				t.Fatalf("delete dialog = %q, want %q", plain, test.want)
			}
		})
	}
}

func TestDeleteDialogUsesDestructiveAccentsOnly(t *testing.T) {
	if destructiveBadgeStyle.GetBackground() != redColor || destructiveKeycapStyle.GetBackground() != redColor {
		t.Fatal("delete accents do not use red")
	}
	if dialogHeaderBadgeStyle.GetBackground() == redColor || keycapStyle.GetBackground() == redColor {
		t.Fatal("normal dialog accents use red")
	}
}

func TestDialogKeepsStatusVisibleInFooter(t *testing.T) {
	m := NewMenu().(model)
	m.width = 64
	m.height = 20
	m.status = "Saved project settings."
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(m.renderInputOverlay("Project Settings"), "")
	if !strings.Contains(plain, "Saved project settings.") {
		t.Fatalf("dialog status missing from %q", plain)
	}
	if strings.LastIndex(plain, "Saved project settings.") < strings.LastIndex(plain, "Esc") {
		t.Fatalf("status was not rendered below the dialog: %q", plain)
	}
}
