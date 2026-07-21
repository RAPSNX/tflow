package tmux

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/rapsnx/tflow/internal/diag"
)

func TestAttachCommandUsesTflowSocket(t *testing.T) {
	t.Setenv("TMUX", "")

	cmd, err := Manager{}.AttachCommand(context.Background(), "dev")
	if err != nil {
		t.Fatalf("AttachCommand returned error: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-L tflow") {
		t.Fatalf("attach command = %q, want tflow socket", got)
	}
}

func TestAttachCommandUsesRunningTflowSocketPath(t *testing.T) {
	t.Setenv("TMUX", "/run/user/1000/tmux-1000/tflow,49726,0")

	cmd, err := Manager{}.AttachCommand(context.Background(), "dev")
	if err != nil {
		t.Fatalf("AttachCommand returned error: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-S /run/user/1000/tmux-1000/tflow") {
		t.Fatalf("attach command = %q, want -S with the running socket path", got)
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

func TestSwitchClientRetriesAfterStaleClientError(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 1 && args[1] == "-c" {
			return "", fmt.Errorf("can't find client /dev/pts/4")
		}
		return "", nil
	}}

	if err := manager.SwitchClient("otter-temp"); err != nil {
		t.Fatalf("SwitchClient returned error: %v", err)
	}
	wantCalls := [][]string{
		{"switch-client", "-c", "/dev/pts/4", "-t", "otter-temp"},
		{"switch-client", "-t", "otter-temp"},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %v", calls, wantCalls)
	}
	for i, want := range wantCalls {
		if strings.Join(calls[i], "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("calls[%d] = %#v, want %v", i, calls[i], want)
		}
	}
}

func TestSwitchClientReturnsErrorWhenRetryAlsoFails(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	manager := Manager{Run: func(args ...string) (string, error) {
		return "", fmt.Errorf("no current client")
	}}

	if err := manager.SwitchClient("otter-temp"); err == nil {
		t.Fatal("SwitchClient returned nil error, want stale client failure")
	}
}

func TestListSessionsIncludesTemporaryMarker(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			return "otter-temp\t1\t1\t1\tinstance-1\totter\nsmall\t2\t0\t0\t\tdev\n", nil
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
	if sessions[0].Label != "otter" || sessions[1].Label != "dev" {
		t.Fatalf("labels = %#v", sessions)
	}
	if sessions[1].Temporary {
		t.Fatal("expected second session to be persistent")
	}
}

func TestListSessionsTreatsAnyAttachedClientCountAsAttached(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			return strings.Join([]string{
				"none\t1\t0\t0\t\tnone",
				"one\t1\t1\t0\t\tone",
				"many\t1\t3\t0\t\tmany",
			}, "\n") + "\n", nil
		},
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("len(sessions) = %d", len(sessions))
	}
	if sessions[0].Attached {
		t.Fatalf("session %q with count 0 should not be attached", sessions[0].Name)
	}
	if !sessions[1].Attached {
		t.Fatalf("session %q with count 1 should be attached", sessions[1].Name)
	}
	if !sessions[2].Attached {
		t.Fatalf("session %q with count 3 should be attached", sessions[2].Name)
	}
}

func TestListSessionsTreatsAbsentDedicatedServerAsEmpty(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			return "", fmt.Errorf("no server running on /tmp/tmux-1000/tflow")
		},
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 0 {
		t.Fatalf("sessions = %#v, want empty", sessions)
	}
}

func TestKillSessionIgnoresMissingSessionAndServer(t *testing.T) {
	for _, message := range []string{"can't find session: gone", "no server running on /tmp/tmux-1000/tflow"} {
		manager := Manager{Run: func(args ...string) (string, error) { return "", fmt.Errorf("%s", message) }}
		if err := manager.KillSession("gone"); err != nil {
			t.Fatalf("KillSession(%q) returned %v", message, err)
		}
	}
}

func TestSessionPanesAllDeadReportsTrueWhenEveryPaneExited(t *testing.T) {
	manager := Manager{Run: func(args ...string) (string, error) {
		if args[0] == "list-panes" {
			return "1\n1\n1\n", nil
		}
		return "", nil
	}}
	dead, err := manager.SessionPanesAllDead("otter-temp")
	if err != nil {
		t.Fatalf("SessionPanesAllDead returned error: %v", err)
	}
	if !dead {
		t.Fatal("SessionPanesAllDead = false, want true when every pane is dead")
	}
}

