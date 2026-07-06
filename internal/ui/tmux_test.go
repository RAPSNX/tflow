package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestShellQuote(t *testing.T) {
	if got, want := shellQuote("it's"), `'it'"'"'s'`; got != want {
		t.Fatalf("shellQuote = %q, want %q", got, want)
	}
}

func TestNormalizeCWDExpandsHome(t *testing.T) {
	t.Setenv("HOME", "/tmp/home")

	if got := normalizeCWD("~/project"); got != "/tmp/home/project" {
		t.Fatalf("normalizeCWD = %q, want /tmp/home/project", got)
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
	t.Setenv("SHELL", "/bin/zsh")

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
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] session #[fg=#94e2d5]#{@tflow-session} #[bg=#181825,fg=#313244,nobold]"},
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

func TestSyncSessionMetadataSetsProjectAndDisplayMarkers(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	err := manager.SyncSessionMetadata(map[string]sessionMetadata{
		"garden_code": {Project: "garden", DisplayName: "code"},
	})
	if err != nil {
		t.Fatalf("SyncSessionMetadata returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "garden_code", "@tflow-project", "garden"},
		{"set-option", "-t", "garden_code", "@tflow-session", "code"},
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

func TestListSessionsParsesTmuxOutput(t *testing.T) {
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			return "garden_code\t1\t1\ngarden_shell\t2\t0\n", nil
		},
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if !sessions[0].Attached || sessions[1].Attached {
		t.Fatalf("unexpected attached flags: %#v", sessions)
	}
}

func TestSetSessionTemporaryOnlySetsMarker(t *testing.T) {
	var calls [][]string
	manager := tmuxSessionManager{
		run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", true); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	want := []string{"set-option", "-t", "otter-temp", "@tflow-temp", "1"}
	if got := fmt.Sprint(calls); !strings.Contains(got, fmt.Sprint(want)) {
		t.Fatalf("calls = %#v, want %v", calls, want)
	}
}
