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

func TestMergeAppStatesPreservesConcurrentProjectWorkdirDuringSessionChange(t *testing.T) {
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	latest := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/new", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "renamed"}},
	}}}

	merged := mergeAppStates(latest, base, desired)
	project, ok := storedProjectByName(merged, "small")
	if !ok || project.Workdir != "/new" || len(project.Sessions) != 1 || project.Sessions[0].Label != "renamed" {
		t.Fatalf("project = %#v, want the concurrent workdir and desired session rename", project)
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

func TestMergeAppStatesPreservesDesiredSessionOrder(t *testing.T) {
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-first", Label: "first"},
			{ID: "tflow-p-second", Label: "second"},
		},
	}}}
	merged := mergeAppStates(appState{}, appState{}, desired)
	project, ok := storedProjectByName(merged, "small")
	if !ok || len(project.Sessions) != 2 {
		t.Fatalf("project = %#v", project)
	}
	if project.Sessions[0].ID != "tflow-p-first" || project.Sessions[1].ID != "tflow-p-second" {
		t.Fatalf("session order = %#v, want first then second", project.Sessions)
	}
}

func TestSaveStateRejectsConcurrentDuplicateProjectLabel(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	base := appState{Projects: []storedProject{{Name: "small", Workdir: "/small", Sessions: []persistentSession{}}}}
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
	for _, model := range []*model{&first, &second} {
		model.sessions = []session{{Name: "tflow-p-one"}}
		if model == &second {
			model.sessions[0].Name = "tflow-p-two"
		}
		model.sessionProjects[model.sessions[0].Name] = "small"
		model.sessionLabels[model.sessions[0].Name] = "code"
	}
	if err := first.saveState(); err != nil {
		t.Fatal(err)
	}
	if err := second.saveState(); err == nil {
		t.Fatal("second save accepted a duplicate project label")
	}
	state, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(state, "small")
	if !ok || len(project.Sessions) != 1 || project.Sessions[0].ID != "tflow-p-one" {
		t.Fatalf("project = %#v, want only the first session", project)
	}
}

func TestSaveStateRejectsConcurrentProjectRenameCollision(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	path := appStatePath()
	base := appState{Projects: []storedProject{
		{Name: "small", Workdir: "/small", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}}},
		{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{{ID: "tflow-p-two", Label: "two"}}},
	}}
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
	for _, model := range []*model{&first, &second} {
		model.sessions = []session{{Name: "tflow-p-one"}, {Name: "tflow-p-two"}}
	}
	first.projects = []string{"workspace", "garden"}
	delete(first.projectConfigs, "small")
	first.projectConfigs["workspace"] = projectConfig{Name: "workspace", Workdir: "/small"}
	first.sessionProjects["tflow-p-one"] = "workspace"
	second.projects = []string{"small", "workspace"}
	delete(second.projectConfigs, "garden")
	second.projectConfigs["workspace"] = projectConfig{Name: "workspace", Workdir: "/garden"}
	second.sessionProjects["tflow-p-two"] = "workspace"

	if err := first.saveState(); err != nil {
		t.Fatal(err)
	}
	if err := second.saveState(); err == nil || err.Error() != "project already exists" {
		t.Fatalf("second save error = %v, want project already exists", err)
	}
	state, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	workspace, ok := storedProjectByName(state, "workspace")
	if !ok || len(workspace.Sessions) != 1 || workspace.Sessions[0].ID != "tflow-p-one" {
		t.Fatalf("workspace project = %#v, want only first rename", workspace)
	}
	garden, ok := storedProjectByName(state, "garden")
	if !ok || len(garden.Sessions) != 1 || garden.Sessions[0].ID != "tflow-p-two" {
		t.Fatalf("garden project = %#v, want preserved original project", garden)
	}
}
