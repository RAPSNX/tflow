package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// mutating state. A bare name is resolved on PATH; an absolute path is
// checked directly. Terminal sessions (empty command) always pass.
func validateMaterializeExecutable(sessionType, resolvedCommand string) error {
	if resolvedCommand == "" {
		return nil
	}
	if filepath.IsAbs(resolvedCommand) {
		info, err := os.Stat(resolvedCommand)
		if err != nil {
			return fmt.Errorf("%s executable %q not found: %w", sessionType, resolvedCommand, err)
		}
		if info.IsDir() || info.Mode()&0o111 == 0 {
			return fmt.Errorf("%s executable %q is not executable", sessionType, resolvedCommand)
		}
		return nil
	}
	if _, err := exec.LookPath(resolvedCommand); err != nil {
		return fmt.Errorf("%s executable %q not found on PATH", sessionType, resolvedCommand)
	}
	return nil
}

// isBareExecutableToken reports whether value is a single path-like token
// (a bare executable name or an absolute path) with no shell arguments,
// matching the accepted shape for a project's agent-binary setting.
func isBareExecutableToken(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	return !strings.ContainsAny(value, " \t\n")
}
