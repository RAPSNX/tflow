package ui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	tmuxSocket     = "tflow"
	menuMarker     = "@tflow-menu"
	projectMarker  = "@tflow-project"
	tempMarker     = "@tflow-temp"
	menuWidth      = "36"
	menuToggleKey  = "C-f"
	menuCurrentEnv = "TFLOW_CURRENT_SESSION"
)

type session struct {
	Name      string
	Windows   int
	Attached  bool
	Temporary bool
}

type tmuxController interface {
	ListSessions() ([]session, error)
	CreateSession(name, cwd, command string) (session, error)
	SetSessionTemporary(name string, temporary bool) error
	AttachCommand(name string) (*exec.Cmd, error)
	KillSession(name string) error
	RenameSession(oldName, newName string) error
	SwitchClient(name string) error
	EnsureControlMode(binaryPath string) error
	SyncSessionProjects(sessionProjects map[string]string) error
	ToggleMenu(binaryPath string) error
	ClosePane(paneID string) error
	QuitAll(paneID string) error
}

type tmuxRunner func(args ...string) (string, error)

type tmuxSessionManager struct {
	run tmuxRunner
}

func newSessionManager() tmuxController {
	return tmuxSessionManager{run: runTmux}
}

func ToggleMenu() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return newSessionManager().ToggleMenu(exe)
}

func (m tmuxSessionManager) runner() tmuxRunner {
	if m.run != nil {
		return m.run
	}
	return runTmux
}

func (m tmuxSessionManager) ListSessions() ([]session, error) {
	out, err := m.runner()("list-sessions", "-F", "#{session_name}\t#{session_windows}\t#{session_attached}\t#{"+tempMarker+"}")
	if err != nil {
		if isNoTmuxServer(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	sessions := make([]session, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
			continue
		}
		s := session{Name: strings.TrimSpace(parts[0])}
		if len(parts) > 1 {
			fmt.Sscanf(parts[1], "%d", &s.Windows)
		}
		if len(parts) > 2 {
			s.Attached = strings.TrimSpace(parts[2]) == "1"
		}
		if len(parts) > 3 {
			s.Temporary = strings.TrimSpace(parts[3]) == "1"
		}
		sessions = append(sessions, s)
	}
	return sessions, nil
}

func (m tmuxSessionManager) CreateSession(name, cwd, command string) (session, error) {
	name = sanitizeSessionName(name)
	if name == "" {
		return session{}, fmt.Errorf("session name is empty")
	}
	cwd = normalizeCWD(cwd)
	command = strings.TrimSpace(command)

	if _, err := m.runner()("has-session", "-t", name); err == nil {
		return session{Name: name}, nil
	} else if !isNoTmuxSession(err) && !isNoTmuxServer(err) {
		return session{}, err
	}

	args := []string{"new-session", "-d", "-s", name, "-c", cwd}
	if command != "" {
		args = append(args, userShell(), "-lc", command)
	}
	if _, err := m.runner()(args...); err != nil {
		return session{}, err
	}
	if _, err := m.runner()("rename-window", "-t", name+":1", name); err != nil && !strings.Contains(err.Error(), "can't find window") {
		return session{}, err
	}
	return session{Name: name, Windows: 1}, nil
}

func (m tmuxSessionManager) SetSessionTemporary(name string, temporary bool) error {
	value := "off"
	marker := "0"
	if temporary {
		value = "on"
		marker = "1"
	}
	if _, err := m.runner()("set-option", "-t", name, "destroy-unattached", value); err != nil {
		return err
	}
	_, err := m.runner()("set-option", "-t", name, tempMarker, marker)
	return err
}

func (tmuxSessionManager) AttachCommand(name string) (*exec.Cmd, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("session name is empty")
	}
	return exec.Command("tmux", "-L", tmuxSocket, "attach-session", "-t", name), nil
}

func (m tmuxSessionManager) KillSession(name string) error {
	_, err := m.runner()("kill-session", "-t", name)
	if err != nil && isNoTmuxSession(err) {
		return nil
	}
	return err
}

func (m tmuxSessionManager) RenameSession(oldName, newName string) error {
	oldName = sanitizeSessionName(oldName)
	newName = sanitizeSessionName(newName)
	if oldName == "" || newName == "" {
		return fmt.Errorf("session name is empty")
	}
	if oldName == newName {
		return nil
	}
	_, err := m.runner()("rename-session", "-t", oldName, newName)
	return err
}

func (m tmuxSessionManager) SwitchClient(name string) error {
	_, err := m.runner()("switch-client", "-t", name)
	return err
}

