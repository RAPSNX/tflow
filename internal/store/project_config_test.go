package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeProjectConfig(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		input       ProjectConfig
		wantName    string
		wantWorkdir string
	}{
		{
			name:        "trims and normalizes name and workdir",
			input:       ProjectConfig{Name: "  my-project  ", Workdir: "  /tmp/work  "},
			wantName:    "my-project",
			wantWorkdir: filepath.Clean("/tmp/work"),
		},
		{
			name:        "expands home tilde in workdir",
			input:       ProjectConfig{Name: "home-proj", Workdir: "~/code/proj"},
			wantName:    "home-proj",
			wantWorkdir: filepath.Join(home, "code/proj"),
		},
		{
			name:        "empty workdir is preserved",
			input:       ProjectConfig{Name: "empty-dir", Workdir: "   "},
			wantName:    "empty-dir",
			wantWorkdir: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeProjectConfig(tt.input)
			if got.Name != tt.wantName {
				t.Fatalf("name = %q, want %q", got.Name, tt.wantName)
			}
			if got.Workdir != tt.wantWorkdir {
				t.Fatalf("workdir = %q, want %q", got.Workdir, tt.wantWorkdir)
			}
		})
	}
}

func TestExpandHomeDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	if got := expandHomeDir("~"); got != home {
		t.Fatalf("expandHomeDir(~) = %q, want %q", got, home)
	}
	if got := expandHomeDir("~/foo/bar"); got != filepath.Join(home, "foo/bar") {
		t.Fatalf("expandHomeDir(~/foo/bar) = %q, want %q", got, filepath.Join(home, "foo/bar"))
	}
	if got := expandHomeDir("/already/abs"); got != "/already/abs" {
		t.Fatalf("expandHomeDir(/already/abs) = %q, want %q", got, "/already/abs")
	}
	if got := expandHomeDir("~otheruser/path"); got != "~otheruser/path" {
		t.Fatalf("expandHomeDir(~otheruser/path) = %q, want %q", got, "~otheruser/path")
	}
	if got := expandHomeDir(""); got != "" {
		t.Fatalf("expandHomeDir(empty) = %q, want empty", got)
	}
}
