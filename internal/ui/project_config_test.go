package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectConfigRoundTrip(t *testing.T) {
	original := projectConfig{
		Name:        "small",
		Workdir:     "/tmp/project",
		AgentBinary: "aider",
		Protect:     true,
		Cluster: clusterConfig{
			ConnectionCmd: "aws eks update-kubeconfig --name prod",
		},
	}

	data := marshalProjectConfig(original)
	if !strings.Contains(string(data), `agent-cmd: "aider"`) {
		t.Fatalf("marshaled config = %q, want agent-cmd", string(data))
	}
	if strings.Contains(string(data), "agent-binary") {
		t.Fatalf("marshaled config = %q, want no agent-binary", string(data))
	}

	parsed, err := parseProjectConfig(data)
	if err != nil {
		t.Fatalf("parseProjectConfig returned error: %v", err)
	}

	if parsed.Name != original.Name {
		t.Fatalf("name = %q, want %q", parsed.Name, original.Name)
	}
	if parsed.Workdir != original.Workdir {
		t.Fatalf("workdir = %q, want %q", parsed.Workdir, original.Workdir)
	}
	if parsed.Cluster.ConnectionCmd != original.Cluster.ConnectionCmd {
		t.Fatalf("connection-cmd = %q, want %q", parsed.Cluster.ConnectionCmd, original.Cluster.ConnectionCmd)
	}
	if parsed.AgentBinary != original.AgentBinary {
		t.Fatalf("agent-cmd = %q, want %q", parsed.AgentBinary, original.AgentBinary)
	}
	if !parsed.Protect {
		t.Fatal("expected protect to round-trip")
	}
}

func TestParseProjectConfigAcceptsLegacyAgentBinary(t *testing.T) {
	cfg, err := parseProjectConfig([]byte(strings.Join([]string{
		`name: "small"`,
		`agent-binary: "aider"`,
	}, "\n")))
	if err != nil {
		t.Fatalf("parseProjectConfig returned error: %v", err)
	}
	if cfg.AgentBinary != "aider" {
		t.Fatalf("agent-cmd = %q, want aider", cfg.AgentBinary)
	}
}

func TestParseProjectConfigAcceptsClusterPathShorthand(t *testing.T) {
	cfg, err := parseProjectConfig([]byte(strings.Join([]string{
		`name: "small"`,
		`cluster: "/tmp/kubeconfig"`,
	}, "\n")))
	if err != nil {
		t.Fatalf("parseProjectConfig returned error: %v", err)
	}
	if cfg.Cluster.Path != "/tmp/kubeconfig" {
		t.Fatalf("cluster path = %q", cfg.Cluster.Path)
	}
}

func TestLoadProjectConfigsUsesAppConfigProjects(t *testing.T) {
	baseDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(baseDir, "config.yaml"), []byte(strings.Join([]string{
		`projects:`,
		`  - name: "small"`,
		`    workdir: "/tmp/project"`,
		`    agent-cmd: "aider"`,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfgs, err := loadProjectConfigs(filepath.Join(baseDir, "state.json"), appState{Projects: []projectState{{Name: "garden"}}})
	if err != nil {
		t.Fatalf("loadProjectConfigs returned error: %v", err)
	}
	if _, ok := cfgs["garden"]; ok {
		t.Fatalf("state project leaked into persistent config: %#v", cfgs)
	}
	cfg := cfgs["small"]
	if cfg.Name != "small" || cfg.Workdir != "/tmp/project" || cfg.AgentBinary != "aider" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestSaveProjectConfigsWritesAppConfigProjects(t *testing.T) {
	baseDir := t.TempDir()
	statePath := filepath.Join(baseDir, "state.json")
	configs := map[string]projectConfig{
		"small": {Name: "small", Workdir: "/tmp/project", AgentBinary: "aider"},
	}
	if err := saveProjectConfigs(statePath, configs); err != nil {
		t.Fatalf("saveProjectConfigs returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(baseDir, "config.yaml"))
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "projects:") || !strings.Contains(text, `agent-cmd: "aider"`) {
		t.Fatalf("config.yaml = %q, want projects with agent-cmd", text)
	}
}
