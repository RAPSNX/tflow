package ui

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/diag"
)

func TestRunCreateWorkerCreatesAndSwitchesPersistentSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var switched, project string
	manager := fakeTmuxController{
		createSession:     func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
		setSessionProject: func(name, value string) error { project = value; return nil },
		switchClient:      func(name string) error { switched = name; return nil },
	}
	err := runCreateWorker(manager, createRequest{Kind: "session", Project: "small", Label: "code", Workdir: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(switched, "tflow-p-") || project != "small" {
		t.Fatalf("switch = %q, project = %q", switched, project)
	}
}

func TestRunCreateWorkerWaitsForStateLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	unlock, err := lockAppState(appStatePath())
	if err != nil {
		t.Fatal(err)
	}
	completed := make(chan error, 1)
	go func() {
		completed <- runCreateWorker(fakeTmuxController{
			createSession: func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
		}, createRequest{Kind: "session", Project: "small", Label: "code", Workdir: "/tmp"})
	}()

	select {
	case err := <-completed:
		t.Fatalf("worker completed while state lock was held: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-completed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not resume after state lock was released")
	}
}

func TestRunCreateWorkerEmitsDiagnosticWhenLockReleaseFails(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	originalLock := lockAppState
	lockAppState = func(path string) (func() error, error) {
		unlock, err := originalLock(path)
		if err != nil {
			return nil, err
		}
		return func() error {
			_ = unlock()
			return fmt.Errorf("lock release boom")
		}, nil
	}
	t.Cleanup(func() { lockAppState = originalLock })

	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) { return session{Name: name}, nil },
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Project: "small", Label: "code", Workdir: "/tmp"}); err != nil {
		t.Fatalf("runCreateWorker returned error: %v", err)
	}
	if !strings.Contains(buf.String(), "lock release boom") {
		t.Fatalf("diagnostic output = %q, want lock-release failure reported", buf.String())
	}
}

func TestRunCreateWorkerRejectsDuplicateProject(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	created := 0
	manager := fakeTmuxController{
		createSession: func(name, cwd, command string) (session, error) {
			created++
			return session{Name: name}, nil
		},
	}
	request := createRequest{Kind: "project", Project: "small", Label: "code", Workdir: "/tmp"}
	if err := runCreateWorker(manager, request); err != nil {
		t.Fatal(err)
	}
	if err := runCreateWorker(manager, request); err == nil || !strings.Contains(err.Error(), "project already exists") {
		t.Fatalf("duplicate project error = %v, want project already exists", err)
	}
	if created != 1 {
		t.Fatalf("created sessions = %d, want 1", created)
	}
}

func TestRunCreateWorkerRejectsDuplicateVolatileLabel(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var created session
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			if created.Name == "" {
				return nil, nil
			}
			return []session{created}, nil
		},
		createSession: func(name, cwd, command string) (session, error) {
			created = session{Name: name}
			return created, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			created.Temporary, created.Instance = temporary, instanceID
			return nil
		},
		setSessionLabel: func(name, label string) error {
			created.Label = label
			return nil
		},
	}
	request := createRequest{Kind: "session", Label: "notes", Workdir: "/tmp", Instance: "one"}
	if err := runCreateWorker(manager, request); err != nil {
		t.Fatal(err)
	}
	if err := runCreateWorker(manager, request); err == nil || !strings.Contains(err.Error(), "session name already exists") {
		t.Fatalf("duplicate volatile label error = %v, want session name already exists", err)
	}
	if created.Label != "notes" || !created.Temporary || created.Instance != "one" {
		t.Fatalf("created volatile session = %#v", created)
	}
}

func TestRunCreateWorkerAllowsVolatileLabelCaseVariantsButRejectsExactDuplicate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var sessions []session
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return sessions, nil },
		createSession: func(name, cwd, command string) (session, error) {
			s := session{Name: name}
			sessions = append(sessions, s)
			return s, nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			for i := range sessions {
				if sessions[i].Name == name {
					sessions[i].Temporary, sessions[i].Instance = temporary, instanceID
				}
			}
			return nil
		},
		setSessionLabel: func(name, label string) error {
			for i := range sessions {
				if sessions[i].Name == name {
					sessions[i].Label = label
				}
			}
			return nil
		},
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Label: "Notes", Workdir: "/tmp", Instance: "one"}); err != nil {
		t.Fatal(err)
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Label: "notes", Workdir: "/tmp", Instance: "one"}); err != nil {
		t.Fatalf("case-distinct volatile label rejected: %v", err)
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Label: "Notes", Workdir: "/tmp", Instance: "one"}); err == nil || !strings.Contains(err.Error(), "session name already exists") {
		t.Fatalf("exact duplicate volatile label error = %v, want session name already exists", err)
	}
	if len(sessions) != 2 || sessions[0].Label != "Notes" || sessions[1].Label != "notes" {
		t.Fatalf("sessions = %#v, want distinct case-variant labels preserved verbatim", sessions)
	}
}

