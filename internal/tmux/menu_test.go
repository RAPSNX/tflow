package tmux

import (
	"fmt"
	"strings"
	"testing"
)

func TestEnsureControlModeBindsToggleKey(t *testing.T) {
	t.Setenv("SHELL", "/bin/zsh")

	var calls [][]string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			calls = append(calls, append([]string(nil), args...))
			return "", nil
		},
	}

	if err := manager.EnsureControlMode("/tmp/tflow", Palette{
		Surface0: "#313244",
		Subtext:  "#a6adc8",
		Text:     "#cdd6f4",
		Blue:     "#89b4fa",
		Mantle:   "#181825",
		Teal:     "#94e2d5",
	}); err != nil {
		t.Fatalf("EnsureControlMode returned error: %v", err)
	}

	wants := [][]string{
		{"set-option", "-g", "status", "on"},
		{"set-option", "-g", "status-position", "top"},
		{"set-option", "-g", "status-style", "bg=#181825,fg=#cdd6f4"},
		{"set-option", "-g", "default-terminal", "tmux-256color"},
		{"set-option", "-g", "terminal-overrides", ",*:Tc"},
		{"set-option", "-g", "terminal-features", "xterm-256color:RGB,screen-256color:RGB,tmux-256color:RGB"},
		{"set-option", "-g", "status-left", "#[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] project #[fg=#89b4fa]#{@tflow-project} #[bg=#181825,fg=#313244,nobold]  #[bg=#313244,fg=#a6adc8]#[bg=#313244,fg=#cdd6f4,bold] session #[fg=#94e2d5]#S #[bg=#181825,fg=#313244,nobold]"},
		{"set-option", "-g", "window-status-format", ""},
		{"set-option", "-g", "window-status-current-format", ""},
		{"set-option", "-g", "default-shell", "/bin/zsh"},
		{"set-option", "-g", "default-command", "exec '/bin/zsh' -l"},
		{"bind-key", "-n", "C-f", "run-shell", "TFLOW_CURRENT_SESSION='#{session_name}' TFLOW_CURRENT_CLIENT='#{client_name}' exec '/tmp/tflow' toggle-menu"},
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
	for _, want := range []string{"display-popup", "-c @2", "-E", "-w " + menuWidth, "-h " + menuHeight, "-x #{popup_pane_left}", "-y C", "-e " + CurrentSessionEnv + "=otter-temp", "-e " + CurrentClientEnv + "=@2"} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup command = %q, want %q", got, want)
		}
	}
	for _, want := range []string{"trap cleanup EXIT HUP INT TERM", "set-environment -gu " + popupEnvKey("@2"), "/tmp/tflow", " menu"} {
		if !strings.Contains(got, want) {
			t.Fatalf("display-popup command = %q, want popup script to contain %q", got, want)
		}
	}
	if strings.Contains(got, "exec /tmp/tflow menu") || strings.Contains(got, "exec '/tmp/tflow' menu") {
		t.Fatalf("display-popup command = %q, want popup script to keep shell cleanup active", got)
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
	t.Setenv(CurrentClientEnv, "@2")

	var got []string
	manager := Manager{
		Run: func(args ...string) (string, error) {
			got = append([]string(nil), args...)
			return "", nil
		},
	}

	if err := manager.QuitAll("%7"); err != nil {
		t.Fatalf("QuitAll returned error: %v", err)
	}

	if len(got) != 2 || got[0] != "run-shell" {
		t.Fatalf("run command = %#v, want run-shell", got)
	}
	for _, want := range []string{"display-popup -C -c '@2'", "set-environment -gu " + popupEnvKey("@2"), "detach-client -t '@2'"} {
		if !strings.Contains(got[1], want) {
			t.Fatalf("run-shell script = %q, want %q", got[1], want)
		}
	}
	if strings.Contains(got[1], "kill-pane") {
		t.Fatalf("run-shell script = %q, should not kill the active pane when closing a popup", got[1])
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
