package store

import (
	"path/filepath"
	"testing"
)

func TestMoveSessionMovesSessionBetweenProjectsPreservingIDAndLabel(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{
			{ID: "tflow-p-1", Label: "otter"},
			{ID: "tflow-p-2", Label: "fox"},
		}},
		{Name: "garden", Workdir: "/garden", Sessions: []PersistentSession{
			{ID: "tflow-p-3", Label: "bee"},
		}},
	}}

	got, err := MoveSession(state, "tflow-p-1", "garden")
	if err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}

	small, ok := findProject(got, "small")
	if !ok || len(small.Sessions) != 1 || small.Sessions[0].ID != "tflow-p-2" {
		t.Fatalf("source project = %#v", small)
	}
	garden, ok := findProject(got, "garden")
	if !ok || len(garden.Sessions) != 2 {
		t.Fatalf("target project = %#v", garden)
	}
	if garden.Sessions[0].ID != "tflow-p-3" || garden.Sessions[1].ID != "tflow-p-1" || garden.Sessions[1].Label != "otter" {
		t.Fatalf("target sessions = %#v, want moved session appended at the end preserving id and label", garden.Sessions)
	}
}

func TestMoveSessionAppendsAtEndOfTargetProjectSessions(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		{Name: "garden", Sessions: []PersistentSession{
			{ID: "tflow-p-a", Label: "a"},
			{ID: "tflow-p-b", Label: "b"},
			{ID: "tflow-p-c", Label: "c"},
		}},
	}}
	got, err := MoveSession(state, "tflow-p-1", "garden")
	if err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}
	garden, ok := findProject(got, "garden")
	if !ok {
		t.Fatal("target project missing")
	}
	want := []string{"tflow-p-a", "tflow-p-b", "tflow-p-c", "tflow-p-1"}
	if len(garden.Sessions) != len(want) {
		t.Fatalf("sessions = %#v", garden.Sessions)
	}
	for i, id := range want {
		if garden.Sessions[i].ID != id {
			t.Fatalf("session order = %#v, want %#v", garden.Sessions, want)
		}
	}
}

func TestMoveSessionRejectsLabelCollisionInTargetProjectWithoutMutating(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		{Name: "garden", Sessions: []PersistentSession{{ID: "tflow-p-2", Label: "otter"}}},
	}}

	_, err := MoveSession(state, "tflow-p-1", "garden")
	if err == nil {
		t.Fatal("expected label collision error")
	}

	small, _ := findProject(state, "small")
	garden, _ := findProject(state, "garden")
	if len(small.Sessions) != 1 || small.Sessions[0].ID != "tflow-p-1" {
		t.Fatalf("source project mutated: %#v", small)
	}
	if len(garden.Sessions) != 1 || garden.Sessions[0].ID != "tflow-p-2" {
		t.Fatalf("target project mutated: %#v", garden)
	}
}

func TestMoveSessionDeletesSourceProjectWhenItsFinalSessionMoves(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		{Name: "garden", Sessions: []PersistentSession{{ID: "tflow-p-2", Label: "fox"}}},
	}}

	got, err := MoveSession(state, "tflow-p-1", "garden")
	if err != nil {
		t.Fatalf("MoveSession returned error: %v", err)
	}
	if _, ok := findProject(got, "small"); ok {
		t.Fatalf("source project metadata remains: %#v", got.Projects)
	}
	if len(got.Projects) != 1 || got.Projects[0].Name != "garden" {
		t.Fatalf("projects = %#v, want only garden", got.Projects)
	}
}

func TestMoveSessionRejectsUnknownSessionWithoutMutating(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
	}}
	if _, err := MoveSession(state, "tflow-p-missing", "small"); err == nil {
		t.Fatal("expected error for unknown session")
	}
}

func TestMoveSessionRejectsUnknownTargetProject(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
	}}
	if _, err := MoveSession(state, "tflow-p-1", "missing"); err == nil {
		t.Fatal("expected error for unknown target project")
	}
}

func TestMoveSessionRejectsMovingWithinSameProject(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
	}}
	if _, err := MoveSession(state, "tflow-p-1", "small"); err == nil {
		t.Fatal("expected error for moving within the same project")
	}
}

func TestMoveSessionThroughMutateAppStatePersistsAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.json")
	initial := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		{Name: "garden", Workdir: "/garden", Sessions: []PersistentSession{{ID: "tflow-p-2", Label: "fox"}}},
	}}
	if err := SaveAppState(path, initial); err != nil {
		t.Fatal(err)
	}

	if _, err := MutateAppState(path, func(state AppState) (AppState, error) {
		return MoveSession(state, "tflow-p-1", "garden")
	}); err != nil {
		t.Fatal(err)
	}

	got, err := LoadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := findProject(got, "small"); ok {
		t.Fatalf("source project remains after persisted move: %#v", got.Projects)
	}
	garden, ok := findProject(got, "garden")
	if !ok || len(garden.Sessions) != 2 || garden.Sessions[1].ID != "tflow-p-1" {
		t.Fatalf("persisted target project = %#v", garden)
	}
}

func findProject(state AppState, name string) (Project, bool) {
	for _, project := range state.Projects {
		if project.Name == name {
			return project, true
		}
	}
	return Project{}, false
}
