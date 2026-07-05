package ui

import (
	"fmt"
	"os"
	"strings"
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

func TestZellijConfigContentsBindsCtrlFToMenuLauncher(t *testing.T) {
	config := zellijConfigContents("/tmp/tflow-open-menu")

	if !strings.Contains(config, `bind "Ctrl f" {`) {
		t.Fatalf("config missing ctrl+f binding: %s", config)
	}
	if !strings.Contains(config, `Run "/tmp/tflow-open-menu" {`) {
		t.Fatalf("config missing launcher run action: %s", config)
	}
	if !strings.Contains(config, `close_on_exit true`) {
		t.Fatalf("config missing close_on_exit: %s", config)
	}
}

func TestZellijLauncherScriptUsesExecutableBeforePathLookup(t *testing.T) {
	script := zellijLauncherScript()

	if !strings.Contains(script, "exec tflow menu") {
		t.Fatalf("script missing PATH fallback: %s", script)
	}
	if !strings.Contains(script, "menu") {
		t.Fatalf("script missing menu invocation: %s", script)
	}
}
