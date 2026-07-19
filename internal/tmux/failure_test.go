package tmux

import (
	"fmt"
	"testing"
)

func TestCloseMenuIgnoresMissingClient(t *testing.T) {
	manager := Manager{Run: func(args ...string) (string, error) {
		return "", fmt.Errorf("can't find client")
	}}
	if err := manager.CloseMenu(); err != nil {
		t.Fatal(err)
	}
}

func TestCloseMenuReturnsOriginalCloseError(t *testing.T) {
	manager := Manager{Run: func(args ...string) (string, error) {
		switch args[0] {
		case "display-message":
			return "client-1", nil
		case "display-popup":
			return "", fmt.Errorf("close failed")
		case "set-environment":
			return "", fmt.Errorf("marker cleanup failed")
		default:
			return "", nil
		}
	}}
	err := manager.CloseMenu()
	if err == nil || err.Error() != "close failed" {
		t.Fatalf("CloseMenu error = %v, want close failed", err)
	}
}
