package tmux

import (
	"fmt"
	"strings"
	"testing"
)

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
				}
			case "show-options":
				return "instance-1", nil
			case "show-environment":
				return "", nil
			case "set-environment":
				return "", nil
			case "display-popup":
				popupArgs = append([]string(nil), args...)
				return "", nil
			default:
				t.Fatalf("unexpected command: %v", args)
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
	for _, want := range []string{"display-popup", "-c @2", "-E", "-w " + menuWidth, "-h " + menuHeight, "-x 0", "-y C", "-e " + CurrentSessionEnv + "=otter-temp", "-e " + CurrentClientEnv + "=@2", "-e " + CurrentInstanceEnv + "=instance-1"} {
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
	var popupArgs []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{session_name}":
					return "dev", nil
				case "#{client_name}":
					return "@2", nil
				default:
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-options":
				return "", nil
			case "show-environment":
				// Even if the global environment still holds a stale entry from a
				// prior client-scoped registry, it must never be consulted.
				return "TFLOW_MENU_INSTANCE_2=instance-stale\n", nil
			case "set-environment":
				return "", nil
			case "display-popup":
				popupArgs = append([]string(nil), args...)
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	got := strings.Join(popupArgs, " ")
	if strings.Contains(got, "-e "+CurrentInstanceEnv+"=") {
		t.Fatalf("display-popup command = %q, want no instance env for a persistent session", got)
	}
}

func TestToggleMenuPrefersActiveSessionInstanceOverAmbientEnv(t *testing.T) {
	t.Setenv(CurrentInstanceEnv, "instance-stale")

	var popupArgs []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{session_name}":
					return "otter-temp", nil
				case "#{client_name}":
					return "@2", nil
				default:
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-options":
				return "instance-live", nil
			case "show-environment", "set-environment":
				return "", nil
			case "display-popup":
				popupArgs = append([]string(nil), args...)
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
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
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{session_name}":
					return currentSession, nil
				case "#{client_name}":
					return "@2", nil
				default:
					return "", fmt.Errorf("unexpected display-message format: %v", args)
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
				switch args[1] {
				case "-gh":
					globalEnv[args[2]] = args[3]
				case "-gu":
					delete(globalEnv, args[2])
				}
				return "", nil
			case "display-popup":
				popupArgs = append([]string(nil), args...)
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
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
