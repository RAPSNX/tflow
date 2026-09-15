package tmux

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	socketName            = "tflow"
	menuPopupEnvPrefix    = "TFLOW_MENU_POPUP_"
	menuInstanceEnvPrefix = "TFLOW_MENU_INSTANCE_"
	projectMarker         = "@tflow-project"
	sessionLabelMarker    = "@tflow-session-label"
	tempMarker            = "@tflow-temp"
	instanceMarker        = "@tflow-instance"
	attentionMarker       = "@tflow-attention"
	visitedMarker         = "@tflow-visited-at"
	// The session list renders at a fixed width (see sessionLabelWidth in
	// internal/ui/view.go), not derived from the popup's own size, so the
	// popup itself needs a fixed width wide enough to fit the badge and
	// that list without wrapping, rather than a percentage of the
	// surrounding terminal.
	menuWidth = "60"
	// menuHeight stays below 100% so the popup fits under the status line.
	// tmux resolves a popup that would overflow by moving it back up rather
	// than shrinking it, so a full-height popup lands on the status line and
	// hides the top bar. Percentages are floored, so this always leaves a row.
	menuHeight = "35%"
	// prefixKey enters prefixTable, a brief wait state for the second key
	// of the chord (mirroring how tmux's own prefix key, e.g. C-b, works:
	// tmux auto-reverts the client to its previous table after exactly one
	// keypress, bound or not, so no explicit timeout/cancel handling is
	// needed beyond the Escape binding below for clarity).
	prefixKey          = "C-f"
	prefixTable        = "tflow-prefix"
	commandTable       = "tflow-command"
	quitKey            = "C-q"
	CurrentSessionEnv  = "TFLOW_CURRENT_SESSION"
	CurrentClientEnv   = "TFLOW_CURRENT_CLIENT"
	CurrentInstanceEnv = "TFLOW_INSTANCE_ID"
	// LastVisitedSessionEnv carries tmux's #{client_last_session} into the
	// client-session-changed hook: the session being switched away from, so
	// its watermark can be stamped at the exact moment of the switch rather
	// than waiting for AttentionScan's next tick.
	LastVisitedSessionEnv = "TFLOW_LAST_VISITED_SESSION"
	MenuModeEnv           = "TFLOW_MENU_MODE"
	MenuModeCommand       = "command"
	MenuModeQuit          = "quit"
)

type Session struct {
	Name      string
	Label     string
	Windows   int
	Attached  bool
	Temporary bool
	Instance  string
	Attention bool
	// VisitedAt is the unix time (seconds) MarkSessionVisited last recorded
	// for this session, or 0 if it has never been visited. AttentionScan
	// compares a window's own last-activity time against this watermark
	// rather than trusting window_activity_flag alone: that flag is only
	// cleared on a window when it is individually selected, so a background
	// (non-active) window's flag can stay set long after the session itself
	// was visited, which would otherwise re-flag stale, pre-visit activity
	// as if it were new.
	VisitedAt int64
}

type Controller interface {
	ListSessions() ([]Session, error)
	// SessionActivityTimestamps reports, per session, the latest
	// window_activity time (unix seconds) across all of that session's
	// windows -- not just its active one.
	SessionActivityTimestamps() (map[string]int64, error)
	SessionAttached(name string) (bool, error)
	CreateSession(name, cwd, command string) (Session, error)
	RenameSession(oldName, newName string) error
	SetSessionProject(name, project string) error
	RunBackground(command string) error
	DisplayMessage(message string) error
	CurrentPaneDir() (string, error)
	SetSessionTemporary(name string, temporary bool, instanceID string) error
	SetSessionLabel(name, label string) error
	SetSessionAttention(name string, attention bool) error
	// MarkSessionVisited clears the attention marker and records the visit
	// time as this session's new activity watermark.
	MarkSessionVisited(name string) error
	SetSessionTopBar(name, content string) error
	AttachCommand(ctx context.Context, name string) (*exec.Cmd, error)
	KillSession(name string) error
	SessionPanesAllDead(name string) (bool, error)
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string, palette Palette) error
	ToggleMenu(binaryPath string) error
	ToggleCommandMenu(binaryPath string) error
	OpenQuit(binaryPath string) error
	CloseMenu() error
	QuitAll() error
	CleanupVolatileSessions(instanceID string) error
	CleanupDetachedClient() error
}

type Runner func(args ...string) (string, error)

type Manager struct {
	Run Runner
}

type Palette struct {
	Surface0 string
	Subtext  string
	Text     string
	Blue     string
	Mantle   string
	Teal     string
	Yellow   string
	Green    string
	Red      string
}

func New() Controller {
	return Manager{Run: Run}
}

func ToggleMenu() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return New().ToggleMenu(exe)
}

func ToggleCommandMenu() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return New().ToggleCommandMenu(exe)
}

func (m Manager) runner() Runner {
	if m.Run != nil {
		return m.Run
	}
	return Run
}

