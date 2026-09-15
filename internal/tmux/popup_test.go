package tmux

import (
	"fmt"
	"strings"
	"testing"
)

// splitBatchGroups splits a fake Run call's args on runBatch's literal ";"
// separator into the individual command groups tmux itself would run in
// order -- mirroring how a real tmux server processes a chained
// "cmd1 \; cmd2 \; cmd3" invocation. A call that was never batched (every
// non-openMenu caller in this package still shells out one command per
// Run call) comes back as a single group, so callers can loop over the
// result unconditionally regardless of whether this particular call was
// batched.
func splitBatchGroups(args []string) [][]string {
	var groups [][]string
	var current []string
	for _, a := range args {
		if a == ";" {
			groups = append(groups, current)
			current = nil
			continue
		}
		current = append(current, a)
	}
	groups = append(groups, current)
	return groups
}

func TestToggleMenuClosesExistingPopup(t *testing.T) {
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{session_name}":
					return "otter-temp", nil
				case "#{client_name}":
					return "@2", nil
				default:
					t.Fatalf("unexpected display-message format: %v", args)
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-environment":
				return popupEnvKey("@2") + "=1\n", nil
			case "display-popup", "set-environment":
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	wantClose := []string{"display-popup", "-C", "-c", "@2"}
	wantUnset := []string{"set-environment", "-gu", popupEnvKey("@2")}
	for _, want := range [][]string{wantClose, wantUnset} {
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

func TestToggleMenuMarksPopupBeforeOpening(t *testing.T) {
	t.Setenv("TMUX", "")

	var popupArgs []string
	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			for _, group := range splitBatchGroups(args) {
				calls = append(calls, append([]string(nil), group...))
				switch group[0] {
				case "display-message":
					switch group[2] {
					case "#{session_name}":
						return "otter-temp", nil
					case "#{client_name}":
						return "@2", nil
					default:
						t.Fatalf("unexpected display-message format: %v", group)
					}
				case "show-options":
					return "instance-1", nil
				case "show-environment":
					return "", nil
				case "set-environment":
					// A write within a batch -- keep looping to process the
					// remaining groups, matching tmux running each command in
					// a chained invocation in order.
				case "display-popup":
					popupArgs = append([]string(nil), group...)
				default:
					t.Fatalf("unexpected command: %v", group)
				}
			}
			return "", nil
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	wantMark := []string{"set-environment", "-gh", popupEnvKey("@2"), "1"}
	found := false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(wantMark, "\x00") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing call %v in %#v", wantMark, calls)
	}

	got := strings.Join(popupArgs, " ")
	for _, want := range []string{"display-popup", "-c @2", "-E", "-w " + menuWidth, "-h " + menuHeight, "-x C", "-y S", "-e " + CurrentSessionEnv + "=otter-temp", "-e " + CurrentClientEnv + "=@2", "-e " + CurrentInstanceEnv + "=instance-1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup command = %q, want %q", got, want)
		}
	}
	for _, want := range []string{"trap cleanup EXIT HUP INT TERM", "tmux -L ", "set-environment", popupEnvKey("@2"), "/tmp/tflow", " menu"} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup command = %q, want popup script to contain %q", got, want)
		}
	}
	if strings.Contains(got, "exec /tmp/tflow menu") || strings.Contains(got, "exec '/tmp/tflow' menu") {
		t.Fatalf("display-popup command = %q, want popup script to keep shell cleanup active", got)
	}
}

func TestToggleMenuFromPersistentSessionResolvesEmptyInstanceWithoutServerEnvFallback(t *testing.T) {
	// A persistent session (no @tflow-instance marker) must never let the popup
	// inherit a stale instance ID left in the tmux global server environment by
	// an earlier volatile session that this client used to be attached to.
	// The popup must still receive an explicit, empty -e TFLOW_INSTANCE_ID=
	// rather than no flag at all, so the spawned popup process's own
	// environment can never fall through to whatever TFLOW_INSTANCE_ID
	// happens to already be sitting in the tmux server's inherited environment.
	var popupArgs []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			for _, group := range splitBatchGroups(args) {
				switch group[0] {
				case "display-message":
					switch group[2] {
					case "#{session_name}":
						return "dev", nil
					case "#{client_name}":
						return "@2", nil
					default:
						return "", fmt.Errorf("unexpected display-message format: %v", group)
					}
				case "show-options":
					return "", nil
				case "show-environment":
					// Even if the global environment still holds a stale entry from a
					// prior client-scoped registry, it must never be consulted.
					return "TFLOW_MENU_INSTANCE_2=instance-stale\n", nil
				case "set-environment":
					// batched write, keep processing remaining groups
				case "display-popup":
					popupArgs = append([]string(nil), group...)
				default:
					return "", fmt.Errorf("unexpected command: %v", group)
				}
			}
			return "", nil
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	found := false
	for i := 0; i+1 < len(popupArgs); i++ {
		if popupArgs[i] == "-e" && popupArgs[i+1] == CurrentInstanceEnv+"=" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("display-popup args = %#v, want explicit empty -e %s=", popupArgs, CurrentInstanceEnv)
	}
}

