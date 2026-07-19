package ui

import (
	"strings"
	"testing"
)

func TestRunCreateWorkerCreatesAndSwitchesPersistentSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var switched, project string
	manager := fakeTmuxController{
		createSession:     func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
		setSessionProject: func(name, value string) error { project = value; return nil },
		switchClient:      func(name string) error { switched = name; return nil },
	}
	err := runCreateWorker(manager, createRequest{Kind: "session", Project: "small", Label: "code", Workdir: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(switched, "tflow-p-") || project != "small" {
		t.Fatalf("switch = %q, project = %q", switched, project)
	}
}

func TestRunCreateWorkerPromotesOnlyCurrentInstance(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var renamed [][2]string
	var switched string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "tflow-v-one-a", Temporary: true, Instance: "one", Label: "a"}, {Name: "tflow-v-one-b", Temporary: true, Instance: "one", Label: "b"}, {Name: "tflow-v-two-c", Temporary: true, Instance: "two", Label: "c"}}, nil
		},
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, [2]string{oldName, newName})
			return nil
		},
		switchClient: func(name string) error { switched = name; return nil },
	}
	err := runCreateWorker(manager, createRequest{Kind: "project", Project: "small", Current: "tflow-v-one-b", Instance: "one", Workdir: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if len(renamed) != 2 || !strings.HasPrefix(switched, "tflow-p-") {
		t.Fatalf("renamed = %#v, switch = %q", renamed, switched)
	}
	for _, rename := range renamed {
		if !strings.HasPrefix(rename[1], "tflow-p-") {
			t.Fatalf("persistent name = %q", rename[1])
		}
	}
}