// statusLeft is the global fallback status-left, shown before the first
// per-session FormatTopBar override lands -- styled to match it, with the
// same filled, rounded-cap pill treatment around the project and session
// segments.
func (p Palette) statusLeft() string {
	return "#[bg=" + p.Mantle + ",fg=" + p.Surface0 + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Blue + ",bold] #{@tflow-project} " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]" +
		"  " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Teal + ",bold] #{?@tflow-session-label,#{@tflow-session-label},#S} " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]"
}

func (p Palette) statusStyle() string {
	return fmt.Sprintf("bg=%s,fg=%s", p.Mantle, p.Text)
}

// statusRight shows the COMMAND indicator only while the client is waiting
// in prefixTable for the chord's second key (Ctrl+F held, about to press
// f) -- not once the popup is actually open (commandTable), since the
// popup itself (badge + bordered session list) is already an unambiguous
// visual cue at that point and the pill would be redundant.
func (p Palette) statusRight() string {
	yellow := p.Yellow
	if yellow == "" {
		yellow = "#f9e2af"
	}
	mantle := p.Mantle
	if mantle == "" {
		mantle = "#181825"
	}
	return "#{?#{==:#{client_key_table}," + prefixTable + "},#[fg=" + yellow + "]#[bg=" + mantle + "]#[bg=" + yellow + "]#[fg=" + mantle + "]#[bold] COMMAND #[nobold]#[fg=" + yellow + "]#[bg=" + mantle + "]#[default],}"
}

func (p Palette) FormatTopBar(project string, labels []string, types []string, attentions []bool, activeIndex int) string {
	if len(labels) == 0 {
		return ""
	}
	if activeIndex < 0 || activeIndex >= len(labels) {
		activeIndex = 0
	}
	var b strings.Builder
	b.WriteString(p.projectSection(project))
	for i, label := range labels {
		label = escapeTmuxFormatLiteral(label)
		sessionType := ""
		if i < len(types) {
			sessionType = types[i]
		}
		live := i == activeIndex
		icon := p.sessionTypeIcon(sessionType, live)
		attention := i < len(attentions) && attentions[i]
		if i > 0 {
			b.WriteString("  ")
		}
		if live {
			b.WriteString("#[bg=" + p.Mantle + ",fg=" + p.Surface0 + "]" +
				"#[bg=" + p.Surface0 + ",fg=" + p.Subtext + "] " + icon + " " + label + p.attentionMark(attention) + " " +
				"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]")
		} else {
			b.WriteString("#[bg=" + p.Mantle + ",fg=" + p.Subtext + "]" + icon + " " + label + p.attentionMark(attention))
		}
	}
	return b.String()
}

// attentionMark renders a red marker after a session's label when its
// runtime-only attention flag is set, independent of its type and whether it
// is the active pill -- matching the popup's after-the-label ordering.
func (p Palette) attentionMark(attention bool) string {
	if !attention {
		return ""
	}
	return " #[fg=" + p.Red + "]!#[fg=" + p.Subtext + "]"
}

// sessionTypeIcon renders a session's type icon colored with its accent,
// reverting back to the surrounding subtext color afterward so the label
// that follows keeps its own active/inactive styling untouched. A live
// session (the one this client is currently attached to) always renders
// green regardless of its type, mirroring the popup's liveChipStyle.
func (p Palette) sessionTypeIcon(sessionType string, live bool) string {
	glyph := typeGlyph(sessionType)
	if live {
		return "#[fg=" + p.Green + "]" + glyph + "#[fg=" + p.Subtext + "]"
	}
	color := p.Blue
	switch sessionType {
	case "git":
		color = p.Teal
	case "agent":
		color = p.Yellow
	}
	return "#[fg=" + color + "]" + glyph + "#[fg=" + p.Subtext + "]"
}

// typeGlyph returns the bare icon glyph for a session type, independent of
// any color -- shared by sessionTypeIcon's live and non-live branches.
func typeGlyph(sessionType string) string {
	switch sessionType {
	case "git":
		return "⎇"
	case "agent":
		return "✦"
	default:
		return ">_"
	}
}

// projectSection renders the leading project name and the arrow dividing it
// from the session section. A volatile context has no project and renders an
// empty section rather than nothing, so the bar keeps its shape in every
// context. Plain text, no fill -- matching the popup's flat style language.
// projectSection renders the leading project name as a filled,
// rounded-cap pill (bg=Surface0 against the status line's ambient
// bg=Mantle) and the arrow dividing it from the session section. A
// volatile context has no project and renders an empty pill rather than
// nothing, so the bar keeps its shape in every context.
func (p Palette) projectSection(project string) string {
	project = escapeTmuxFormatLiteral(strings.TrimSpace(project))
	return "#[bg=" + p.Mantle + ",fg=" + p.Surface0 + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Blue + ",bold] " + project + " " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]" +
		"  "
}

func escapeTmuxFormatLiteral(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}