func TestToggleMenuPrefersActiveSessionInstanceOverAmbientEnv(t *testing.T) {
	t.Setenv(CurrentInstanceEnv, "instance-stale")

	var popupArgs []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			for _, group := range splitBatchGroups(args) {
				switch group[0] {
				case "display-message":
					switch group[2] {
					case "#{session_name}":
						return "otter-temp", nil
					case "#{client_name}":
						return "@2", nil
					default:
						return "", fmt.Errorf("unexpected display-message format: %v", group)
					}
				case "show-options":
					return "instance-live", nil
				case "show-environment", "set-environment":
					// batched read/write, keep processing remaining groups
				case "display-popup":
					popupArgs = append([]string(nil), group...)
				default:
					return "", fmt.Errorf("unexpected command: %v", group)
				}
			}
			return "", nil
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	got := strings.Join(popupArgs, " ")
	if !strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-live") {
		t.Fatalf("display-popup command = %q, want active session instance", got)
	}
	if strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-stale") {
		t.Fatalf("display-popup command = %q, should not use stale ambient instance env", got)
	}
}

// TestToggleMenuFromPersistentSessionRetainsClientOwnedInstance exercises
// the full sequence a real client goes through: open a popup from a
// volatile session (resolving and remembering instance-1 via the
// client-scoped slot), close it, switch to a persistent session (no
// @tflow-instance marker), then open a popup again. The second popup must
// still receive instance-1, recovered from the remembered client-scoped
// slot rather than lost -- unlike the ambient-process-environment fallback
// the architecture forbids, this is a deliberately keyed, explicitly
// queried global-environment entry, exercised here via a fake that actually
// persists set-environment/show-environment state across calls, the same
// way tmux's real global environment would.
func TestToggleMenuFromPersistentSessionRetainsClientOwnedInstance(t *testing.T) {
	globalEnv := map[string]string{}
	currentSession := "otter-temp"
	var popupArgs []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			for _, group := range splitBatchGroups(args) {
				switch group[0] {
				case "display-message":
					switch group[2] {
					case "#{session_name}":
						return currentSession, nil
					case "#{client_name}":
						return "@2", nil
					default:
						return "", fmt.Errorf("unexpected display-message format: %v", group)
					}
				case "show-options":
					if currentSession == "otter-temp" {
						return "instance-1", nil
					}
					return "", nil
				case "show-environment":
					lines := make([]string, 0, len(globalEnv))
					for key, value := range globalEnv {
						lines = append(lines, key+"="+value)
					}
					return strings.Join(lines, "\n"), nil
				case "set-environment":
					switch group[1] {
					case "-gh":
						globalEnv[group[2]] = group[3]
					case "-gu":
						delete(globalEnv, group[2])
					}
				case "display-popup":
					if !(len(group) > 1 && group[1] == "-C") {
						popupArgs = append([]string(nil), group...)
					}
				default:
					return "", fmt.Errorf("unexpected command: %v", group)
				}
			}
			return "", nil
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("first ToggleMenu (open from volatile session) returned error: %v", err)
	}
	if got := strings.Join(popupArgs, " "); !strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-1") {
		t.Fatalf("first popup command = %q, want instance-1 resolved from the session marker", got)
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("second ToggleMenu (close) returned error: %v", err)
	}

	currentSession = "dev"
	popupArgs = nil
	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("third ToggleMenu (open from persistent session) returned error: %v", err)
	}
	if got := strings.Join(popupArgs, " "); !strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-1") {
		t.Fatalf("third popup command = %q, want instance-1 retained via the client-scoped slot after switching to a persistent session", got)
	}
}

func TestToggleCommandMenuOpensAndClosesCommandSidebar(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	popupVisible := false
	var popupArgs []string
	var keyTables []string
	manager := Manager{Run: func(args ...string) (string, error) {
		for _, group := range splitBatchGroups(args) {
			switch group[0] {
			case "show-environment":
				if popupVisible {
					return popupEnvKey("@2") + "=1\n", nil
				}
				return "", nil
			case "show-options":
				return "instance-1", nil
			case "set-environment":
				if group[1] == "-gh" {
					popupVisible = true
				}
				if group[1] == "-gu" {
					popupVisible = false
				}
			case "switch-client":
				keyTables = append(keyTables, group[len(group)-1])
			case "display-popup":
				if len(group) > 1 && group[1] == "-C" {
					popupVisible = false
				} else {
					popupArgs = append([]string(nil), group...)
				}
			default:
				return "", fmt.Errorf("unexpected command: %v", group)
			}
		}
		return "", nil
	}}

	if err := manager.ToggleCommandMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleCommandMenu open: %v", err)
	}
	if got := strings.Join(popupArgs, " "); !strings.Contains(got, "-e "+MenuModeEnv+"="+MenuModeCommand) {
		t.Fatalf("popup command = %q, want command mode", got)
	} else if !strings.Contains(got, "switch-client") || !strings.Contains(got, "root") {
		t.Fatalf("popup command = %q, want root-table cleanup", got)
	}
	if got := strings.Join(keyTables, ","); got != commandTable {
		t.Fatalf("key tables after open = %q, want %q", got, commandTable)
	}

	if err := manager.ToggleCommandMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleCommandMenu close: %v", err)
	}
	if got := strings.Join(keyTables, ","); got != commandTable+",root" {
		t.Fatalf("key tables after close = %q, want %q", got, commandTable+",root")
	}
}

