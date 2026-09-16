package ui

import (
	"os"
	"path/filepath"
	"strings"
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

// TestIsBareExecutableTokenAcceptsValidPathCharacters guards against
// over-restricting valid executable names and absolute paths to a narrow
// character whitelist: shellQuoteLaunchCommand always quotes the resolved
// command as one literal word before it reaches a shell (see
// materializeCommand and Manager.CreateSession), so no character here --
// space, "+", or otherwise -- can be interpreted as shell syntax, and
// validation only needs to judge token shape (bare name or absolute path).
func TestIsBareExecutableTokenAcceptsValidPathCharacters(t *testing.T) {
	for _, value := range []string{
		"codex", "/usr/local/bin/codex", "codex-2",
		"/opt/agent+debug", "/home/me/Agent Tools/codex",
		"codex;id", "codex|id", "codex&&id", "$(id)", "`id`", "codex>out",
	} {
		if !isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = false, want true", value)
		}
	}
}

// TestShellQuoteLaunchCommandNeutralizesShellSyntax guards the safety
// property isBareExecutableToken now relies on: whatever an agent-binary
// value contains, once shell-quoted it runs as one literal word through
// sh -lc rather than being interpreted as shell syntax.
func TestShellQuoteLaunchCommandNeutralizesShellSyntax(t *testing.T) {
	if got := shellQuoteLaunchCommand(""); got != "" {
		t.Fatalf("shellQuoteLaunchCommand(\"\") = %q, want empty so callers' no-command checks still see it", got)
	}
	for _, value := range []string{"codex;id", "codex|id", "$(id)", "`id`", "/home/me/Agent Tools/codex"} {
		quoted := shellQuoteLaunchCommand(value)
		if quoted == value {
			t.Fatalf("shellQuoteLaunchCommand(%q) = %q, want it quoted", value, quoted)
		}
		if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
			t.Fatalf("shellQuoteLaunchCommand(%q) = %q, want single-quoted", value, quoted)
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

// TestIsBareExecutableTokenRejectsArgumentsOnABareName guards the "no
// arguments" contract for a bare name specifically: unlike an absolute
// path, a bare name has no unambiguous end, so whitespace here is the
// arguments delimiter, not a literal character -- "codex --flag" must be
// rejected rather than accepted as one name containing "--flag".
func TestIsBareExecutableTokenRejectsArgumentsOnABareName(t *testing.T) {
	for _, value := range []string{"codex --flag", "codex arg", "codex\ttab"} {
		if isBareExecutableToken(value) {
			t.Fatalf("isBareExecutableToken(%q) = true, want false", value)
		}
	}
}
