package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectConfigRoundTrip(t *testing.T) {
	original := ProjectConfig{
		Name:        "small",
		Workdir:     "/tmp/project",
		AgentBinary: "aider",
		Protect:     true,
		Cluster: ClusterConfig{
			ConnectionCmd: "aws eks update-kubeconfig --name prod",
		},
	}

	parsed, err := ParseProjectConfig(MarshalProjectConfig(original))
	if err != nil {
		t.Fatalf("ParseProjectConfig returned error: %v", err)
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
	cfg, err := ParseProjectConfig([]byte(strings.Join([]string{
		`name: "small"`,
		`cluster: "/tmp/kubeconfig"`,
	}, "\n")))
	if err != nil {
		t.Fatalf("ParseProjectConfig returned error: %v", err)
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
		`projects-dir: ` + YAMLString(projectsDir),
		`theme: "catppuccin"`,
	}, "\n")), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectsDir, "small.yaml"), MarshalProjectConfig(ProjectConfig{Name: "small"}), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	statePath := filepath.Join(baseDir, "state.json")
	cfgs, err := LoadProjectConfigs(statePath, AppState{Projects: []string{"default"}})
	if err != nil {
		t.Fatalf("LoadProjectConfigs returned error: %v", err)
	}
	if _, ok := cfgs["small"]; !ok {
		t.Fatalf("expected configured project to load from %s", projectsDir)
	}
}
