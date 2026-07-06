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

	parsed, err := parseProjectConfig(marshalProjectConfig(original))
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
		t.Fatalf("agent-binary = %q, want %q", parsed.AgentBinary, original.AgentBinary)
	}
	if !parsed.Protect {
		t.Fatal("expected protect to round-trip")
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

func TestLoadProjectConfigsUsesConfiguredProjectsDir(t *testing.T) {
	baseDir := t.TempDir()
	projectsDir := filepath.Join(baseDir, "custom-projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, "config.yaml"), []byte(strings.Join([]string{
		`projects-dir: ` + yamlString(projectsDir),
		`theme: "catppuccin"`,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectsDir, "small.yaml"), marshalProjectConfig(projectConfig{Name: "small"}), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	statePath := filepath.Join(baseDir, "state.json")
	cfgs, err := loadProjectConfigs(statePath, appState{Projects: []projectState{{Name: "garden"}}})
	if err != nil {
		t.Fatalf("loadProjectConfigs returned error: %v", err)
	}
	if _, ok := cfgs["small"]; !ok {
		t.Fatalf("expected configured project to load from %s", projectsDir)
	}
	if _, ok := cfgs["garden"]; !ok {
		t.Fatalf("expected state project to be present")
	}
}
