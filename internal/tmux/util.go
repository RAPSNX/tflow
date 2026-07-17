package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
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
	if strings.TrimSpace(cwd) == "" {
		if wd, err := os.Getwd(); err == nil {
			cwd = wd
		}
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = "."
	}
	cwd = expandHomeDir(cwd)
	if abs, err := filepath.Abs(cwd); err == nil {
		return abs
	}
	return cwd
}

func expandHomeDir(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	if len(path) > 1 && path[1] != '/' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, path[2:])
}

func SanitizeSessionName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
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

func normalizeProjectName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func normalizeProjectList(projects []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		if _, ok := seen[project]; ok {
			continue
		}
		seen[project] = struct{}{}
		result = append(result, project)
	}
	sort.Strings(result)
	return result
}
