package ui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSessionStartDirFallsBackWhenDirectoryDoesNotExist(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	missingDir := filepath.Join(t.TempDir(), "missing")
	if got := sessionStartDir(missingDir); got != home {
		t.Fatalf("sessionStartDir(%q) = %q, want %q", missingDir, got, home)
	}
}

func TestDefaultSessionDirUsesCurrentDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore current directory: %v", err)
		}
	})

	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(cwd, 0o755); err != nil {
		t.Fatalf("create current directory: %v", err)
	}
	if err := os.Chdir(cwd); err != nil {
		t.Fatalf("change current directory: %v", err)
	}

	if got := defaultSessionDir(); got != cwd {
		t.Fatalf("defaultSessionDir = %q, want %q", got, cwd)
	}
}

func TestCreateSessionRejectsDuplicateLabelWithinProject(t *testing.T) {
	m := newModel(fakeTmuxController{}, "").(model)
	m.projects = []string{"small"}
	m.sessions = []session{{Name: "small--code"}}
	m.sessionProjects = map[string]string{"small--code": "small"}
	m.sessionLabels = map[string]string{"small--code": "code"}
	m.selectedProject = "small"
	m.mode = inputCreateSession
	m.input.SetValue("code")

	updated, cmd := m.updateModal(tea.KeyMsg{Type: tea.KeyEnter})
	got := updated.(model)
	if cmd != nil {
		t.Fatal("expected no create command")
	}
	if got.status != "Session name already exists in this project." {
		t.Fatalf("status = %q", got.status)
	}
}

func TestSanitizeSessionName(t *testing.T) {
	if got, want := sanitizeSessionName(" Prod/Main 01 "), "Prod/Main 01"; got != want {
		t.Fatalf("sanitizeSessionName = %q, want %q", got, want)
	}
}

func TestProjectNormalizationPreservesOrderAndDeduplicates(t *testing.T) {
	got := normalizeProjectList([]string{"small", "default", "alpha", "small"})
	want := []string{"small", "default", "alpha"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("normalizeProjectList = %#v, want %#v", got, want)
	}
}

// TestKillingSessionWritesNoMarkersForUnrelatedSessions proves that killing
// one session never rewrites tmux markers for any other session: the killed
// session no longer exists in tmux, and no other session's project or label
// marker is affected by removing it.
func TestKillingSessionWritesNoMarkersForUnrelatedSessions(t *testing.T) {
	var projectWrites, labelWrites []string
	m := newModel(fakeTmuxController{
		setSessionProject: func(name, project string) error {
			projectWrites = append(projectWrites, name+"="+project)
			return nil
		},
		setSessionLabel: func(name, label string) error {
			labelWrites = append(labelWrites, name+"="+label)
			return nil
		},
	}, "").(model)
	m.statePath = filepath.Join(t.TempDir(), "store.json")
	m.projects = []string{"small"}
	m.projectConfigs = map[string]projectConfig{"small": {Name: "small", Workdir: "/tmp/small"}}
	m.sessions = make([]session, 0, 50)
	m.sessionProjects = map[string]string{}
	m.sessionLabels = map[string]string{}
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("small--existing-%02d", i)
		m.sessions = append(m.sessions, session{Name: name})
		m.sessionProjects[name] = "small"
		m.sessionLabels[name] = fmt.Sprintf("animal-%02d", i)
	}
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(sessionKilledMsg{name: "small--existing-00", project: "small"})
	got := updated.(model)
	if got.err != nil {
		t.Fatalf("kill reported error: %v", got.err)
	}

	if len(projectWrites) != 0 || len(labelWrites) != 0 {
		t.Fatalf("marker writes = project:%#v label:%#v, want none", projectWrites, labelWrites)
	}
}

func TestCreateVolatileSessionClearsStaleMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.sessionProjects = map[string]string{"notes": "old-project"}
	m.sessionLabels = map[string]string{"notes": "old label"}
	m.statePath = t.TempDir() + "/store.json"
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}

	updated, _ := m.Update(sessionCreatedMsg{
		session:  session{Name: "notes", Temporary: true, Instance: "instance-1"},
		volatile: true,
	})
	got := updated.(model)

	if _, ok := got.sessionProjects["notes"]; ok {
		t.Fatalf("stale project metadata remains: %#v", got.sessionProjects)
	}
	if _, ok := got.sessionLabels["notes"]; ok {
		t.Fatalf("stale label metadata remains: %#v", got.sessionLabels)
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := storedSessionByID(state, "notes"); ok {
		t.Fatalf("stale session state remains: %#v", state)
	}
}

