package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"tflow/internal/store"
)

var tempSessionAnimals = []string{
	"otter",
	"fox",
	"lynx",
	"koala",
	"badger",
	"panda",
	"falcon",
	"gecko",
	"orca",
	"wombat",
}

func NormalizeCWD(cwd string) string {
	return store.NormalizeCWD(cwd)
}

func SanitizeSessionName(name string) string {
	parts := strings.Split(name, "--")
	if len(parts) == 2 {
		project := store.NormalizeProjectName(parts[0])
		label := store.NormalizeProjectName(parts[1])
		if project != "" && label != "" {
			return project + "--" + label
		}
	}
	return store.NormalizeProjectName(name)
}

func ProjectSessionName(project, label string) string {
	project = store.NormalizeProjectName(project)
	label = store.NormalizeProjectName(label)
	if project == "" || label == "" {
		return ""
	}
	return project + "--" + label
}

func NextTempSessionName(existing []Session) string {
	used := map[string]struct{}{}
	for _, session := range existing {
		used[session.Name] = struct{}{}
	}
	for _, animal := range tempSessionAnimals {
		name := animal + "-temp"
		if _, ok := used[name]; !ok {
			return name
		}
	}
	for i := 2; ; i++ {
		for _, animal := range tempSessionAnimals {
			name := fmt.Sprintf("%s-temp-%d", animal, i)
			if _, ok := used[name]; !ok {
				return name
			}
		}
	}
}

func Run(args ...string) (string, error) {
	cmd := exec.Command("tmux", append([]string{"-L", socketName}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
}

func IsNoServer(err error) bool {
	if err == nil {
		return false
	}
	// These tmux stderr fragments were captured against tmux 3.7b.
	msg := err.Error()
	return strings.Contains(msg, "no server running") ||
		(strings.Contains(msg, "error connecting to ") && strings.Contains(msg, "No such file or directory"))
}

func IsSessionExists(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate session") ||
		strings.Contains(msg, "session already exists")
}

func isNoSession(err error) bool {
	// These tmux stderr fragments were captured against tmux 3.7b.
	return err != nil && strings.Contains(err.Error(), "can't find session")
}

func ShellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func userShell() string {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		return "/bin/sh"
	}
	return shell
}

func loginShellCommand() string {
	return "exec " + ShellQuote(userShell()) + " -l"
}
