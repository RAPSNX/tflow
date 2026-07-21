package tmux

import (
	"fmt"
	"strings"
	"testing"
)

func TestQuitAllResolvesInstanceFromSessionOnlyIgnoringServerEnv(t *testing.T) {
	// QuitAll must resolve the instance solely from the current session's
	// @tflow-instance marker; it must never fall back to a client-scoped
	// registry in the tmux global server environment, which no longer exists.
	t.Setenv(CurrentSessionEnv, "dev")
	t.Setenv(CurrentClientEnv, "@2")
	t.Setenv(CurrentInstanceEnv, "instance-stale")

	var killed []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "show-options":
				return "instance-live", nil
			case "show-environment":
				// Even if the global environment still holds a stale entry, it
				// must never be consulted for instance resolution.
				return "TFLOW_MENU_INSTANCE_2=instance-stale\n", nil
			case "list-sessions":
				return strings.Join([]string{
					"otter-temp\t1\t0\t1\tinstance-stale",
					"fox-temp\t1\t0\t1\tinstance-live",
				}, "\n"), nil
			case "kill-session":
				killed = append(killed, args[2])
				return "", nil
			case "run-shell":
				return "", nil
			default:
				return "", nil
			}
		},
	}

	if err := manager.QuitAll(); err != nil {
		t.Fatalf("QuitAll returned error: %v", err)
	}

	if got, want := strings.Join(killed, ","), "fox-temp"; got != want {
		t.Fatalf("killed = %q, want %q", got, want)
	}
}

func TestToggleMenuDoesNotFallBackToAmbientEnvWithoutSessionOrClientInstance(t *testing.T) {
	t.Setenv(CurrentInstanceEnv, "instance-stale")

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
			case "show-options", "show-environment", "set-environment":
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
	if strings.Contains(got, "-e "+CurrentInstanceEnv+"=instance-stale") {
		t.Fatalf("display-popup command = %q, should not use ambient instance env without session or client ownership", got)
	}
}

func TestQuitAllDoesNotFallBackToAmbientEnvWithoutSessionOrClientInstance(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "dev")
	t.Setenv(CurrentClientEnv, "@2")
	t.Setenv(CurrentInstanceEnv, "instance-stale")

	var killed []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "show-options", "show-environment", "run-shell":
				return "", nil
			case "list-sessions":
				return strings.Join([]string{
					"otter-temp\t1\t0\t1\tinstance-stale",
					"fox-temp\t1\t0\t1\tinstance-live",
				}, "\n"), nil
			case "kill-session":
				killed = append(killed, args[2])
				return "", nil
			default:
				return "", nil
			}
		},
	}

	if err := manager.QuitAll(); err != nil {
		t.Fatalf("QuitAll returned error: %v", err)
	}

	if len(killed) != 0 {
		t.Fatalf("killed = %#v, want none", killed)
	}
}

func TestToggleMenuUnmarksPopupIfOpenFails(t *testing.T) {
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
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-options":
				return "instance-1", nil
			case "show-environment", "set-environment":
				return "", nil
			case "display-popup":
				return "", fmt.Errorf("popup failed")
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err == nil {
		t.Fatal("ToggleMenu returned nil error, want failure")
	}

	wantUnset := []string{"set-environment", "-gu", popupEnvKey("@2")}
	found := false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(wantUnset, "\x00") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing cleanup call %v in %#v", wantUnset, calls)
	}
}

func TestQuitAllDetachesExplicitClientWhenAvailable(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			switch args[0] {
			case "show-options":
				return "instance-1", nil
			case "list-sessions":
				return "otter-temp\t1\t1\t1\tinstance-1\nfox-temp\t1\t0\t1\tinstance-1\n", nil
			case "run-shell", "kill-session":
				return "", nil
			default:
				return "", nil
			}
		},
	}

	if err := manager.QuitAll(); err != nil {
		t.Fatalf("QuitAll returned error: %v", err)
	}

	foundKill := false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join([]string{"kill-session", "-t", "fox-temp"}, "\x00") {
			foundKill = true
			break
		}
	}
	if !foundKill {
		t.Fatalf("calls = %#v, want detached volatile session cleanup", calls)
	}

	got := calls[len(calls)-1]
	if len(got) != 2 || got[0] != "run-shell" {
		t.Fatalf("run command = %#v, want run-shell", got)
	}
	for _, want := range []string{
		"tmux -L 'tflow' 'display-popup' '-C' '-c' '@2'",
		"tmux -L 'tflow' 'set-environment' '-gu' '" + popupEnvKey("@2") + "'",
		"tmux -L 'tflow' 'detach-client' '-t' '@2'",
	} {
		if !strings.Contains(got[1], want) {
			t.Fatalf("run-shell script = %q, want %q", got[1], want)
		}
	}
	if strings.Contains(got[1], "kill-pane") {
		t.Fatalf("run-shell script = %q, should not kill the active pane when closing a popup", got[1])
	}
}