func TestToggleCommandMenuResetsKeyTableAfterPopupOpenFailure(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	var keyTables []string
	manager := Manager{Run: func(args ...string) (string, error) {
		for _, group := range splitBatchGroups(args) {
			switch group[0] {
			case "show-environment":
				return "", nil
			case "show-options":
				return "instance-1", nil
			case "set-environment":
				// batched write, keep processing remaining groups
			case "switch-client":
				keyTables = append(keyTables, group[len(group)-1])
			case "display-popup":
				// tmux stops the batch here -- the earlier groups in this
				// same invocation (mark + switch-client to commandTable)
				// already applied, matching a real tmux server.
				return "", fmt.Errorf("popup failed")
			default:
				return "", fmt.Errorf("unexpected command: %v", group)
			}
		}
		return "", nil
	}}

	if err := manager.ToggleCommandMenu("/tmp/tflow"); err == nil {
		t.Fatal("ToggleCommandMenu returned nil error after popup failure")
	}
	if got := strings.Join(keyTables, ","); got != commandTable+",root" {
		t.Fatalf("key tables after failed open = %q, want %q", got, commandTable+",root")
	}
}

// TestRunBatchChainsGroupsWithTmuxSeparator guards runBatch's own argv
// shape directly: each group joined by a literal ";" token (tmux's own
// command separator, not a shell feature), in order, with no group merged
// or reordered.
func TestRunBatchChainsGroupsWithTmuxSeparator(t *testing.T) {
	var got []string
	manager := Manager{Run: func(args ...string) (string, error) {
		got = args
		return "", nil
	}}

	if _, err := manager.runBatch(
		[]string{"set-environment", "-gh", "KEY", "1"},
		[]string{"switch-client", "-c", "@2", "-T", "tflow-command"},
		[]string{"display-popup", "-c", "@2"},
	); err != nil {
		t.Fatalf("runBatch returned error: %v", err)
	}

	want := []string{
		"set-environment", "-gh", "KEY", "1", ";",
		"switch-client", "-c", "@2", "-T", "tflow-command", ";",
		"display-popup", "-c", "@2",
	}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("runBatch args = %#v, want %#v", got, want)
	}
}

// TestToggleCommandMenuOpenCollapsesThreeWritesIntoOneSubprocessCall guards
// the item-C latency fix directly: opening the command sidebar must shell
// out exactly once for the mark-popup + key-table-switch + display-popup
// sequence, instead of three separate subprocess round-trips -- the two
// prior reads (menuPopupVisible's show-environment, resolveInstanceID's
// show-options) and the remember-instance write stay their own calls, since
// their results decide what runs next.
func TestToggleCommandMenuOpenCollapsesThreeWritesIntoOneSubprocessCall(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	rawCallCount := 0
	manager := Manager{Run: func(args ...string) (string, error) {
		rawCallCount++
		groups := splitBatchGroups(args)
		if len(groups) == 3 {
			// This is the batched write: mark, key-table switch, then
			// display-popup, in that order.
			if groups[0][0] != "set-environment" || groups[1][0] != "switch-client" || groups[2][0] != "display-popup" {
				t.Fatalf("unexpected batch group order: %#v", groups)
			}
			return "", nil
		}
		switch args[0] {
		case "show-environment":
			return "", nil
		case "show-options":
			return "instance-1", nil
		case "set-environment":
			return "", nil
		default:
			t.Fatalf("unexpected unbatched command: %v", args)
			return "", nil
		}
	}}

	if err := manager.ToggleCommandMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleCommandMenu open: %v", err)
	}

	// menuPopupVisible (show-environment) + resolveInstanceID (show-options)
	// + rememberClientInstance (set-environment) + the one batched write =
	// 4 raw subprocess invocations, down from 6 before this three-write
	// sequence was collapsed into a single runBatch call.
	if rawCallCount != 4 {
		t.Fatalf("raw subprocess call count = %d, want 4", rawCallCount)
	}
}
