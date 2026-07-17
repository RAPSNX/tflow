package ui

import "tflow/internal/store"

type appState = store.AppState
type clusterConfig = store.ClusterConfig
type projectConfig = store.ProjectConfig

func loadAppState(path string) (appState, error) {
	return store.LoadAppState(path)
}

func saveAppState(path string, state appState) error {
	return store.SaveAppState(path, state)
}

func normalizeAppState(state appState) appState {
	return store.NormalizeAppState(state)
}

func ensureStartupState() error {
	return store.EnsureStartupState()
}

func appStatePath() string {
	return store.AppStatePath()
}

func normalizeProjectConfig(cfg projectConfig) projectConfig {
	return store.NormalizeProjectConfig(cfg)
}
