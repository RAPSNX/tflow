package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadAppStateDefaultsToEmptyStoreWhenFileMissing(t *testing.T) {
	state, err := LoadAppState(filepath.Join(t.TempDir(), "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("state = %#v, want empty", state)
	}
}

func TestAcquireAppStateLockSerializesCallers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	unlockFirst, err := AcquireAppStateLock(path)
	if err != nil {
		t.Fatal(err)
	}

	acquired := make(chan func() error, 1)
	errs := make(chan error, 1)
	go func() {
		unlock, err := AcquireAppStateLock(path)
		if err != nil {
			errs <- err
			return
		}
		acquired <- unlock
	}()

	select {
	case <-acquired:
		t.Fatal("second caller acquired the state lock before the first released it")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(50 * time.Millisecond):
	}
	if err := unlockFirst(); err != nil {
		t.Fatal(err)
	}

	select {
	case unlockSecond := <-acquired:
		if err := unlockSecond(); err != nil {
			t.Fatal(err)
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(time.Second):
		t.Fatal("second caller did not acquire the released state lock")
	}
}

func TestSaveAndLoadAppStateRoundTripsOrderedProjectRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	want := AppState{Projects: []Project{
		{Name: "small", Workdir: "/tmp/project-small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}, {ID: "tflow-p-2", Label: "fox"}}},
		{Name: "garden", Workdir: "/tmp/project-garden", Sessions: []PersistentSession{}},
	}}
	if err := SaveAppState(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Projects) != 2 || got.Projects[0].Name != "small" || got.Projects[0].Workdir != "/tmp/project-small" || got.Projects[0].Sessions[1].Label != "fox" {
		t.Fatalf("round trip = %#v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	plain := string(data)
	for _, field := range []string{"projects", "name", "workdir", "sessions", "id", "label"} {
		if !strings.Contains(plain, field) {
			t.Fatalf("missing canonical field %s in %s", field, plain)
		}
	}
	for _, removed := range []string{"project_order", "session_projects", "session_labels", "instance"} {
		if strings.Contains(plain, removed) {
			t.Fatalf("obsolete field %q in %s", removed, plain)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestLoadAppStateIgnoresUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	data := `{"projects":[{"name":"small","workdir":"/tmp","sessions":[{"id":"tflow-p-1","label":"otter","unknown":true}]}],"unknown":true}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 1 || state.Projects[0].Sessions[0].ID != "tflow-p-1" {
		t.Fatalf("state = %#v", state)
	}
}

func TestLoadAppStateRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	if err := os.WriteFile(path, []byte(`{"projects":`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadAppState(path); err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("error = %v, want malformed state error", err)
	}
}

func TestLoadAppStateDoesNotReadLegacyConfigState(t *testing.T) {
	stateHome := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	legacyPath := filepath.Join(configHome, "tflow", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"projects":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadAppState(AppStatePath())
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 0 {
		t.Fatalf("legacy config state was loaded: %#v", state)
	}
}

func TestNormalizeProjectListMatchesFormerCallSiteBehavior(t *testing.T) {
	got := NormalizeProjectList([]string{" Small ", "default", "Alpha/One", "small", "alpha.one", "", "alpha-one"})
	want := []string{"small", "default", "alpha-one"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("NormalizeProjectList = %#v, want %#v", got, want)
	}
}

func TestNormalizeCWDFallsBackToHomeWhenCurrentDirectoryIsUnavailable(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })
	home := t.TempDir()
	t.Setenv("HOME", home)
	missingDir := filepath.Join(t.TempDir(), "missing")
	if err := os.Mkdir(missingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(missingDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(missingDir); err != nil {
		t.Fatal(err)
	}
	for _, cwd := range []string{"", "."} {
		if got := NormalizeCWD(cwd); got != home {
			t.Fatalf("NormalizeCWD(%q) = %q, want %q", cwd, got, home)
		}
	}
}
