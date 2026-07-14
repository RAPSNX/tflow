package ui

import (
	"fmt"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDefaultSessionDirPrefersHome(t *testing.T) {
	oldHome := os.Getenv("HOME")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
	})
	_ = os.Setenv("HOME", "/tmp/home")

	if got := defaultSessionDir(); got != "/tmp/home" {
		t.Fatalf("defaultSessionDir = %q, want /tmp/home", got)
	}
}

func TestCreateSessionUsesCurrentDirectory(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			if command != "" {
				t.Fatalf("command = %q, want empty", command)
			}
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != m.cwd {
		t.Fatalf("cwd = %q, want %q", gotCWD, m.cwd)
	}
}

func TestCreateSessionUsesProjectDirectoryWhenConfigured(t *testing.T) {
	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/project-small"}}
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != "/tmp/project-small" {
		t.Fatalf("cwd = %q, want /tmp/project-small", gotCWD)
	}
}

func TestCreateSessionUsesExpandedHomeDirectoryWhenConfigured(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	var gotCWD string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "~/project-small"}}
	m.mode = inputCreateSession
	m.input.SetValue("dev")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCWD != "/tmp/home/project-small" {
		t.Fatalf("cwd = %q, want /tmp/home/project-small", gotCWD)
	}
}

func TestCreateK9sSessionUsesClusterPath(t *testing.T) {
	var gotCWD string
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCWD = cwd
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Workdir: "/tmp/project-small", Cluster: clusterConfig{Path: "/tmp/kubeconfig"}},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindK9s
	m.input.SetValue("k9s")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "export KUBECONFIG='/tmp/kubeconfig'; exec k9s" {
		t.Fatalf("command = %q", gotCommand)
	}
	if gotCWD != "/tmp/project-small" {
		t.Fatalf("cwd = %q, want /tmp/project-small", gotCWD)
	}
}

func TestCreateK9sSessionUsesConnectionCommand(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", Cluster: clusterConfig{ConnectionCmd: "aws eks update-kubeconfig --name prod"}},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindK9s
	m.input.SetValue("k9s")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "aws eks update-kubeconfig --name prod && exec k9s" {
		t.Fatalf("command = %q", gotCommand)
	}
}

func TestCreateAgentSessionUsesConfiguredAgentBinary(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.projectConfigs = map[string]projectConfig{
		"small": {Name: "small", AgentBinary: "aider"},
	}
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindAgent
	m.input.SetValue("agent")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "exec 'aider'" {
		t.Fatalf("command = %q", gotCommand)
	}
	if msg.kind != sessionTypeAgent {
		t.Fatalf("kind = %q, want %q", msg.kind, sessionTypeAgent)
	}
}

func TestCreateAgentSessionDefaultsToCodexBinary(t *testing.T) {
	var gotCommand string
	m := newModel(fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			gotCommand = command
			return session{Name: name, Windows: 1}, nil
		},
	}, "", "").(model)
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.createSessionKind = sessionKindAgent
	m.input.SetValue("agent")

	_, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected create command")
	}
	msg := cmd().(sessionCreatedMsg)
	if msg.err != nil {
		t.Fatalf("sessionCreatedMsg.err = %v", msg.err)
	}
	if gotCommand != "exec 'codex'" {
		t.Fatalf("command = %q", gotCommand)
	}
}

func TestProjectEditorCommandSplitsEditorArgs(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")

	cmd, err := projectEditorCommand("/tmp/project.yaml")
	if err != nil {
		t.Fatalf("projectEditorCommand returned error: %v", err)
	}
	if got, want := fmt.Sprint(cmd.Args), fmt.Sprint([]string{"code", "--wait", "/tmp/project.yaml"}); got != want {
		t.Fatalf("args = %s, want %s", got, want)
	}
}

func TestSetProjectDirectoryPersistsValue(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(fakeTmuxController{}, "", "").(model)
	m.statePath = tmp + "/state.json"
	m.projects = []string{defaultProjectName, "small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small"}}
	m.selectedProject = "small"
	m.mode = inputSetProjectDir
	m.input.SetValue("/tmp/small-project")

	updated, cmd := m.commitProjectDir()
	got := *(updated.(*model))
	if cmd != nil {
		t.Fatal("expected no command")
	}
	if got.projectConfig("small").Workdir != "/tmp/small-project" {
		t.Fatalf("workdir = %q, want /tmp/small-project", got.projectConfig("small").Workdir)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "prod-main-01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestProjectNormalizationPreservesOrderAndDeduplicates(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"small", "default", "alpha"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}
