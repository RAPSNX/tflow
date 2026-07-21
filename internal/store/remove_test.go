package store

import "testing"

func TestRemoveSessionDropsTheSessionOnly(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{
			{ID: "tflow-p-1", Label: "otter"},
			{ID: "tflow-p-2", Label: "fox"},
		}},
	}}

	got := RemoveSession(state, "tflow-p-1")

	small, ok := findProject(got, "small")
	if !ok || len(small.Sessions) != 1 || small.Sessions[0].ID != "tflow-p-2" {
		t.Fatalf("project = %#v, want only tflow-p-2 left", small)
	}
}

func TestRemoveSessionDropsProjectWhenItBecomesEmpty(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
		{Name: "garden", Workdir: "/garden", Sessions: []PersistentSession{{ID: "tflow-p-2", Label: "bee"}}},
	}}

	got := RemoveSession(state, "tflow-p-1")

	if len(got.Projects) != 1 || got.Projects[0].Name != "garden" {
		t.Fatalf("projects = %#v, want only garden left", got.Projects)
	}
}

func TestRemoveSessionIsNoOpWhenSessionIDNotFound(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
	}}

	got := RemoveSession(state, "tflow-p-missing")

	small, ok := findProject(got, "small")
	if !ok || len(small.Sessions) != 1 || small.Sessions[0].ID != "tflow-p-1" {
		t.Fatalf("project = %#v, want state unchanged", small)
	}
}

func TestRemoveSessionIsNoOpForEmptySessionID(t *testing.T) {
	state := AppState{Projects: []Project{
		{Name: "small", Workdir: "/small", Sessions: []PersistentSession{{ID: "tflow-p-1", Label: "otter"}}},
	}}

	got := RemoveSession(state, "  ")

	small, ok := findProject(got, "small")
	if !ok || len(small.Sessions) != 1 {
		t.Fatalf("project = %#v, want state unchanged", small)
	}
}
