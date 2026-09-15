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

// stubExecutableOnPath puts a fake, always-successful executable named name
// on PATH for the duration of the test, so materialization tests for
// commands like lazygit don't depend on that host software actually being
// installed (CI runs go test ./... without installing it).
func stubExecutableOnPath(t *testing.T, name string) {
	t.Helper()
	dir := t.TempDir()
	writeExecutable(t, filepath.Join(dir, name))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
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
	for _, value := range []string{"codex", "/usr/local/bin/codex", "codex-2"} {
		if !isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = false, want true", value)
		}
	}
}

// TestIsBareExecutableTokenRejectsRelativeAndTildePaths guards a contract
// narrower than "only shell metacharacters are rejected": ARCHITECTURE.md
// permits only a bare executable name or an absolute path here. A relative
// or tilde-relative value must be rejected even though it contains none of
// the shell metacharacters above, because validateMaterializeExecutable
// joins any non-absolute path to the project workdir rather than expanding
// "~" or resolving it relative to anything else -- so "~/bin/codex" could
// never materialize even when that executable exists.
func TestIsBareExecutableTokenRejectsRelativeAndTildePaths(t *testing.T) {
	for _, value := range []string{"./bin/codex", "~/bin/codex", "../codex", "sub/codex"} {
		if isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = true, want false", value)
		}
	}
}