func TestSessionPanesAllDeadReportsFalseWhenAnyPaneLive(t *testing.T) {
	manager := Manager{Run: func(args ...string) (string, error) {
		if args[0] == "list-panes" {
			return "1\n0\n1\n", nil
		}
		return "", nil
	}}
	dead, err := manager.SessionPanesAllDead("otter-temp")
	if err != nil {
		t.Fatalf("SessionPanesAllDead returned error: %v", err)
	}
	if dead {
		t.Fatal("SessionPanesAllDead = true, want false when a pane is still live")
	}
}

func TestSessionPanesAllDeadTreatsMissingSessionAsFalse(t *testing.T) {
	for _, message := range []string{"can't find session: gone", "no server running on /tmp/tmux-1000/tflow"} {
		manager := Manager{Run: func(args ...string) (string, error) { return "", fmt.Errorf("%s", message) }}
		dead, err := manager.SessionPanesAllDead("gone")
		if err != nil {
			t.Fatalf("SessionPanesAllDead(%q) returned error: %v", message, err)
		}
		if dead {
			t.Fatalf("SessionPanesAllDead(%q) = true, want false for a missing session/server", message)
		}
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

func TestSetSessionLabelUsesMetadata(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}
	if err := manager.SetSessionLabel("tflow-v-instance-1-abc", "otter"); err != nil {
		t.Fatalf("SetSessionLabel returned error: %v", err)
	}
	want := []string{"set-option", "-t", "tflow-v-instance-1-abc", sessionLabelMarker, "otter"}
	if len(calls) != 1 || strings.Join(calls[0], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("calls = %#v, want %v", calls, want)
	}
}

// TestSetSessionLabelStripsDelimitersBeforeRoundTrip verifies that a label
// containing a tab or newline is filtered before it reaches tmux metadata,
// so a subsequent ListSessions call (which parses each row with
// strings.Split(line, "\t")) sees exactly one session with the same label a
// user would have seen displayed, instead of a truncated or split row.
func TestSetSessionLabelStripsDelimitersBeforeRoundTrip(t *testing.T) {
	var storedLabel string
	manager := Manager{Run: func(args ...string) (string, error) {
		if len(args) == 5 && args[0] == "set-option" && args[3] == sessionLabelMarker {
			storedLabel = args[4]
		}
		return "", nil
	}}

	pastedLabel := "Deploy\tTool\nExtra Row"
	if err := manager.SetSessionLabel("tflow-v-instance-1-abc", pastedLabel); err != nil {
		t.Fatalf("SetSessionLabel returned error: %v", err)
	}
	if strings.ContainsAny(storedLabel, "\t\n\r") {
		t.Fatalf("stored label %q still contains a delimiter character", storedLabel)
	}
	want := "DeployToolExtra Row"
	if storedLabel != want {
		t.Fatalf("stored label = %q, want %q", storedLabel, want)
	}

	// Simulate tmux's list-sessions output using the label exactly as tmux
	// would have stored it, to confirm ListSessions round-trips a single
	// session row rather than splitting on the injected delimiters.
	listManager := Manager{Run: func(args ...string) (string, error) {
		return "tflow-v-instance-1-abc\t1\t0\t1\tinstance-1\t" + storedLabel + "\n", nil
	}}
	sessions, err := listManager.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions returned error: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("len(sessions) = %d, want 1 (label injection split the row)", len(sessions))
	}
	if sessions[0].Label != want {
		t.Fatalf("round-tripped label = %q, want %q", sessions[0].Label, want)
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

func TestSetSessionTemporaryClearsMarkersBeforeLaterFailure(t *testing.T) {
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) == 5 && args[0] == "set-option" && args[3] == "destroy-unattached" {
			return "", fmt.Errorf("cannot update destroy-unattached")
		}
		return "", nil
	}}

	if err := manager.SetSessionTemporary("tflow-p-otter", false, ""); err == nil {
		t.Fatal("SetSessionTemporary returned nil error")
	}
	want := [][]string{
		{"set-option", "-t", "tflow-p-otter", tempMarker, "0"},
		{"set-option", "-t", "tflow-p-otter", instanceMarker, ""},
	}
	if len(calls) < len(want) {
		t.Fatalf("calls = %#v", calls)
	}
	for index, call := range want {
		if strings.Join(calls[index], "\x00") != strings.Join(call, "\x00") {
			t.Fatalf("call %d = %#v, want %#v", index, calls[index], call)
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

func TestDisplayMessageTargetsCurrentClient(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "", nil
	}}

	if err := manager.DisplayMessage("create failed"); err != nil {
		t.Fatalf("DisplayMessage returned error: %v", err)
	}
	want := []string{"display-message", "-c", "/dev/pts/4", "create failed"}
	if len(calls) != 1 || strings.Join(calls[0], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("calls = %#v, want %v", calls, want)
	}
}

