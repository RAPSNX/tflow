package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"

	"tflow/internal/store"
)

var tempSessionAnimals = []string{
	"otter", "fox", "lynx", "koala", "badger",
	"panda", "falcon", "gecko", "orca", "wombat",
	"beaver", "bison", "caracal", "dolphin", "elephant",
	"ferret", "giraffe", "hedgehog", "iguana", "jaguar",
	"kestrel", "lemur", "manatee", "narwhal", "penguin",
}

const volatileSessionPrefix = "volatile-"

func ContainsAnimalName(name string) bool {
	for _, animal := range tempSessionAnimals {
		if name == animal {
			return true
		}
	}
	return false
}

func RandomAnimalName() string {
	return tempSessionAnimals[rand.IntN(len(tempSessionAnimals))]
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

// VolatileSessionName returns a tmux-safe, instance-qualified name for a
// volatile session. The label is intentionally kept separate for display.
func VolatileSessionName(instanceID, label string) string {
	instanceID = store.NormalizeProjectName(instanceID)
	label = store.NormalizeProjectName(label)
	if instanceID == "" || label == "" {
		return ""
	}
	return volatileSessionPrefix + instanceID + "--" + label
}

// VolatileSessionLabel returns the display label encoded in a volatile session
// name for the supplied instance. It returns an empty string for other names.
func VolatileSessionLabel(name, instanceID string) string {
	instanceID = store.NormalizeProjectName(instanceID)
	if instanceID == "" {
		return ""
	}
	prefix := volatileSessionPrefix + instanceID + "--"
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	return strings.TrimPrefix(name, prefix)
}

func NextTempSessionName(existing []Session) string {
	used := map[string]struct{}{}
	for _, session := range existing {
		used[session.Name] = struct{}{}
	}

	singles := make([]string, 0, len(tempSessionAnimals))
	for _, animal := range tempSessionAnimals {
		if _, exists := used[animal]; !exists {
			singles = append(singles, animal)
		}
	}
	if len(singles) > 0 {
		return singles[rand.IntN(len(singles))]
	}

	for _, first := range tempSessionAnimals {
		for _, second := range tempSessionAnimals {
			if first == second {
				continue
			}
			name := first + "-" + second
			if _, exists := used[name]; !exists {
				return name
			}
		}
	}
	for suffix := 2; ; suffix++ {
		for _, first := range tempSessionAnimals {
			for _, second := range tempSessionAnimals {
				if first == second {
					continue
				}
				name := fmt.Sprintf("%s-%s-%d", first, second, suffix)
				if _, exists := used[name]; !exists {
					return name
				}
			}
		}
	}
}

// NextTempSessionNameForInstance chooses a label unused by volatile sessions
// owned by this instance. Other tflow instances may use the same visible label.
func NextTempSessionNameForInstance(existing []Session, instanceID string) string {
	labels := make([]Session, 0, len(existing))
	for _, session := range existing {
		if !session.Temporary || session.Instance != instanceID {
			continue
		}
		label := VolatileSessionLabel(session.Name, instanceID)
		if label == "" {
			label = session.Name // Legacy volatile sessions used their label as the tmux name.
		}
		labels = append(labels, Session{Name: label})
	}
	return NextTempSessionName(labels)
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
