package ui

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	runtmux "github.com/rapsnx/tflow/internal/tmux"
)

func TestMenuStartsWithCurrentSessionSelected(t *testing.T) {
	m := newMenu().(model)
	m.currentSession = "dev"
	m.projects = []string{defaultProjectName}
	m.sessions = []session{{Name: "dev"}}
	m.sessionProjects = map[string]string{"dev": defaultProjectName}
	m.syncSelection()

	if m.selectedProject != defaultProjectName {
		t.Fatalf("selectedProject = %q, want %q", m.selectedProject, defaultProjectName)
	}
	if m.selectedSession != "dev" {
		t.Fatalf("selectedSession = %q, want dev", m.selectedSession)
	}
}

func TestPrepareStartupCreatesSessionBeforeControlMode(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	oldStateHome := os.Getenv("XDG_STATE_HOME")
	oldConfigHome := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		_ = os.Setenv("XDG_STATE_HOME", oldStateHome)
		_ = os.Setenv("XDG_CONFIG_HOME", oldConfigHome)
	})
	_ = os.Setenv("XDG_STATE_HOME", stateHome)
	_ = os.Setenv("XDG_CONFIG_HOME", configHome)

	var calls []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return nil, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			calls = append(calls, "create:"+name)
			if command != "" {
				t.Fatalf("command = %q, want empty", command)
			}
			return session{Name: name}, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			calls = append(calls, fmt.Sprintf("temporary:%s:%t:%s", name, temporary, instanceID))
			return nil
		},
		setSessionLabel: func(name, label string) error {
			calls = append(calls, "label:"+name+":"+label)
			if !runtmux.ContainsAnimalName(label) {
				t.Fatalf("label = %q, want animal label", label)
			}
			return nil
		},
		ensureControlMode: func(binaryPath string) error {
			calls = append(calls, "control:"+binaryPath)
			return nil
		},
	}

	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err != nil {
		t.Fatalf("prepareStartup returned error: %v", err)
	}
	if !strings.HasPrefix(name, "tflow-v-instance-1-") {
		t.Fatalf("name = %q, want opaque volatile id", name)
	}

	if got, want := fmt.Sprint(calls), fmt.Sprint([]string{"create:" + name, "temporary:" + name + ":true:instance-1", "label:" + name + ":" + strings.TrimPrefix(calls[2], "label:"+name+":"), "control:/tmp/tflow"}); got != want {
		t.Fatalf("calls = %s, want %s", got, want)
	}
}

func TestPrepareStartupRetriesWhenTempSessionNameAlreadyExists(t *testing.T) {
	var calls []string
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			calls = append(calls, name)
			if len(calls) == 1 {
				return session{}, fmt.Errorf("duplicate session")
			}
			return session{Name: name}, nil
		},
	}
	name, err := prepareStartup(manager, "/tmp/tflow", "/tmp/project", "instance-1")
	if err != nil || len(calls) != 2 || calls[0] == calls[1] || name != calls[1] {
		t.Fatalf("retry name = %q calls = %#v err = %v", name, calls, err)
	}
}

// TestStartWithManagerDoesNotLeakInstanceIDIntoProcessEnv proves startup
// never taints its own process environment with the instance ID -- doing so
// would leak into any tmux server exec.Command forks with no explicit Env,
// letting popups that should resolve an empty instance fall back to this
// ambient value instead. See popupInstanceEnvArgs for the matching guarantee
// on the popup side.
func TestStartWithManagerDoesNotLeakInstanceIDIntoProcessEnv(t *testing.T) {
	before := os.Getenv(menuInstanceEnv)
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return nil, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return nil
		},
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, "sh", "-c", ":"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-2", fixedAttachContext(context.Background())); err != nil {
		t.Fatalf("startWithManager returned error: %v", err)
	}
	if got := os.Getenv(menuInstanceEnv); got != before {
		t.Fatalf("%s = %q, want unchanged (%q); startup must not set it in the process environment", menuInstanceEnv, got, before)
	}
}

func TestStartWithManagerCleansUpInstanceVolatileSessionsAfterAttach(t *testing.T) {
	var cleaned []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return nil, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			return nil
		},
		attachCommand: func(ctx context.Context, name string) (*exec.Cmd, error) {
			return exec.CommandContext(ctx, "sh", "-c", ":"), nil
		},
		cleanupVolatile: func(instanceID string) error {
			cleaned = append(cleaned, instanceID)
			return nil
		},
	}

	if err := startWithManager(manager, "/tmp/tflow", "/tmp/project", "instance-2", fixedAttachContext(context.Background())); err != nil {
		t.Fatalf("startWithManager returned error: %v", err)
	}
	if got, want := fmt.Sprint(cleaned), fmt.Sprint([]string{"instance-2"}); got != want {
		t.Fatalf("cleanup calls = %s, want %s", got, want)
	}
}

func TestNewInstanceIDWithEntropyUsesRandomToken(t *testing.T) {
	now := time.Unix(0, 123456789)
	got := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}), 99)
	if want := "tflow-21i3v9-000102030405"; got != want {
		t.Fatalf("newInstanceIDWithEntropy = %q, want %q", got, want)
	}
}

func TestNewInstanceIDWithEntropyFallsBackToPID(t *testing.T) {
	now := time.Unix(0, 123456789)
	got := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1}), 4242)
	if want := "tflow-21i3v9-4242"; got != want {
		t.Fatalf("newInstanceIDWithEntropy = %q, want %q", got, want)
	}
}

func TestNewInstanceIDWithEntropyDiffersWithinSameTick(t *testing.T) {
	now := time.Unix(0, 123456789)
	first := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{0, 1, 2, 3, 4, 5}), 99)
	second := newInstanceIDWithEntropy(now, bytes.NewReader([]byte{6, 7, 8, 9, 10, 11}), 99)
	if first == second {
		t.Fatalf("same-tick instance ids matched: %q", first)
	}
}

func TestSessionsLoadedRecoversAttachedVolatileSessionWhenCurrentContextIsStale(t *testing.T) {
	m := newModel(fakeTmuxController{}, "stale-session").(model)
	m.instanceID = "instance-1"
	m.projects = []string{"old-project"}
	m.sessions = []session{{Name: "stale-session", Temporary: true, Instance: "instance-1"}}

	updated, _ := m.Update(sessionsLoadedMsg{sessions: []session{{Name: "tflow-v-instance-1-live", Temporary: true, Instance: "instance-1", Attached: true}}})
	got := updated.(model)
	if got.currentSession != "tflow-v-instance-1-live" {
		t.Fatalf("currentSession = %q, want attached volatile session", got.currentSession)
	}
	if sessions := got.contextSessions(); len(sessions) != 1 || sessions[0].Name != got.currentSession {
		t.Fatalf("context sessions = %#v, want recovered live session", sessions)
	}
}
