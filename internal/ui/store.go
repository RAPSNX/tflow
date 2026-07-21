package ui

import "github.com/rapsnx/tflow/internal/store"

type appState = store.AppState
type storedProject = store.Project
type persistentSession = store.PersistentSession
type projectConfig = store.ProjectConfig

func loadAppState(path string) (appState, error) {
	return store.LoadAppState(path)
}

func saveAppState(path string, state appState) error {
	return store.SaveAppState(path, state)
}

func mutateAppState(path string, mutate func(appState) (appState, error)) (appState, error) {
	return store.MutateAppState(path, mutate)
}

func mutateAppStateLocked(path string, mutate func(appState) (appState, error)) (appState, error) {
	return store.MutateAppStateLocked(path, mutate)
}

func reconcileAppState(path string, snapshotSessions func() (map[string]struct{}, error)) (bool, error) {
	return store.ReconcileAppState(path, snapshotSessions)
}

// lockAppState is a variable so tests can force a lock-release failure
// without reaching into the store package's internal flock seam.
var lockAppState = func(path string) (func() error, error) {
	return store.AcquireAppStateLock(path)
}

func normalizeAppState(state appState) appState {
	return store.NormalizeAppState(state)
}

func appStatePath() string {
	return store.AppStatePath()
}

func normalizeProjectConfig(cfg projectConfig) projectConfig {
	return store.NormalizeProjectConfig(cfg)
}

func normalizeProjectName(name string) string {
	return store.NormalizeProjectName(name)
}

func normalizeProjectList(projects []string) []string {
	return store.NormalizeProjectList(projects)
}

func containsString(values []string, want string) bool {
	return store.ContainsString(values, want)
}
