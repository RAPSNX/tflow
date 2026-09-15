package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
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
// matching the accepted shape for a project's agent-binary setting. A
// materialized agent session runs this value through a real shell (`sh -lc`,
// see Manager.CreateSession), so this rejects shell metacharacters outright
// rather than only whitespace: a value like "codex;id" contains no
// whitespace but is not a bare executable token either.
func isBareExecutableToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune("-_./~", r):
		default:
			return false
		}
	}
	// Only a bare name (no separator at all) or an absolute path is
	// accepted; a relative or tilde-relative path is rejected here rather
	// than accepted and mishandled later -- validateMaterializeExecutable
	// joins a non-absolute path to the project workdir, not the user's
	// home, so "~" is never expanded and a value like "~/bin/codex" could
	// never resolve even when that executable exists.
	if strings.ContainsRune(value, filepath.Separator) && !filepath.IsAbs(value) {
		return false
	}
	return true
}
