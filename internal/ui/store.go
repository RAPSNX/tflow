package ui

import "tflow/internal/store"

type appConfig = store.AppConfig
type appState = store.AppState
type clusterConfig = store.ClusterConfig
type projectConfig = store.ProjectConfig
type themeOverrides = store.ThemeOverrides

func loadAppConfig() (appConfig, error) {
	return store.LoadAppConfig()
}

func loadAppConfigForStatePath(statePath string) (appConfig, error) {
	return store.LoadAppConfigForStatePath(statePath)
}

func loadAppConfigForDir(baseDir string) (appConfig, error) {
	return store.LoadAppConfigForDir(baseDir)
}

func saveDefaultAppConfig() error {
	return store.SaveDefaultAppConfig()
}

func loadProjectConfigs(statePath string, state appState) (map[string]projectConfig, error) {
	return store.LoadProjectConfigs(statePath, state)
}

func saveProjectConfigs(statePath string, configs map[string]projectConfig) error {
	return store.SaveProjectConfigs(statePath, configs)
}

func removeProjectConfigFile(statePath, project string) error {
	return store.RemoveProjectConfigFile(statePath, project)
}

func marshalProjectConfig(cfg projectConfig) []byte {
	return store.MarshalProjectConfig(cfg)
}

func parseProjectConfig(data []byte) (projectConfig, error) {
	return store.ParseProjectConfig(data)
}

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
