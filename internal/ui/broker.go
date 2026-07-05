package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultSessionName  = "default"
	zellijDefaultLayout = "compact"
	zellijSetupWait     = 2 * time.Second
)

type session struct {
	Name     string
	Windows  int
	Attached bool
}

type sessionManager interface {
	ListSessions() ([]session, error)
	CreateSession(name, cwd, project string) (session, error)
	KillSession(name string) error
	SetSessionProject(name, project string) error
}

type sessionBroker struct{}

type zellijTab struct {
	Name  string `json:"name"`
	TabID int    `json:"tab_id"`
}

func newSessionManager() sessionManager {
	return newSessionBroker()
}

func newSessionBroker() *sessionBroker {
	return &sessionBroker{}
}

func (b *sessionBroker) ListSessions() ([]session, error) {
	output, err := runZellijOutput("list-sessions", "--short", "--no-formatting")
	if err != nil {
		if strings.Contains(output, "No active zellij sessions found.") {
			return nil, nil
		}
		return nil, commandError(err, output)
	}

	current := currentZellijSession()
	names := parseSessionNames(output)
	sessions := make([]session, 0, len(names))
	for _, name := range names {
		sessions = append(sessions, session{
			Name:     name,
			Windows:  1,
			Attached: name == current,
		})
	}
	return sessions, nil
}

func (b *sessionBroker) CreateSession(name, cwd, project string) (session, error) {
	name = sanitizeSessionName(name)
	if name == "" {
		return session{}, fmt.Errorf("session name is empty")
	}
	project = normalizeProjectName(project)
	if project == "" {
		project = defaultProjectName
	}
	cwd = normalizeCWD(cwd)

	sessions, err := b.ListSessions()
	if err != nil {
		return session{}, err
	}
	for _, existing := range sessions {
		if existing.Name == name {
			if err := b.SetSessionProject(name, project); err != nil {
				return session{}, err
			}
			existing.Attached = existing.Name == currentZellijSession()
			return existing, nil
		}
	}

	args := []string{
		"attach", "-b", "-c", name, "options",
		"--default-cwd", cwd,
		"--default-layout", zellijDefaultLayout,
		"--pane-frames", "false",
		"--show-startup-tips", "false",
		"--show-release-notes", "false",
		"--simplified-ui", "true",
		"--mouse-mode", "false",
	}
	if output, err := runZellijOutput(args...); err != nil {
		return session{}, commandError(err, output)
	}
	if err := waitForSession(name, zellijSetupWait); err != nil {
		return session{}, err
	}
	if err := b.SetSessionProject(name, project); err != nil {
		return session{}, err
	}
	return session{Name: name, Windows: 1, Attached: name == currentZellijSession()}, nil
}

func (b *sessionBroker) KillSession(name string) error {
	name = sanitizeSessionName(name)
	if name == "" {
		return nil
	}
	output, err := runZellijOutput("kill-session", name)
	if err != nil {
		if strings.Contains(output, "No session named") {
			return nil
		}
		return commandError(err, output)
	}
	return nil
}

func (b *sessionBroker) SetSessionProject(name, project string) error {
	name = sanitizeSessionName(name)
	project = normalizeProjectName(project)
	if name == "" {
		return fmt.Errorf("session name is empty")
	}
	if project == "" {
		project = defaultProjectName
	}

	tabs, err := listSessionTabs(name)
	if err != nil {
		return err
	}
	if len(tabs) == 0 {
		return fmt.Errorf("session %q has no tabs", name)
	}
	if tabs[0].Name == project {
		return nil
	}

	output, err := runZellijOutput("--session", name, "action", "rename-tab-by-id", fmt.Sprint(tabs[0].TabID), project)
	if err != nil {
		return commandError(err, output)
	}
	return nil
}

func (b *sessionBroker) AttachSession(name, cwd string) error {
	name = sanitizeSessionName(name)
	if name == "" {
		return fmt.Errorf("session name is empty")
	}
	cwd = normalizeCWD(cwd)

	if currentZellijSession() != "" {
		if currentZellijSession() == name {
			return nil
		}
		return runInteractiveZellij(
			"action", "switch-session", name,
			"--cwd", cwd,
		)
	}

	return runInteractiveZellij(
		"attach", name, "options",
		"--default-cwd", cwd,
		"--default-layout", zellijDefaultLayout,
		"--pane-frames", "false",
		"--show-startup-tips", "false",
		"--show-release-notes", "false",
		"--simplified-ui", "true",
		"--mouse-mode", "false",
	)
}