func TestToggleMenuOpensClosesThenOpensAgain(t *testing.T) {
	popupVisible := false
	popupOpenCount := 0
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
				return "instance-1", nil
			case "show-environment":
				if popupVisible {
					return popupEnvKey("@2") + "=1\n", nil
				}
				return "", nil
			case "set-environment":
				if len(args) >= 4 && args[1] == "-gh" && args[2] == popupEnvKey("@2") {
					popupVisible = true
				}
				if len(args) >= 3 && args[1] == "-gu" && args[2] == popupEnvKey("@2") {
					popupVisible = false
				}
				return "", nil
			case "display-popup":
				if len(args) >= 2 && args[1] == "-C" {
					popupVisible = false
					return "", nil
				}
				popupOpenCount++
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	for _, label := range []string{"open", "close", "reopen"} {
		if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
			t.Fatalf("%s toggle returned error: %v", label, err)
		}
	}

	if popupOpenCount != 2 {
		t.Fatalf("popupOpenCount = %d, want 2", popupOpenCount)
	}
	if !popupVisible {
		t.Fatal("popupVisible = false, want true after reopen")
	}
}

func TestToggleMenuUsesBoundContextEnv(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			switch args[0] {
			case "show-options":
				return "", nil
			case "show-environment", "set-environment", "display-popup":
				return "", nil
			case "display-message":
				t.Fatalf("display-message should not be used when bound context env is present: %v", args)
				return "", nil
			default:
				t.Fatalf("unexpected command: %v", args)
				return "", nil
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	got := strings.Join(calls[len(calls)-1], " ")
	for _, want := range []string{"-c @2", "-e " + CurrentSessionEnv + "=otter-temp", "-e " + CurrentClientEnv + "=@2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup call = %q, want %q", got, want)
		}
	}
}

func TestToggleMenuClearsStalePopupMarkerOnCloseError(t *testing.T) {
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
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-environment":
				return popupEnvKey("@2") + "=1\n", nil
			case "display-popup":
				return "", fmt.Errorf("exit status 1")
			case "set-environment":
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}

	wantUnset := []string{"set-environment", "-gu", popupEnvKey("@2")}
	found := false
	for _, call := range calls {
		if strings.Join(call, "\x00") == strings.Join(wantUnset, "\x00") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing cleanup call %v in %#v", wantUnset, calls)
	}
}

func TestToggleMenuIgnoresMissingPopupMarkerOnClose(t *testing.T) {
	manager := Manager{
		Run: func(args ...string) (string, error) {
			switch args[0] {
			case "display-message":
				switch args[2] {
				case "#{session_name}":
					return "otter-temp", nil
				case "#{client_name}":
					return "/dev/pts/0", nil
				default:
					return "", fmt.Errorf("unexpected display-message format: %v", args)
				}
			case "show-environment":
				return popupEnvKey("/dev/pts/0") + "=1\n", nil
			case "display-popup":
				return "", nil
			case "set-environment":
				if len(args) >= 3 && args[1] == "-gu" {
					return "", fmt.Errorf("exit status 1")
				}
				return "", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}

	if err := manager.ToggleMenu("/tmp/tflow"); err != nil {
		t.Fatalf("ToggleMenu returned error: %v", err)
	}
}

func TestOpenQuitOpensQuitConfirmationPopup(t *testing.T) {
	t.Setenv(CurrentSessionEnv, "otter-temp")
	t.Setenv(CurrentClientEnv, "@2")

	var popupArgs []string
	manager := Manager{Run: func(args ...string) (string, error) {
		switch args[0] {
		case "show-environment", "set-environment":
			return "", nil
		case "show-options":
			return "instance-1", nil
		case "display-popup":
			popupArgs = append([]string(nil), args...)
			return "", nil
		default:
			return "", fmt.Errorf("unexpected command: %v", args)
		}
	}}

	if err := manager.OpenQuit("/tmp/tflow"); err != nil {
		t.Fatalf("OpenQuit returned error: %v", err)
	}
	got := strings.Join(popupArgs, " ")
	for _, want := range []string{"-c @2", "-e " + CurrentInstanceEnv + "=instance-1", "-e " + MenuModeEnv + "=" + MenuModeQuit} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup command = %q, want %q", got, want)
		}
	}
}
