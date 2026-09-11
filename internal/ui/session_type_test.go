package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestValidateMaterializeExecutableResolvesRelativePathAgainstWorkdir(t *testing.T) {
	workdir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workdir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeExecutable(t, filepath.Join(workdir, "bin", "agent"))

	if err := validateMaterializeExecutable(sessionTypeAgent, "./bin/agent", workdir); err != nil {
		t.Fatalf("relative path under workdir should validate, got: %v", err)
	}

	// A relative path that only exists relative to *this test's* working
	// directory (not the project's workdir) must not validate -- otherwise
	// the check is silently keying off the wrong directory.
	if err := validateMaterializeExecutable(sessionTypeAgent, "./bin/agent", t.TempDir()); err == nil {
		t.Fatal("expected an error for a relative path absent from the given workdir")
	}
}

func TestValidateMaterializeExecutableAcceptsBareNameOnPath(t *testing.T) {
	if err := validateMaterializeExecutable(sessionTypeGit, "lazygit-definitely-not-a-real-binary", "/tmp"); err == nil {
		t.Fatal("expected an error for a bare name absent from PATH")
	}
}

func TestValidateMaterializeExecutableAcceptsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent")
	writeExecutable(t, path)

	if err := validateMaterializeExecutable(sessionTypeAgent, path, "/some/other/dir"); err != nil {
		t.Fatalf("absolute path should validate regardless of workdir, got: %v", err)
	}
}

func TestValidateMaterializeExecutableAllowsEmptyCommand(t *testing.T) {
	if err := validateMaterializeExecutable(sessionTypeTerminal, "", "/tmp"); err != nil {
		t.Fatalf("empty command (terminal session) should always pass, got: %v", err)
	}
}

func TestIsBareExecutableTokenRejectsShellMetacharacters(t *testing.T) {
	// These carry no whitespace but must still be rejected: a materialized
	// agent session runs this value through a real shell (sh -lc), so a
	// metacharacter here is a shell-injection vector, not a valid path.
	for _, value := range []string{"codex;id", "codex|id", "codex&&id", "$(id)", "`id`", "codex>out"} {
		if isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = true, want false", value)
		}
	}
	for _, value := range []string{"codex", "/usr/local/bin/codex", "./bin/codex", "~/bin/codex", "codex-2"} {
		if !isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = false, want true", value)
		}
	}
}