func TestRunCreateWorkerPreservesLabelCasingAndRejectsExactProjectDuplicate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var sessions []session
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) { return sessions, nil },
		createSession: func(name, cwd, command string) (session, error) {
			s := session{Name: name}
			sessions = append(sessions, s)
			return s, nil
		},
	}
	if err := runCreateWorker(manager, createRequest{Kind: "project", Project: "small", Label: "Main", Workdir: "/tmp"}); err != nil {
		t.Fatal(err)
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Project: "small", Label: "main", Workdir: "/tmp"}); err != nil {
		t.Fatalf("case-distinct project label rejected: %v", err)
	}
	if err := runCreateWorker(manager, createRequest{Kind: "session", Project: "small", Label: "Main", Workdir: "/tmp"}); err == nil || !strings.Contains(err.Error(), "session name already exists in this project") {
		t.Fatalf("exact duplicate project label error = %v, want session name already exists in this project", err)
	}

	state, err := loadAppState(appStatePath())
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(state, "small")
	if !ok || len(project.Sessions) != 2 {
		t.Fatalf("saved project = %#v, want two sessions", project)
	}
	gotLabels := map[string]bool{}
	for _, s := range project.Sessions {
		gotLabels[s.Label] = true
	}
	if !gotLabels["Main"] || !gotLabels["main"] {
		t.Fatalf("stored labels = %#v, want verbatim casing preserved for both %q and %q", project.Sessions, "Main", "main")
	}
}

func TestRunCreateWorkerPromotesOnlyCurrentInstance(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	var renamed [][2]string
	var switched string
	var cleared []string
	manager := fakeTmuxController{
		listSessions: func() ([]session, error) {
			return []session{{Name: "tflow-v-one-a", Temporary: true, Instance: "one", Label: "a"}, {Name: "tflow-v-one-b", Temporary: true, Instance: "one", Label: "b"}, {Name: "tflow-v-two-c", Temporary: true, Instance: "two", Label: "c"}}, nil
		},
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, [2]string{oldName, newName})
			return nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			if !temporary && instanceID == "" {
				cleared = append(cleared, name)
			}
			return nil
		},
		switchClient: func(name string) error { switched = name; return nil },
	}
	err := runCreateWorker(manager, createRequest{Kind: "project", Project: "small", Current: "tflow-v-one-b", Instance: "one", Workdir: "/requested"})
	if err != nil {
		t.Fatal(err)
	}
	if len(renamed) != 2 || !strings.HasPrefix(switched, "tflow-p-") {
		t.Fatalf("renamed = %#v, switch = %q", renamed, switched)
	}
	for _, rename := range renamed {
		if !strings.HasPrefix(rename[1], "tflow-p-") {
			t.Fatalf("persistent name = %q", rename[1])
		}
	}
	if len(cleared) != 2 {
		t.Fatalf("cleared volatile markers for %q, want two sessions", cleared)
	}
	state, err := loadAppState(appStatePath())
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(state, "small")
	if !ok || project.Workdir != "/requested" {
		t.Fatalf("saved project = %#v, want workdir %q", project, "/requested")
	}
}

func TestPromoteVolatileSessionsKeepsMarkersWhenStateSaveFails(t *testing.T) {
	var cleared []string
	var killed []string
	var renamed [][2]string
	manager := fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, [2]string{oldName, newName})
			return nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			cleared = append(cleared, name)
			return nil
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}
	m, err := buildModel(manager, "tflow-v-one-a")
	if err != nil {
		t.Fatal(err)
	}
	m.instanceID = "one"
	m.statePath = t.TempDir()
	m.sessions = []session{{Name: "tflow-v-one-a", Temporary: true, Instance: "one", Label: "a"}}

	if err := m.promoteVolatileSessions("small", "/requested", "tflow-v-one-a"); err == nil {
		t.Fatal("promoteVolatileSessions returned nil error for a directory state path")
	}
	if len(cleared) != 0 {
		t.Fatalf("cleared volatile markers before saving state: %q", cleared)
	}
	if len(killed) != 0 {
		t.Fatalf("killed sessions = %q, want none", killed)
	}
	if len(renamed) != 2 || !strings.HasPrefix(renamed[0][1], "tflow-p-") || renamed[1] != [2]string{renamed[0][1], "tflow-v-one-a"} {
		t.Fatalf("renames = %#v, want a promotion followed by rollback", renamed)
	}
}