func (m tmuxSessionManager) EnsureControlMode(binaryPath string) error {
	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	runShell := "exec " + shellQuote(binaryPath) + " toggle-menu"
	statusLeft := "#[bg=#313244,fg=#a6adc8]" +
		"#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} " +
		"#[bg=#181825,fg=#313244,nobold]" +
		"  #[bg=#313244,fg=#a6adc8]" +
		"#[bg=#313244,fg=#cdd6f4,bold] section #[fg=#94e2d5]#S " +
		"#[bg=#181825,fg=#313244,nobold]"
	commands := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "status-left-length", "120"},
		{"set-option", "-g", "status-right-length", "0"},
		{"set-option", "-g", "status-left", statusLeft},
		{"set-option", "-g", "status-right", ""},
		{"set-option", "-g", "window-status-separator", ""},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
		{"set-option", "-g", "detach-on-destroy", "off"},
		{"set-option", "-g", "default-shell", userShell()},
		{"set-option", "-g", "default-command", loginShellCommand()},
		{"bind-key", "-n", menuToggleKey, "run-shell", runShell},
	}
	for _, args := range commands {
		if _, err := m.runner()(args...); err != nil && !isNoTmuxServer(err) {
			return err
		}
	}
	return nil
}

func (m tmuxSessionManager) SyncSessionProjects(sessionProjects map[string]string) error {
	for name, project := range sessionProjects {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		project = normalizeProjectName(project)
		if project == "" {
			project = defaultProjectName
		}
		if _, err := m.runner()("set-option", "-t", name, projectMarker, project); err != nil {
			if isNoTmuxSession(err) || isNoTmuxServer(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (m tmuxSessionManager) ToggleMenu(binaryPath string) error {
	windowID, err := m.currentValue("#{window_id}")
	if err != nil {
		return err
	}

	if paneID, ok, err := m.menuPane(windowID); err != nil {
		return err
	} else if ok {
		return m.ClosePane(paneID)
	}

	if strings.TrimSpace(binaryPath) == "" {
		return fmt.Errorf("tflow binary path is empty")
	}

	currentSession, err := m.currentValue("#{session_name}")
	if err != nil {
		return err
	}

	menuCommand := fmt.Sprintf("%s=%s exec %s menu", menuCurrentEnv, shellQuote(currentSession), shellQuote(binaryPath))
	paneID, err := m.runner()("split-window", "-t", windowID, "-h", "-b", "-l", menuWidth, "-P", "-F", "#{pane_id}", menuCommand)
	if err != nil {
		return err
	}
	paneID = strings.TrimSpace(paneID)
	if paneID == "" {
		return fmt.Errorf("tmux did not return a menu pane id")
	}
	if _, err := m.runner()("set-option", "-p", "-t", paneID, menuMarker, "1"); err != nil {
		return err
	}
	return nil
}

func (m tmuxSessionManager) ClosePane(paneID string) error {
	if strings.TrimSpace(paneID) == "" {
		return nil
	}
	_, err := m.runner()("kill-pane", "-t", paneID)
	return err
}

func (m tmuxSessionManager) QuitAll(paneID string) error {
	script := []string{}
	if trimmed := strings.TrimSpace(paneID); trimmed != "" {
		script = append(script, "tmux -L "+shellQuote(tmuxSocket)+" kill-pane -t "+shellQuote(trimmed)+" >/dev/null 2>&1")
	}
	script = append(script, "tmux -L "+shellQuote(tmuxSocket)+" detach-client >/dev/null 2>&1")
	_, err := m.runner()("run-shell", strings.Join(script, "; "))
	return err
}

func (m tmuxSessionManager) currentValue(format string) (string, error) {
	out, err := m.runner()("display-message", "-p", format)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (m tmuxSessionManager) menuPane(windowID string) (string, bool, error) {
	out, err := m.runner()("list-panes", "-t", windowID, "-F", "#{pane_id}\t#{"+menuMarker+"}")
	if err != nil {
		return "", false, err
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.Split(line, "\t")
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "1" {
			return strings.TrimSpace(parts[0]), true, nil
		}
	}
	return "", false, nil
}

func normalizeCWD(cwd string) string {
	if strings.TrimSpace(cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	cwd = expandHomeDir(cwd)
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

func expandHomeDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	if len(path) > 1 && path[1] != '/' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

func sanitizeSessionName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var b strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func runTmux(args ...string) (string, error) {
	cmd := exec.Command("tmux", append([]string{"-L", tmuxSocket}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func isNoTmuxServer(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no server running") ||
		(strings.Contains(msg, "error connecting to ") && strings.Contains(msg, "No such file or directory"))
}

func isNoTmuxSession(err error) bool {
	return err != nil && strings.Contains(err.Error(), "can't find session")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func userShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return "/bin/sh"
	}
	return shell
}

func loginShellCommand() string {
	return "exec " + shellQuote(userShell()) + " -l"
}
