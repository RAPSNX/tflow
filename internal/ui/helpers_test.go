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

func TestMergeAppStatesPreservesConcurrentAgentBinaryDuringWorkdirChange(t *testing.T) {
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", AgentBinary: "codex", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	latest := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", AgentBinary: "claude", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/new", AgentBinary: "codex", Sessions: []persistentSession{{ID: "tflow-p-one", Label: "one"}},
	}}}

	merged := mergeAppStates(latest, base, desired)
	project, ok := storedProjectByName(merged, "small")
	if !ok || project.Workdir != "/new" || project.AgentBinary != "claude" {
		t.Fatalf("project = %#v, want desired workdir and the concurrent agent-binary preserved", project)
	}
}

// TestMergeAppStatesReinsertsFullProjectAfterConcurrentDeletion guards
// against a regression where saving a scalar-only field change (e.g.
// agent-binary) on a project a concurrent instance just deleted resurrected
// it with an empty Sessions slice: mergeStateProjectFields's not-found
// fallback used to call ensureStateProject, which only carries scalar
// fields, and the later per-session merge loop skips every session whose
// desired value still matches base -- silently dropping them all.
func TestMergeAppStatesReinsertsFullProjectAfterConcurrentDeletion(t *testing.T) {
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", AgentBinary: "codex", Sessions: []persistentSession{
			{ID: "tflow-p-one", Label: "one"},
			{ID: "tflow-p-two", Label: "two"},
		},
	}}}
	// A concurrent instance deleted project "small" entirely.
	latest := appState{}
	// This instance's editor still has "small" open, unaware of the
	// deletion, and saves only a changed agent-binary -- every session is
	// otherwise identical to base.
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/old", AgentBinary: "claude", Sessions: []persistentSession{
			{ID: "tflow-p-one", Label: "one"},
			{ID: "tflow-p-two", Label: "two"},
		},
	}}}

	merged := mergeAppStates(latest, base, desired)
	project, ok := storedProjectByName(merged, "small")
	if !ok {
		t.Fatalf("project %#v missing after reinsertion, merged = %#v", "small", merged)
	}
	if project.AgentBinary != "claude" {
		t.Fatalf("project.AgentBinary = %q, want the desired agent-binary", project.AgentBinary)
	}
	if len(project.Sessions) != 2 {
		t.Fatalf("project.Sessions = %#v, want both original sessions preserved, not dropped", project.Sessions)
	}
}

// TestMergeAppStatesPreservesConcurrentRenameDuringCommandChange guards
// against a regression where mergeAppStates's per-session merge replaced a
// session's entire stored tuple with desired's version whenever any field
// (e.g. an agent's command) differed from base, discarding a concurrent
// instance's already-saved rename of the same session.
func TestMergeAppStatesPreservesConcurrentRenameDuringCommandChange(t *testing.T) {
	base := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "agent", Type: sessionTypeAgent, Command: "old-codex"},
		},
	}}}
	// A concurrent instance renamed the session.
	latest := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "renamed", Type: sessionTypeAgent, Command: "old-codex"},
		},
	}}}
	// This instance's editor only changed the agent's command.
	desired := appState{Projects: []storedProject{{
		Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "agent", Type: sessionTypeAgent, Command: "new-codex"},
		},
	}}}

	merged := mergeAppStates(latest, base, desired)
	project, ok := storedProjectByName(merged, "small")
	if !ok || len(project.Sessions) != 1 {
		t.Fatalf("project = %#v, want the single session preserved", project)
	}
	session := project.Sessions[0]
	if session.Label != "renamed" {
		t.Fatalf("session.Label = %q, want the concurrent rename preserved", session.Label)
	}
	if session.Command != "new-codex" {
		t.Fatalf("session.Command = %q, want the desired command applied", session.Command)
	}
}

// TestMergeAppStatesPreservesConcurrentMoveDuringCommandChange guards the
// same merge against a concurrent move to a different project.
func TestMergeAppStatesPreservesConcurrentMoveDuringCommandChange(t *testing.T) {
	base := appState{Projects: []storedProject{
		{Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "agent", Type: sessionTypeAgent, Command: "old-codex"},
		}},
		{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{}},
	}}
	// A concurrent instance moved the session into "garden".
	latest := appState{Projects: []storedProject{
		{Name: "small", Workdir: "/small", Sessions: []persistentSession{}},
		{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "agent", Type: sessionTypeAgent, Command: "old-codex"},
		}},
	}}
	// This instance's editor, still showing the session under "small", only
	// changed the agent's command.
	desired := appState{Projects: []storedProject{
		{Name: "small", Workdir: "/small", Sessions: []persistentSession{
			{ID: "tflow-p-agent", Label: "agent", Type: sessionTypeAgent, Command: "new-codex"},
		}},
		{Name: "garden", Workdir: "/garden", Sessions: []persistentSession{}},
	}}

	merged := mergeAppStates(latest, base, desired)
	garden, ok := storedProjectByName(merged, "garden")
	if !ok || len(garden.Sessions) != 1 || garden.Sessions[0].Command != "new-codex" {
		t.Fatalf("garden project = %#v, want the moved session with the desired command", garden)
	}
	small, ok := storedProjectByName(merged, "small")
	if !ok || len(small.Sessions) != 0 {
		t.Fatalf("small project = %#v, want the session gone (moved away)", small)
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

func TestSaveStateRejectsConcurrentDuplicateAgentSession(t *testing.T) {
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
	// Two clients independently provision an agent session for the same
	// project under different labels, so the fast in-memory label check each
	// one runs on its own never sees the other's addition.
	first.assignSessionProject("tflow-p-agent-1", "small")
	first.setSessionLabel("tflow-p-agent-1", "agent")
	first.setSessionType("tflow-p-agent-1", sessionTypeAgent)
	first.setSessionCommand("tflow-p-agent-1", "codex")

	second.assignSessionProject("tflow-p-agent-2", "small")
	second.setSessionLabel("tflow-p-agent-2", "agent-2")
	second.setSessionType("tflow-p-agent-2", sessionTypeAgent)
	second.setSessionCommand("tflow-p-agent-2", "codex")

	if err := first.saveState(); err != nil {
		t.Fatal(err)
	}
	if err := second.saveState(); err == nil {
		t.Fatal("second save accepted a second agent session for the same project")
	}

	state, err := loadAppState(path)
	if err != nil {
		t.Fatal(err)
	}
	project, ok := storedProjectByName(state, "small")
	if !ok || len(project.Sessions) != 1 || project.Sessions[0].ID != "tflow-p-agent-1" {
		t.Fatalf("project = %#v, want only the first agent session", project)
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
