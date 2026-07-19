package tmux

import (
	"fmt"
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
	}, map[string]string{
		"dev": "development",
	})
	if err != nil {
		t.Fatalf("SyncSessionProjects returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "dev", "@tflow-project", "small"},
		{"set-option", "-t", "api", "@tflow-project", ""},
		{"set-option", "-t", "blank", "@tflow-project", ""},
		{"set-option", "-t", "dev", "@tflow-session-label", "development"},
		{"set-option", "-t", "api", "@tflow-session-label", "api"},
		{"set-option", "-t", "blank", "@tflow-session-label", "blank"},
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
			return "otter-temp\t1\t1\t1\tinstance-1\nsmall\t2\t0\t0\t\n", nil
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
	if sessions[0].Instance != "instance-1" {
		t.Fatalf("first session instance = %q", sessions[0].Instance)
	}
	if sessions[1].Temporary {
		t.Fatal("expected second session to be persistent")
	}
}

func TestSetSessionTemporaryKeepsSessionAliveWhenUnattached(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", true, "instance-1"); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	for _, want := range [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-u", "-t", "otter-temp", "client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "1"},
		{"set-option", "-t", "otter-temp", "@tflow-instance", "instance-1"},
	} {
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

	for _, call := range calls {
		if len(call) >= 4 && call[0] == "set-hook" && call[1] == "-t" && call[2] == "otter-temp" && call[3] == "client-attached" {
			t.Fatalf("installed destructive client-attached hook: %#v", call)
		}
	}
}

func TestSetSessionTemporarySetsVolatileDisplayLabel(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}
	name := VolatileSessionName("instance-1", "otter")
	if err := manager.SetSessionTemporary(name, true, "instance-1"); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}
	want := []string{"set-option", "-t", name, sessionLabelMarker, "otter"}
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(want, "\x00") {
			return
		}
	}
	t.Fatalf("missing call %v in %#v", want, calls)
}

func TestSetSessionTemporaryClearsDeferredCleanupWhenMadePersistent(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", false, ""); err != nil {
		t.Fatalf("SetSessionTemporary returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-t", "otter-temp", "destroy-unattached", "off"},
		{"set-hook", "-u", "-t", "otter-temp", "client-attached"},
		{"set-option", "-t", "otter-temp", "@tflow-temp", "0"},
		{"set-option", "-t", "otter-temp", "@tflow-instance", ""},
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

func TestSetSessionTemporaryRequiresInstanceIDWhenVolatile(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			return "", nil
		},
	}

	if err := manager.SetSessionTemporary("otter-temp", true, ""); err == nil {
		t.Fatal("SetSessionTemporary returned nil error, want missing instance failure")
	}
}

func TestCreateSessionReturnsDuplicateSessionError(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			if args[0] == "new-session" {
				return "", fmt.Errorf("duplicate session: otter-temp")
			}
			return "", nil
		},
	}

	_, err := manager.CreateSession("otter-temp", "/tmp", "")
	if err == nil {
		t.Fatal("CreateSession returned nil error, want duplicate session failure")
	}
	if !IsSessionExists(err) {
		t.Fatalf("CreateSession error = %v, want duplicate session classification", err)
	}
}

func TestCleanupVolatileSessionsKillsOnlyMatchingInstance(t *testing.T) {
	var killed []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "list-sessions":
				return strings.Join([]string{
					"otter-temp\t1\t0\t1\tinstance-1",
					"fox-temp\t1\t0\t1\tinstance-2",
					"small\t1\t0\t0\tinstance-1",
				}, "\n"), nil
			case "kill-session":
				killed = append(killed, args[2])
				return "", nil
			default:
				return "", nil
			}
		},
	}

	if err := manager.CleanupVolatileSessions("instance-1"); err != nil {
		t.Fatalf("CleanupVolatileSessions returned error: %v", err)
	}
	if got, want := strings.Join(killed, ","), "otter-temp"; got != want {
		t.Fatalf("killed = %q, want %q", got, want)
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

	if err := manager.RenameSession("legacy_code", "lala"); err != nil {
		t.Fatalf("RenameSession returned error: %v", err)
	}

	want := []string{"rename-session", "-t", "legacy_code", "lala"}
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
	existing := []Session{{Name: "otter"}, {Name: "fox"}}
	got := NextTempSessionName(existing)
	if !ContainsAnimalName(got) || got == "otter" || got == "fox" {
		t.Fatalf("NextTempSessionName = %q, want an unused single animal", got)
	}
}

func TestNextTempSessionNameForInstanceIgnoresOtherInstances(t *testing.T) {
	existing := make([]Session, 0, len(tempSessionAnimals))
	for _, animal := range tempSessionAnimals {
		existing = append(existing, Session{Name: VolatileSessionName("instance-1", animal), Temporary: true, Instance: "instance-1"})
	}
	if got := NextTempSessionNameForInstance(existing, "instance-2"); !ContainsAnimalName(got) {
		t.Fatalf("NextTempSessionNameForInstance = %q, want an available single animal", got)
	}
}

func TestVolatileSessionNameKeepsLabelSeparate(t *testing.T) {
	first := VolatileSessionName("instance-1", "code")
	second := VolatileSessionName("instance-2", "code")
	if first == second {
		t.Fatalf("volatile names collide: %q", first)
	}
	if got := VolatileSessionLabel(first, "instance-1"); got != "code" {
		t.Fatalf("VolatileSessionLabel = %q, want code", got)
	}
}

func TestNextTempSessionNameUsesPairsThenSuffixes(t *testing.T) {
	existing := make([]Session, 0, len(tempSessionAnimals)+len(tempSessionAnimals)*(len(tempSessionAnimals)-1))
	for _, animal := range tempSessionAnimals {
		existing = append(existing, Session{Name: animal})
	}
	for _, first := range tempSessionAnimals {
		for _, second := range tempSessionAnimals {
			if first != second {
				existing = append(existing, Session{Name: first + "-" + second})
			}
		}
	}
	if got := NextTempSessionName(existing); got != "otter-fox-2" {
		t.Fatalf("NextTempSessionName = %q, want otter-fox-2", got)
	}
}