func TestPromoteVolatileSessionsRestoresRenamesOnMidLoopRenameFailure(t *testing.T) {
	var renamed [][2]string
	var killed []string
	var cleared []string
	renameCalls := 0
	manager := fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renamed = append(renamed, [2]string{oldName, newName})
			renameCalls++
			if renameCalls == 2 {
				return fmt.Errorf("tmux: rename failed")
			}
			return nil
		},
		setSessionTemporary: func(name string, temporary bool, instanceID string) error {
			cleared = append(cleared, name)
			return nil
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}
	m, err := buildModel(manager, "tflow-v-one-a")
	if err != nil {
		t.Fatal(err)
	}
	m.instanceID = "one"
	m.statePath = t.TempDir() + "/store.json"
	m.sessions = []session{
		{Name: "tflow-v-one-a", Temporary: true, Instance: "one", Label: "a"},
		{Name: "tflow-v-one-b", Temporary: true, Instance: "one", Label: "b"},
	}

	err = m.promoteVolatileSessions("small", "/requested", "tflow-v-one-a")
	if err == nil || !strings.Contains(err.Error(), "rename failed") {
		t.Fatalf("promoteVolatileSessions error = %v, want the original rename failure", err)
	}
	// Three rename calls: promote(a) succeeds, promote(b) fails, restore(a)
	// back to its volatile name.
	if len(renamed) != 3 {
		t.Fatalf("rename calls = %#v, want promote(a), failed promote(b), restore(a)", renamed)
	}
	if !strings.HasPrefix(renamed[0][1], "tflow-p-") {
		t.Fatalf("first promotion rename = %#v, want a persistent-style target", renamed[0])
	}
	if renamed[2] != [2]string{renamed[0][1], "tflow-v-one-a"} {
		t.Fatalf("restore rename = %#v, want %q -> %q", renamed[2], renamed[0][1], "tflow-v-one-a")
	}
	if len(killed) != 0 {
		t.Fatalf("killed sessions = %q, want none since the restore rename succeeded (no orphaned persistent name)", killed)
	}
	if len(cleared) != 0 {
		t.Fatalf("cleared volatile markers before saving state: %q, want none (no leftover ownership marker changes)", cleared)
	}
	if _, err := os.Stat(m.statePath); err == nil {
		t.Fatal("state file was written despite a mid-loop promotion failure")
	}
}

func TestPromoteVolatileSessionsCleansUpWhenRestoreRenameAlsoFails(t *testing.T) {
	var buf bytes.Buffer
	original := diag.Output
	diag.Output = &buf
	t.Cleanup(func() { diag.Output = original })

	var killed []string
	renameCalls := 0
	manager := fakeTmuxController{
		renameSession: func(oldName, newName string) error {
			renameCalls++
			switch renameCalls {
			case 2:
				return fmt.Errorf("tmux: rename failed")
			case 3:
				return fmt.Errorf("tmux: restore rename failed")
			default:
				return nil
			}
		},
		killSession: func(name string) error {
			killed = append(killed, name)
			return nil
		},
	}
	m, err := buildModel(manager, "tflow-v-one-a")
	if err != nil {
		t.Fatal(err)
	}
	m.instanceID = "one"
	m.statePath = t.TempDir() + "/store.json"
	m.sessions = []session{
		{Name: "tflow-v-one-a", Temporary: true, Instance: "one", Label: "a"},
		{Name: "tflow-v-one-b", Temporary: true, Instance: "one", Label: "b"},
	}

	err = m.promoteVolatileSessions("small", "/requested", "tflow-v-one-a")
	if err == nil || !strings.Contains(err.Error(), "rename failed") || strings.Contains(err.Error(), "restore") {
		t.Fatalf("promoteVolatileSessions error = %v, want the original promotion error unmasked by the restore failure", err)
	}
	if len(killed) != 1 {
		t.Fatalf("killed sessions = %q, want the one stranded promoted session killed as a fallback", killed)
	}
	if !strings.Contains(buf.String(), "restore promoted session") {
		t.Fatalf("diagnostic output = %q, want a restore-failure diagnostic", buf.String())
	}
}

func TestCreateVolatileSessionUsesActivePaneDirectory(t *testing.T) {
	var command string
	m := newModel(fakeTmuxController{
		currentPaneDir: func() (string, error) { return "/active/pane", nil },
		runBackground: func(value string) error {
			command = value
			return nil
		},
	}, "tflow-v-one-a").(model)
	m.instanceID = "one"
	m.mode = inputCreateSession
	m.input.SetValue("notes")

	updated, closeMenu := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	if closeMenu == nil {
		t.Fatal("expected close-menu command")
	}
	if got := updated.(model); got.mode != inputNone {
		t.Fatalf("mode = %v, want inputNone", got.mode)
	}
	request := createRequestFromCommand(t, command)
	if request.Project != "" || request.Workdir != "/active/pane" {
		t.Fatalf("request = %#v, want an unscoped active-pane request", request)
	}
}

func createRequestFromCommand(t *testing.T, command string) createRequest {
	t.Helper()
	index := strings.LastIndex(command, " create-worker ")
	if index < 0 {
		t.Fatalf("create worker command = %q", command)
	}
	encoded := strings.Trim(strings.TrimSpace(command[index+len(" create-worker "):]), "'")
	payload, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var request createRequest
	if err := json.Unmarshal(payload, &request); err != nil {
		t.Fatal(err)
	}
	return request
}
