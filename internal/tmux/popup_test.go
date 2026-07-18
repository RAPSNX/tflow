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
	wantInstance := []string{"set-environment", "-gh", instanceEnvKey("@2"), "instance-1"}
	found = false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(wantInstance, "\x00") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing call %v in %#v", wantInstance, calls)
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

func TestToggleMenuFallsBackToStoredClientInstanceEnv(t *testing.T) {
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
				return instanceEnvKey("@2") + "=instance-1\n", nil
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
	if !strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-1") {
		t.Fatalf("display-popup command = %q, want stored client instance env", got)
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
