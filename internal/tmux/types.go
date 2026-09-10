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
	menuWidth             = "36"
	menuHeight            = "100%"
	commandKey            = "C-Space"
	commandTable          = "tflow-command"
	quitKey               = "C-q"
	CurrentSessionEnv     = "TFLOW_CURRENT_SESSION"
	CurrentClientEnv      = "TFLOW_CURRENT_CLIENT"
	CurrentInstanceEnv    = "TFLOW_INSTANCE_ID"
	MenuModeEnv           = "TFLOW_MENU_MODE"
	MenuModeQuit          = "quit"
)

type Session struct {
	Name      string
	Label     string
	Windows   int
	Attached  bool
	Temporary bool
	Instance  string
}

type Controller interface {
	ListSessions() ([]Session, error)
	CreateSession(name, cwd, command string) (Session, error)
	RenameSession(oldName, newName string) error
	SetSessionProject(name, project string) error
	RunBackground(command string) error
	DisplayMessage(message string) error
	CurrentPaneDir() (string, error)
	SetSessionTemporary(name string, temporary bool, instanceID string) error
	SetSessionLabel(name, label string) error
	SetSessionTopBar(name, content string) error
	AttachCommand(ctx context.Context, name string) (*exec.Cmd, error)
	KillSession(name string) error
	SessionPanesAllDead(name string) (bool, error)
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string, palette Palette) error
	ToggleMenu(binaryPath string) error
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

func (p Palette) FormatTopBar(labels []string, activeIndex int) string {
	if len(labels) == 0 {
		return ""
	}
	if activeIndex < 0 || activeIndex >= len(labels) {
		activeIndex = 0
	}
	var b strings.Builder
	for i, label := range labels {
		label = escapeTmuxFormatLiteral(label)
		if i == 0 && i != activeIndex {
			b.WriteString("#[bg=" + p.Mantle + ",fg=" + p.Subtext + "] ")
		} else if i > 0 {
			b.WriteString("  ")
		}
		if i == activeIndex {
			b.WriteString("#[bg=" + p.Surface0 + ",fg=" + p.Subtext + "]" +
				"#[bg=" + p.Surface0 + ",fg=" + p.Text + ",bold] " + label + " " +
				"#[bg=" + p.Mantle + ",fg=" + p.Surface0 + ",nobold]")
		} else {
			b.WriteString("#[bg=" + p.Mantle + ",fg=" + p.Subtext + "]" + label)
		}
	}
	return b.String()
}

func escapeTmuxFormatLiteral(value string) string {
	return strings.ReplaceAll(value, "#", "##")
}
