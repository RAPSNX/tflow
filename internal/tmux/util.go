package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"tflow/internal/store"
)

var tempSessionAnimals = []string{
	"otter", "fox", "lynx", "koala", "badger",
	"panda", "falcon", "gecko", "orca", "wombat",
	"beaver", "bison", "caracal", "dolphin", "elephant",
	"ferret", "giraffe", "hedgehog", "iguana", "jaguar",
	"kestrel", "lemur", "manatee", "narwhal", "penguin",
}

const (
	persistentSessionPrefix = "tflow-p-"
	volatileSessionPrefix   = "tflow-v-"
)

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
	return store.NormalizeProjectName(name)
}

// NormalizeSessionLabel trims a user-entered session label and strips control
// characters (including the tab and newline bytes ListSessions uses to
// delimit fields and rows) without altering casing or any other printable
// character. This keeps tmux's tab/newline-delimited session metadata safe
// to parse while leaving user-entered labels preserving their exact
// displayed value; only slug-shaped internal tmux identifiers go through
// SanitizeSessionName.
func NormalizeSessionLabel(label string) string {
	filtered := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, label)
	return strings.TrimSpace(filtered)
}

func PersistentSessionName(id string) string {
	id = SanitizeSessionName(id)
	if id == "" {
		return ""
	}
	return persistentSessionPrefix + id
}

// VolatileSessionName returns a tmux-safe, instance-qualified internal name.
// The display label is stored in tmux metadata, never encoded in this name.
func VolatileSessionName(instanceID, id string) string {
	instanceID = store.NormalizeProjectName(instanceID)
	id = SanitizeSessionName(id)
	if instanceID == "" || id == "" {
		return ""
	}
	return volatileSessionPrefix + instanceID + "-" + id
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
		label := strings.TrimSpace(session.Label)
		if label == "" {
			label = session.Name
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

func IsNoSession(err error) bool {
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
