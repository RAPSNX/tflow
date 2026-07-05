package ui

import (
	"fmt"
	"os"
	"testing"
)

func TestParseSessionNames(t *testing.T) {
	input := "beta [Created 1m ago]\nalpha [Created 2m ago]"
	got := parseSessionNames(input)
	want := []string{"alpha", "beta"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("parseSessionNames = %#v, want %#v", got, want)
	}
}

func TestCurrentZellijSessionPrefersSessionName(t *testing.T) {
	oldName := os.Getenv("ZELLIJ_SESSION_NAME")
	oldSession := os.Getenv("ZELLIJ_SESSION")
	t.Cleanup(func() {
		_ = os.Setenv("ZELLIJ_SESSION_NAME", oldName)
		_ = os.Setenv("ZELLIJ_SESSION", oldSession)
	})
	_ = os.Setenv("ZELLIJ_SESSION_NAME", "Prod/Main")
	_ = os.Setenv("ZELLIJ_SESSION", "ignored")

	if got := currentZellijSession(); got != "prod-main" {
		t.Fatalf("currentZellijSession = %q, want %q", got, "prod-main")
	}
}
