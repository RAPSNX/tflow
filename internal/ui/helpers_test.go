package ui

import "testing"

func TestMergeAppStatesPreservesConcurrentDisjointChanges(t *testing.T) {
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	latest := appState{Projects: []storedProject{
		{Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}}},
		{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{{ID: "tflow-p-two", Label: "two"}}},
	}}
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "renamed"}},
	}}}

	merged := mergeAppStates(latest, base, desired)
	if len(merged.Projects) != 2 {
		t.Fatalf("projects = %#v, want both changes", merged.Projects)
	}
	first, ok := storedProjectByName(merged, "small")
	if !ok || len(first.Sessions) != 1 || first.Sessions[0].Label != "renamed" {
		t.Fatalf("small project = %#v, want renamed session", first)
	}
	second, ok := storedProjectByName(merged, "garden")
	if !ok || len(second.Sessions) != 1 || second.Sessions[0].ID != "tflow-p-two" {
		t.Fatalf("garden project = %#v, want concurrent session", second)
	}
}

func TestSaveStatePreservesConcurrentDisjointChanges(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	if err := saveAppState(path, base); err != nil {
		t.Fatal(err)
	}

	first, err := buildModel(fakeTmuxController{}, "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := buildModel(fakeTmuxController{}, "")
	if err != nil {
		t.Fatal(err)
	}
	first.sessions = []session{{Name: "tflow-p-one"}}
	first.sessionLabels["tflow-p-one"] = "renamed"
	second.sessions = []session{{Name: "tflow-p-one"}, {Name: "tflow-p-two"}}
	second.projects = append(second.projects, "garden")
	second.projectConfigs["garden"] = projectConfig{Name: "garden", Workdir: "/garden"}
	second.sessionProjects["tflow-p-two"] = "garden"
	second.sessionLabels["tflow-p-two"] = "two"

	if err := first.saveState(); err != nil {
		t.Fatal(err)
	}
	if err := second.saveState(); err != nil {
		t.Fatal(err)
	}

	state, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	small, ok := storedProjectByName(state, "small")
	if !ok || len(small.Sessions) != 1 || small.Sessions[0].Label != "renamed" {
		t.Fatalf("small project = %#v, want preserved rename", small)
	}
	garden, ok := storedProjectByName(state, "garden")
	if !ok || len(garden.Sessions) != 1 || garden.Sessions[0].ID != "tflow-p-two" {
		t.Fatalf("garden project = %#v, want concurrent session", garden)
	}
}
