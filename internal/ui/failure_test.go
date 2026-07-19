package ui

import (
	"fmt"
	"strings"
	"testing"
)

func TestCreateProjectRequestKillsNewSessionWhenStateSaveFails(t *testing.T) {
	var created, killed string
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created = name
			return session{Name: name}, nil
		},
		killSession: func(name string) error {
			killed = name
			return fmt.Errorf("cleanup failed")
		},
	}
	m, err := buildModel(manager, "")
	if err != nil {
		t.Fatal(err)
	}
	// A directory cannot be atomically replaced by the temporary state file.
	m.statePath = t.TempDir()
	if err := m.createProjectRequest(createRequest{Project: "small", Label: "code", Workdir: "/small"}); err == nil {
		t.Fatal("createProjectRequest returned nil error")
	} else if strings.Contains(err.Error(), "cleanup failed") {
		t.Fatalf("createProjectRequest returned cleanup error: %v", err)
	}
	if created == "" || killed != created {
		t.Fatalf("created = %q, killed = %q", created, killed)
	}
}

func TestPrepareStartupReturnsOriginalErrorWhenCleanupFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	_, err := prepareStartup(fakeTmuxController{
		listSessions: func() ([]session, error) { return nil, nil },
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return fmt.Errorf("tag failed")
		},
		killSession: func(name string) error { return fmt.Errorf("cleanup failed") },
	}, "/tmp/tflow", "/tmp/project", "instance-1")
	if err == nil || err.Error() != "tag startup session: tag failed" {
		t.Fatalf("prepareStartup error = %v, want original tag error", err)
	}
}