func TestRenameVolatileSessionClearsStaleMetadata(t *testing.T) {
	m := newModel(fakeTmuxController{}, volatileSessionName("instance-1", "notes")).(model)
	m.instanceID = "instance-1"
	m.sessions = []session{{Name: volatileSessionName("instance-1", "notes"), Label: "notes", Temporary: true, Instance: "instance-1"}}
	m.currentSession = volatileSessionName("instance-1", "notes")
	m.selectedSession = volatileSessionName("instance-1", "notes")
	m.renameTarget = renameTarget{session: volatileSessionName("instance-1", "notes")}
	m.sessionProjects = map[string]string{volatileSessionName("instance-1", "notes"): "old-project", volatileSessionName("instance-1", "dev"): "other-project"}
	m.sessionLabels = map[string]string{volatileSessionName("instance-1", "notes"): "old-notes", volatileSessionName("instance-1", "dev"): "old-dev"}
	m.statePath = t.TempDir() + "/store.json"
	if err := m.saveState(); err != nil {
		t.Fatal(err)
	}
	m.input.SetValue("dev")

	updated, cmd := m.commitRename()
	if cmd == nil {
		t.Fatal("expected rename command")
	}
	msg := cmd().(sessionRenamedMsg)
	if msg.err != nil {
		t.Fatalf("rename returned error: %v", msg.err)
	}

	pending := *(updated.(*model))
	updated, followUp := pending.Update(msg)
	if followUp == nil {
		t.Fatal("expected reload command after rename")
	}
	got := updated.(model)
	if _, ok := got.sessionProjects[volatileSessionName("instance-1", "notes")]; ok {
		t.Fatalf("stale target metadata remains: %#v", got.sessionProjects)
	}
	if got.sessionLabel(volatileSessionName("instance-1", "notes")) != "dev" {
		t.Fatalf("volatile label = %q, want dev", got.sessionLabel(volatileSessionName("instance-1", "notes")))
	}
	state, err := loadAppState(m.statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes", "dev"} {
		if _, ok := storedSessionByID(state, name); ok {
			t.Fatalf("stale session state remains for %s: %#v", name, state)
		}
	}
}

func TestVolatileContextShowsOnlyCurrentInstanceSessions(t *testing.T) {
	m := newModel(fakeTmuxController{}, "scratch-temp").(model)
	m.instanceID = "instance-1"
	m.projects = []string{"project"}
	m.sessions = []session{
		{Name: "scratch-temp", Temporary: true, Instance: "instance-1"},
		{Name: "notes", Temporary: true, Instance: "instance-1"},
		{Name: "other", Temporary: true, Instance: "instance-2"},
		{Name: "project--code"},
	}
	m.sessionProjects = map[string]string{"project--code": "project"}
	m.syncSelection()

	if m.selectedProject != "" {
		t.Fatalf("selectedProject = %q, want volatile context", m.selectedProject)
	}
	var names []string
	for _, s := range m.contextSessions() {
		names = append(names, s.Name)
	}
	if got, want := fmt.Sprint(names), "[scratch-temp notes]"; got != want {
		t.Fatalf("visible sessions = %s, want %s", got, want)
	}
}

func TestNewSessionIDUsesCryptographicEntropy(t *testing.T) {
	id, err := newSessionIDWithEntropy(bytes.NewReader(make([]byte, 16)))
	if err != nil || id != "00000000000000000000000000000000" {
		t.Fatalf("newSessionIDWithEntropy = %q, %v", id, err)
	}
}

func TestPersistentSessionCreationRetriesIDCollisions(t *testing.T) {
	var names []string
	m := newModel(fakeTmuxController{createSession: func(name, cwd, command string) (session, error) {
		names = append(names, name)
		if len(names) == 1 {
			return session{}, fmt.Errorf("duplicate session")
		}
		return session{Name: name}, nil
	}}, "").(model)
	created, err := m.createPersistentSession("/tmp", "")
	if err != nil || len(names) != 2 || names[0] == names[1] || created.Name != names[1] || !strings.HasPrefix(created.Name, "tflow-p-") {
		t.Fatalf("creation = %#v, names = %#v, err = %v", created, names, err)
	}
}