func TestDisplayMessageRetriesAfterStaleClientError(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 1 && args[1] == "-c" {
			return "", fmt.Errorf("can't find client /dev/pts/4")
		}
		return "", nil
	}}

	if err := manager.DisplayMessage("create failed"); err != nil {
		t.Fatalf("DisplayMessage returned error: %v", err)
	}
	wantCalls := [][]string{
		{"display-message", "-c", "/dev/pts/4", "create failed"},
		{"display-message", "create failed"},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %v", calls, wantCalls)
	}
	for i, want := range wantCalls {
		if strings.Join(calls[i], "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("calls[%d] = %#v, want %v", i, calls[i], want)
		}
	}
}

func TestCurrentPaneDirTargetsCurrentClient(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		return "/workspace/project\n", nil
	}}

	dir, err := manager.CurrentPaneDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != "/workspace/project" {
		t.Fatalf("directory = %q", dir)
	}
	want := []string{"display-message", "-p", "-c", "/dev/pts/4", "#{pane_current_path}"}
	if len(calls) != 1 || strings.Join(calls[0], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("calls = %#v, want %v", calls, want)
	}
}

func TestCurrentPaneDirRetriesAfterStaleClientError(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 2 && args[2] == "-c" {
			return "", fmt.Errorf("can't find client /dev/pts/4")
		}
		return "/workspace/project\n", nil
	}}

	dir, err := manager.CurrentPaneDir()
	if err != nil {
		t.Fatalf("CurrentPaneDir returned error: %v", err)
	}
	if dir != "/workspace/project" {
		t.Fatalf("directory = %q", dir)
	}
	wantCalls := [][]string{
		{"display-message", "-p", "-c", "/dev/pts/4", "#{pane_current_path}"},
		{"display-message", "-p", "#{pane_current_path}"},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %v", calls, wantCalls)
	}
	for i, want := range wantCalls {
		if strings.Join(calls[i], "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("calls[%d] = %#v, want %v", i, calls[i], want)
		}
	}
}

func TestCurrentPaneDirDoesNotRetryEmptyPathFromResolvedClient(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		// The client resolves successfully but its active pane path is empty.
		return "", nil
	}}

	if _, err := manager.CurrentPaneDir(); err == nil {
		t.Fatal("CurrentPaneDir returned nil error, want empty pane directory failure")
	}
	wantCalls := [][]string{
		{"display-message", "-p", "-c", "/dev/pts/4", "#{pane_current_path}"},
	}
	if len(calls) != len(wantCalls) {
		t.Fatalf("calls = %#v, want %v (no fallback to a different client's pane)", calls, wantCalls)
	}
	if strings.Join(calls[0], "\x00") != strings.Join(wantCalls[0], "\x00") {
		t.Fatalf("calls[0] = %#v, want %v", calls[0], wantCalls[0])
	}
}

func TestCurrentPaneDirReturnsErrorWhenRetryAlsoFails(t *testing.T) {
	t.Setenv(CurrentClientEnv, "/dev/pts/4")
	manager := Manager{Run: func(args ...string) (string, error) {
		return "", fmt.Errorf("no current client")
	}}

	if _, err := manager.CurrentPaneDir(); err == nil {
		t.Fatal("CurrentPaneDir returned nil error, want stale client failure")
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

func TestCreateSessionToleratesMissingWindowOnRename(t *testing.T) {
	var killed bool
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "rename-window":
				return "", fmt.Errorf("can't find window: otter-temp:1")
			case "kill-session":
				killed = true
			}
			return "", nil
		},
	}
	if _, err := manager.CreateSession("otter-temp", "/tmp", ""); err != nil {
		t.Fatalf("CreateSession returned error: %v", err)
	}
	if killed {
		t.Fatal("CreateSession killed the session for a tolerated missing-window rename failure")
	}
}

