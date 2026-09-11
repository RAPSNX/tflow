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
	// The popup renders as one wide, short horizontal strip (badge + inline
	// session pills, mirroring the top bar's own row-of-pills shape) rather
	// than a tall vertical list, so it is wide and short to match -- not
	// narrow and tall.
	menuWidth = "70%"
	// menuHeight stays below 100% so the popup fits under the status line.
	// tmux resolves a popup that would overflow by moving it back up rather
	// than shrinking it, so a full-height popup lands on the status line and
	// hides the top bar. Percentages are floored, so this always leaves a row.
	menuHeight         = "60%"
	commandKey         = "C-Space"
	commandTable       = "tflow-command"
	quitKey            = "C-q"
	CurrentSessionEnv  = "TFLOW_CURRENT_SESSION"
	CurrentClientEnv   = "TFLOW_CURRENT_CLIENT"
	CurrentInstanceEnv = "TFLOW_INSTANCE_ID"
	MenuModeEnv        = "TFLOW_MENU_MODE"
	MenuModeCommand    = "command"
	MenuModeQuit       = "quit"
)

type Session struct {
	Name      string
	Label     string
	Windows   int
	Attached  bool
	Temporary bool
	Instance  string
	Attention bool
}

type Controller interface {
	ListSessions() ([]Session, error)
	WindowActivityBySession() (map[string]bool, error)
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

func (p Palette) statusLeft() string {
	return "#[bg=" + p.Surface0 + ",fg=" + p.Subtext + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Text + ",bold] project #[fg=" + p.Blue + "]#{@tflow-project} " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]" +
		"  #[bg=" + p.Surface0 + ",fg=" + p.Subtext + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Text + ",bold] session #[fg=" + p.Teal + "]#{?@tflow-session-label,#{@tflow-session-label},#S} " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]"
}

func (p Palette) statusStyle() string {
	return fmt.Sprintf("bg=%s,fg=%s", p.Mantle, p.Text)
}

func (p Palette) statusRight() string {
	yellow := p.Yellow
	if yellow == "" {
		yellow = "#f9e2af"
	}
	mantle := p.Mantle
	if mantle == "" {
		mantle = "#181825"
	}
	return "#{?#{==:#{client_key_table}," + commandTable + "},#[fg=" + yellow + "]#[bg=" + mantle + "]#[bg=" + yellow + "]#[fg=" + mantle + "]#[bold] COMMAND #[nobold]#[fg=" + yellow + "]#[bg=" + mantle + "]#[default],}"
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
		icon := p.sessionTypeIcon(sessionType)
		attention := i < len(attentions) && attentions[i]
		if i > 0 {
			b.WriteString("  ")
		}
		if i == activeIndex {
			b.WriteString("#[bg=" + p.Surface0 + ",fg=" + p.Subtext + "]" +
				"#[bg=" + p.Surface0 + ",fg=" + p.Text + ",bold] " + icon + " " + p.attentionMark(attention) + label + " " +
				"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]")
		} else {
			b.WriteString("#[bg=" + p.Mantle + ",fg=" + p.Subtext + "]" + icon + " " + p.attentionMark(attention) + label)
		}
	}
	return b.String()
}

// attentionMark renders a red marker ahead of a session's label when its
// runtime-only attention flag is set, independent of its type and whether it
// is the active pill.
func (p Palette) attentionMark(attention bool) string {
	if !attention {
		return ""
	}
	return "#[fg=" + p.Red + "]!#[fg=" + p.Subtext + "] "
}

// sessionTypeIcon renders a session's type icon colored with its accent,
// reverting back to the surrounding subtext color afterward so the label
// that follows keeps its own active/inactive styling untouched.
func (p Palette) sessionTypeIcon(sessionType string) string {
	switch sessionType {
	case "git":
		return "#[fg=" + p.Teal + "]⎇#[fg=" + p.Subtext + "]"
	case "agent":
		return "#[fg=" + p.Yellow + "]✦#[fg=" + p.Subtext + "]"
	default:
		return "#[fg=" + p.Blue + "]>_#[fg=" + p.Subtext + "]"
	}
}

// projectSection renders the leading project pill and the arrow dividing it
// from the session section. A volatile context has no project and renders an
// empty pill rather than nothing, so the bar keeps its shape in every context.
func (p Palette) projectSection(project string) string {
	project = escapeTmuxFormatLiteral(strings.TrimSpace(project))
	return "#[bg=" + p.Mantle + ",fg=" + p.Surface0 + "]" +
		"#[bg=" + p.Surface0 + ",fg=" + p.Blue + ",bold] " + project + " " +
		"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]" +
		"  "
}

func escapeTmuxFormatLiteral(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}
