package tmux

import (
	"strings"
	"testing"
)

func TestAttachCommandUsesTflowSocket(t *testing.T) {
	cmd, err := Manager{}.AttachCommand("dev")
	if err != nil {
		t.Fatalf("AttachCommand returned error: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-L tflow") {
		t.Fatalf("attach command = %q, want tflow socket", got)
	}
}

func TestSwitchClientUsesExplicitClientWhenAvailable(t *testing.T) {
	t.Setenv(CurrentClientEnv, "@1")

	var got []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			got = append([]string(nil), args...)
			return "", nil
		},
	}

	if err := manager.SwitchClient("dev"); err != nil {
		t.Fatalf("SwitchClient returned error: %v", err)
	}

	want := []string{"switch-client", "-c", "@1", "-t", "dev"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("switch-client args = %#v, want %#v", got, want)
	}
}

func TestSyncSessionProjectsSetsProjectMarker(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	err := manager.SyncSessionProjects(map[string]string{
		"dev":   "small",
		"api":   "",
		"blank": "  ",
	})
	if err != nil {
		t.Fatalf("SyncSessionProjects returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "dev", "@tflow-project", "small"},
		{"set-option", "-t", "api", "@tflow-project", ""},
		{"set-option", "-t", "blank", "@tflow-project", ""},
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

func TestListSessionsIncludesTemporaryMarker(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			return "otter-temp\t1\t1\t1\nsmall\t2\t0\t0\n", nil
		},
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if !sessions[0].Temporary {
		t.Fatal("expected first session to be temporary")
	}
	if sessions[1].Temporary {
		t.Fatal("expected second session to be persistent")
	}
}

func TestSetSessionTemporaryTogglesTmuxOptions(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", true); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-t", "otter-temp", "client-attached", "set-option -t 'otter-temp' destroy-unattached on; set-hook -u -t 'otter-temp' client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "1"},
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

func TestSetSessionTemporaryClearsDeferredCleanupWhenMadePersistent(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", false); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-u", "-t", "otter-temp", "client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "0"},
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

func TestRenameSessionUsesTmuxRenameSession(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.RenameSession("dev", "lala"); err != nil {
		t.Fatalf("RenameSession returned error: %v", err)
	}

	want := []string{"rename-session", "-t", "dev", "lala"}
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

func TestNextTempSessionName(t *testing.T) {
	existing := []Session{{Name: "otter-temp"}, {Name: "fox-temp"}}
	if got, want := NextTempSessionName(existing), "lynx-temp"; got != want {
		t.Fatalf("NextTempSessionName = %q, want %q", got, want)
	}
}