func TestCreateSessionKillsOrphanedSessionWhenPostCreationSetupFails(t *testing.T) {
	var killedName string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "rename-window":
				return "", fmt.Errorf("tmux: rename failed")
			case "kill-session":
				killedName = args[2]
			}
			return "", nil
		},
	}
	_, err := manager.CreateSession("otter-temp", "/tmp", "")
	if err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("CreateSession error = %v, want the original rename failure", err)
	}
	if killedName != "otter-temp" {
		t.Fatalf("killed session = %q, want otter-temp", killedName)
	}
}

func TestCreateSessionPreservesSetupErrorWhenOrphanCleanupAlsoFails(t *testing.T) {
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "rename-window":
				return "", fmt.Errorf("tmux: rename failed")
			case "kill-session":
				return "", fmt.Errorf("tmux: kill failed")
			}
			return "", nil
		},
	}
	_, err := manager.CreateSession("otter-temp", "/tmp", "")
	if err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("CreateSession error = %v, want the original rename failure preserved", err)
	}
	if strings.Contains(err.Error(), "kill failed") {
		t.Fatalf("CreateSession error = %v, want the cleanup failure not to replace the original error", err)
	}
	if !strings.Contains(buf.String(), "kill orphaned session") {
		t.Fatalf("diagnostic output = %q, want an orphan-cleanup failure diagnostic", buf.String())
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
					"tflow-p-otter\t1\t0\t1\tinstance-1",
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

func TestVolatileSessionNameUsesOpaqueID(t *testing.T) {
	first := VolatileSessionName("instance-1", "abc123")
	second := VolatileSessionName("instance-2", "abc123")
	if first == second || first != "tflow-v-instance-1-abc123" {
		t.Fatalf("volatile names = %q, %q", first, second)
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

func TestCleanupDetachedClientRemovesOwnedSessions(t *testing.T) {
	t.Setenv(CurrentSessionEnv, VolatileSessionName("instance-1", "otter"))

	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		switch args[0] {
		case "show-options":
			return "instance-1", nil
		case "list-sessions":
			return strings.Join([]string{
				VolatileSessionName("instance-1", "otter") + "\t1\t0\t1\tinstance-1",
				VolatileSessionName("instance-2", "fox") + "\t1\t0\t1\tinstance-2",
				"project--dev\t1\t0\t0\t",
			}, "\n"), nil
		default:
			return "", nil
		}
	}}

	if err := manager.CleanupDetachedClient(); err != nil {
		t.Fatalf("CleanupDetachedClient returned error: %v", err)
	}

	want := []string{"kill-session", "-t", VolatileSessionName("instance-1", "otter")}
	found := false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(want, "\x00") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("calls = %#v, want %v", calls, want)
	}
	kills := 0
	for _, call := range calls {
		if len(call) == 3 && call[0] == "kill-session" {
			kills++
		}
		if call[0] == "set-environment" || call[0] == "show-environment" {
			t.Fatalf("calls = %#v, should never touch the tmux global environment", calls)
		}
	}
	if kills != 1 {
		t.Fatalf("kill count = %d, want 1", kills)
	}
}

func TestCleanupDetachedClientResolvesInstancePurelyFromSessionContext(t *testing.T) {
	// A client that detaches from a persistent session (no @tflow-instance marker)
	// must resolve an empty instance ID and never consult the tmux server
	// environment for a stale value from an earlier session.
	t.Setenv(CurrentSessionEnv, "dev")

	var calls [][]string
	manager := Manager{Run: func(args ...string) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		switch args[0] {
		case "show-options":
			return "", nil
		case "list-sessions":
			t.Fatalf("list-sessions should not be called when the session has no instance marker")
		}
		return "", nil
	}}

	if err := manager.CleanupDetachedClient(); err != nil {
		t.Fatalf("CleanupDetachedClient returned error: %v", err)
	}
	for _, call := range calls {
		if call[0] == "set-environment" || call[0] == "show-environment" {
			t.Fatalf("calls = %#v, should never touch the tmux global environment", calls)
		}
	}
}