func Start() error {
	broker := newSessionBroker()
	if _, err := broker.CreateSession(defaultSessionName, defaultSessionDir(), defaultProjectName); err != nil {
		return err
	}
	if err := ensureDefaultStartupState(); err != nil {
		return err
	}
	return broker.AttachSession(defaultSessionName, defaultSessionDir())
}

func RunMenu(current string) (string, error) {
	return runMenu(newSessionManager(), current)
}

func OpenMenu() error {
	current := currentZellijSession()
	selected, err := RunMenu(current)
	if err != nil {
		return err
	}
	if selected == "" {
		return nil
	}

	broker := newSessionBroker()
	return broker.AttachSession(selected, defaultSessionDir())
}

func defaultSessionDir() string {
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home
	}
	if cwd, err := os.Getwd(); err == nil && strings.TrimSpace(cwd) != "" {
		return cwd
	}
	return "."
}

func listSessionTabs(name string) ([]zellijTab, error) {
	output, err := runZellijOutput("--session", name, "action", "list-tabs", "--json")
	if err != nil {
		return nil, commandError(err, output)
	}
	var tabs []zellijTab
	if err := json.Unmarshal([]byte(output), &tabs); err != nil {
		return nil, err
	}
	slices.SortFunc(tabs, func(a, b zellijTab) int {
		return a.TabID - b.TabID
	})
	return tabs, nil
}

func waitForSession(name string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tabs, err := listSessionTabs(name)
		if err == nil && len(tabs) > 0 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("session %q did not become ready", name)
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
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

func currentZellijSession() string {
	for _, key := range []string{"ZELLIJ_SESSION_NAME", "ZELLIJ_SESSION"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return sanitizeSessionName(value)
		}
	}
	return ""
}

func runZellijOutput(args ...string) (string, error) {
	cmd, err := zellijCommand(args...)
	if err != nil {
		return "", err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n"
		}
		output += strings.TrimSpace(stderr.String())
	}
	return output, err
}

func runInteractiveZellij(args ...string) error {
	cmd, err := zellijCommand(args...)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func zellijCommand(args ...string) (*exec.Cmd, error) {
	allArgs := make([]string, 0, len(args)+2)
	configPath, err := ensureZellijConfig()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(configPath) != "" {
		allArgs = append(allArgs, "--config", configPath)
	}
	allArgs = append(allArgs, args...)
	return exec.Command("zellij", allArgs...), nil
}

func ensureZellijConfig() (string, error) {
	configPath := zellijConfigPath()
	launcherPath := zellijLauncherPath()
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(launcherPath, []byte(zellijLauncherScript()), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, []byte(zellijConfigContents(launcherPath)), 0o644); err != nil {
		return "", err
	}
	return configPath, nil
}

func zellijConfigPath() string {
	return filepath.Join(filepath.Dir(appStatePath()), "zellij.kdl")
}

func zellijLauncherPath() string {
	return filepath.Join(filepath.Dir(appStatePath()), "open-menu")
}

func zellijLauncherScript() string {
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	var lines []string
	lines = append(lines, "#!/bin/sh", "set -eu")
	if strings.TrimSpace(executable) != "" {
		lines = append(lines, "if [ -x "+shellQuote(executable)+" ]; then")
		lines = append(lines, "  exec "+shellQuote(executable)+" menu")
		lines = append(lines, "fi")
	}
	lines = append(lines, "exec tflow menu", "")
	return strings.Join(lines, "\n")
}

func zellijConfigContents(launcherPath string) string {
	runAction := fmt.Sprintf(
		"Run %s {\n                floating true\n                close_on_exit true\n            }\n            SwitchToMode \"Normal\"",
		strconv.Quote(launcherPath),
	)
	return strings.Join([]string{
		"keybinds {",
		"    shared_except \"locked\" \"scroll\" \"search\" {",
		"        bind \"Ctrl f\" {",
		"            " + runAction,
		"        }",
		"    }",
		"    shared_among \"scroll\" \"search\" {",
		"        bind \"Ctrl f\" {",
		"            " + runAction,
		"        }",
		"    }",
		"}",
		"",
	}, "\n")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func commandError(err error, output string) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && strings.TrimSpace(output) != "" {
		return fmt.Errorf("%s", strings.TrimSpace(output))
	}
	if strings.TrimSpace(output) != "" {
		return fmt.Errorf("%s: %w", strings.TrimSpace(output), err)
	}
	return err
}

func parseSessionNames(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		names = append(names, fields[0])
	}
	slices.Sort(names)
	return names
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
