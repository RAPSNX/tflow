package tmux

import (
	"fmt"
	"os"
	"os/exec"
)

const (
	socketName         = "tflow"
	menuPopupEnvPrefix = "TFLOW_MENU_POPUP_"
	menuInstancePrefix = "TFLOW_MENU_INSTANCE_"
	projectMarker      = "@tflow-project"
	sessionLabelMarker = "@tflow-session-label"
	tempMarker         = "@tflow-temp"
	instanceMarker     = "@tflow-instance"
	menuWidth          = "36"
	menuHeight         = "100%"
	menuToggleKey      = "C-f"
	CurrentSessionEnv  = "TFLOW_CURRENT_SESSION"
	CurrentClientEnv   = "TFLOW_CURRENT_CLIENT"
	CurrentInstanceEnv = "TFLOW_INSTANCE_ID"
)

type Session struct {
	Name      string
	Windows   int
	Attached  bool
	Temporary bool
	Instance  string
}

type Controller interface {
	ListSessions() ([]Session, error)
	CreateSession(name, cwd, command string) (Session, error)
	SetSessionTemporary(name string, temporary bool, instanceID string) error
	AttachCommand(name string) (*exec.Cmd, error)
	KillSession(name string) error
	RenameSession(oldName, newName string) error
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string, palette Palette) error
	SyncSessionProjects(sessionProjects map[string]string) error
	ToggleMenu(binaryPath string) error
	CloseMenu() error
	QuitAll() error
	CleanupVolatileSessions(instanceID string) error
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
