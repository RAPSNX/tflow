package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMenuStartsWithCurrentSessionSelected(t *testing.T) {
	m := NewMenu().(model)
	m.currentSession = "dev"
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true}
	m.syncSelection()

	if m.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", m.selectedSession)
	}
}

func TestTreeRowsContainProjectAndSessionChildren(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}, {Name: "ops"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName, "ops": "small"}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": false}

	rows := m.treeRows()
	if len(rows) != 3 {
		t.Fatalf("len(rows) = %d, want 3", len(rows))
	}
	if rows[0].kind != rowProject || rows[1].kind != rowSession || rows[2].kind != rowProject {
		t.Fatalf("rows = %#v", rows)
	}
}

func TestCollapseSelectionMovesSessionFocusToProject(t *testing.T) {
	m := NewMenu().(model)
	m.projects = []string{defaultProjectName, "small"}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": "small"}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true}
	m.selectedProject = "small"
	m.selectedSession = "dev"
	m.collapseSelection()

	if m.selectedProject != "small" || m.selectedSession != "" {
		t.Fatalf("selectedProject=%q selectedSession=%q", m.selectedProject, m.selectedSession)
	}
}

func TestMoveProjectUsesIncrementalPrefix(t *testing.T) {
	m := NewMenu().(model)
	m.sessions = []session{{Name: "dev"}}
	m.projects = []string{defaultProjectName, "small", "storage"}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.expandedProjects = map[string]bool{defaultProjectName: true, "small": true, "storage": true}
	m.selectedProject = defaultProjectName
	m.selectedSession = "dev"
	m.mode = inputMoveProject
	m.moveQuery = "sm"

	updated, cmd := m.resolveMoveProject()
	got := updated.(model)
	if got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	if cmd == nil {
		t.Fatal("expected move command")
	}
	msg := cmd().(sessionMovedMsg)
	if msg.project != "small" || msg.session != "dev" {
		t.Fatalf("msg = %#v", msg)
	}
}

func TestCtrlCEscapesMenuByClosingPane(t *testing.T) {
	m := NewMenu().(model)
	m.paneID = "%3"
	closed := ""
	m.tmux = fakeSessionManager{
		closePane: func(paneID string) error {
			closed = paneID
			return nil
		},
	}

	_, cmd := m.updateNormal(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected close command")
	}
	msg := cmd().(menuActionMsg)
	if msg.err != nil {
		t.Fatalf("close returned error: %v", msg.err)
	}
	if closed != "%3" {
		t.Fatalf("closed = %q, want %%3", closed)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "prod-main-01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("it's"), `'it'"'"'s'`; got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}

func TestAttachCommandUsesTflowSocket(t *testing.T) {
	cmd, err := tmuxSessionManager{}.AttachCommand("dev")
	if err != nil {
		t.Fatalf("AttachCommand returned error: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-L tflow") {
		t.Fatalf("attach command = %q, want tflow socket", got)
	}
}

func TestEnsureControlModeBindsToggleKey(t *testing.T) {
	oldShell := os.Getenv("SHELL")
	t.Cleanup(func() {
		_ = os.Setenv("SHELL", oldShell)
	})
	_ = os.Setenv("SHELL", "/bin/zsh")

	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.EnsureControlMode("/tmp/tflow"); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-g", "default-shell", "/bin/zsh"},
		{"set-option", "-g", "default-command", "exec '/bin/zsh' -l"},
		{"bind-key", "-n", "C-f", "run-shell", "exec '/tmp/tflow' toggle-menu"},
	}
	for _, want := range wants {
		found := false
		for _, call := range calls {
			if strings.Join(call, "\x00") == strings.Join(want, "\x00") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing call %v in %#v", want, calls)
		}
	}
}

func TestToggleMenuKillsExistingPane(t *testing.T) {
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				return "@1", nil
			case "list-panes":
				return "%5\t1\n", nil
			case "kill-pane":
				if args[2] != "%5" {
					t.Fatalf("kill-pane target = %q", args[2])
				}
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}
}

func TestProjectNormalizationKeepsDefaultFirst(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"default", "alpha", "small"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}

type fakeSessionManager struct {
	listSessions      func() ([]session, error)
	createSession     func(name, cwd string) (session, error)
	attachCommand     func(name string) (*exec.Cmd, error)
	killSession       func(name string) error
	switchClient      func(name string) error
	ensureControlMode func(binaryPath string) error
	toggleMenu        func(binaryPath string) error
	closePane         func(paneID string) error
}

func (f fakeSessionManager) ListSessions() ([]session, error) {
	if f.listSessions != nil {
		return f.listSessions()
	}
	return nil, nil
}

func (f fakeSessionManager) CreateSession(name, cwd string) (session, error) {
	if f.createSession != nil {
		return f.createSession(name, cwd)
	}
	return session{Name: name, Windows: 1}, nil
}

func (f fakeSessionManager) AttachCommand(name string) (*exec.Cmd, error) {
	if f.attachCommand != nil {
		return f.attachCommand(name)
	}
	return exec.Command("sh", "-lc", ":"), nil
}

func (f fakeSessionManager) KillSession(name string) error {
	if f.killSession != nil {
		return f.killSession(name)
	}
	return nil
}

func (f fakeSessionManager) SwitchClient(name string) error {
	if f.switchClient != nil {
		return f.switchClient(name)
	}
	return nil
}

func (f fakeSessionManager) EnsureControlMode(binaryPath string) error {
	if f.ensureControlMode != nil {
		return f.ensureControlMode(binaryPath)
	}
	return nil
}

func (f fakeSessionManager) ToggleMenu(binaryPath string) error {
	if f.toggleMenu != nil {
		return f.toggleMenu(binaryPath)
	}
	return nil
}

func (f fakeSessionManager) ClosePane(paneID string) error {
	if f.closePane != nil {
		return f.closePane(paneID)
	}
	return nil
}
