package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	runtmux "github.com/rapsnx/tflow/internal/tmux"
)

// materializeCommand resolves the shell command a lazily materialized
// session should run: lazygit for a git session, the project's captured
// agent binary for an agent session, and the empty string (an ordinary
// shell) for anything else, including legacy untyped records.
func materializeCommand(sessionType, command string) string {
	switch sessionType {
	case sessionTypeGit:
		return "lazygit"
	case sessionTypeAgent:
		return strings.TrimSpace(command)
	default:
		return ""
	}
}

// lookupSessionTypeCommand finds the persisted type and command for a
// stored session ID within its project, without requiring it to be running.
func lookupSessionTypeCommand(state appState, project, id string) (sessionType, command string) {
	project = normalizeProjectName(project)
	for _, p := range state.Projects {
		if normalizeProjectName(p.Name) != project {
			continue
		}
		for _, s := range p.Sessions {
			if s.ID == id {
				return strings.TrimSpace(s.Type), strings.TrimSpace(s.Command)
			}
		}
	}
	return "", ""
}

// validateMaterializeExecutable reports a clear, descriptive error when the
// executable a git or agent session would run cannot be found, so
// materialization fails without creating a broken tmux session or otherwise
// mutating state. A bare name (no path separator) is resolved on PATH; an
// absolute path is checked directly; a relative path containing a separator
// is checked against workdir, matching where tmux will actually launch it
// (its session is created with that directory, per CreateSession's cwd
// argument), rather than this process's own working directory. Terminal
// sessions (empty command) always pass.
func validateMaterializeExecutable(sessionType, resolvedCommand, workdir string) error {
	if resolvedCommand == "" {
		return nil
	}
	path := resolvedCommand
	if !filepath.IsAbs(path) && strings.ContainsRune(path, filepath.Separator) {
		path = filepath.Join(workdir, path)
	}
	if filepath.IsAbs(path) {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("%s executable %q not found: %w", sessionType, resolvedCommand, err)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return fmt.Errorf("%s executable %q is not executable", sessionType, resolvedCommand)
		}
		return nil
	}
	if _, err := exec.LookPath(path); err != nil {
		return fmt.Errorf("%s executable %q not found on PATH", sessionType, resolvedCommand)
	}
	return nil
}

// isBareExecutableToken reports whether value is a single path-like token
// (a bare executable name or an absolute path) with no shell arguments,
// matching the accepted shape for a project's agent-binary setting. This
// judges token *shape*, not individual characters: an executable name or
// path may legitimately contain almost anything (spaces, "+", etc.), and
// shellQuoteLaunchCommand quotes the resolved command as one literal word
// before it ever reaches a shell (see materializeCommand and
// Manager.CreateSession), so no character here is a shell-injection vector.
func isBareExecutableToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if !strings.ContainsRune(value, filepath.Separator) {
		// A bare name is looked up on PATH as one word, so whitespace here
		// is the arguments delimiter, not a valid character in the
		// executable's own name -- the setting accepts an executable name
		// or absolute path without arguments, so "codex --flag" must be
		// rejected rather than treated as one name that happens to contain
		// "--flag". Nothing else about a bare name is ambiguous this way,
		// so no other character is restricted.
		return !strings.ContainsAny(value, " \t\n\r")
	}
	// A path containing a separator is only accepted when absolute; a
	// relative or tilde-relative path is rejected here rather than
	// accepted and mishandled later -- validateMaterializeExecutable joins
	// a non-absolute path to the project workdir, not the user's home, so
	// "~" is never expanded and a value like "~/bin/codex" could never
	// resolve even when that executable exists. An absolute path may
	// otherwise contain any character, including a space: its leading "/"
	// already makes it unambiguous (the whole value is the path, not a
	// command plus arguments), and shellQuoteLaunchCommand quotes the
	// resolved command as one literal word before it reaches a shell, so a
	// space here is a literal path character, never an arguments
	// delimiter.
	return filepath.IsAbs(value)
}

// shellQuoteLaunchCommand wraps a non-empty resolved command (from
// materializeCommand) in single quotes so it always runs as one literal
// word through sh -lc regardless of spaces or other characters in an
// executable's path, instead of being word-split or interpreted as shell
// syntax. Empty stays empty so callers' "no command" checks still see it.
func shellQuoteLaunchCommand(resolvedCommand string) string {
	if resolvedCommand == "" {
		return ""
	}
	return runtmux.ShellQuote(resolvedCommand)
}
