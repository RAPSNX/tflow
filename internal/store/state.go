package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type AppState struct {
	Projects        []string                 `json:"projects"`
	SessionProjects map[string]string        `json:"session_projects"`
	SessionTypes    map[string]string        `json:"session_types"`
	ProjectConfigs  map[string]ProjectConfig `json:"project_configs"`
}

type storedState struct {
	ProjectOrder    []string                 `json:"project_order"`
	Projects        map[string]storedProject `json:"projects"`
	SessionProjects map[string]string        `json:"session_projects"`
	SessionTypes    map[string]string        `json:"session_types"`
}

type storedProject struct {
	Workdir     string         `json:"workdir,omitempty"`
	Protect     bool           `json:"protect,omitempty"`
	AgentBinary string         `json:"agent_binary,omitempty"`
	Cluster     *storedCluster `json:"cluster,omitempty"`
}

type storedCluster struct {
	Path          string `json:"path,omitempty"`
	ConnectionCmd string `json:"connection_cmd,omitempty"`
}

type legacyStringListState struct {
	Projects         []string          `json:"projects"`
	SessionProjects  map[string]string `json:"session_projects"`
	SessionTypes     map[string]string `json:"session_types"`
	ProjectDirs      map[string]string `json:"project_dirs"`
	ExpandedProjects map[string]bool   `json:"expanded_projects"`
}

type legacySessionSnapshotState struct {
	Projects []legacyProject `json:"projects"`
}

type legacyProject struct {
	Name     string          `json:"name"`
	Sessions []legacySession `json:"sessions"`
}

type legacySession struct {
	Name     string `json:"name"`
	TmuxName string `json:"tmux_name"`
	Type     string `json:"type"`
}

func AppStatePath() string {
	return filepath.Join(stateHomeDir(), "tflow", "store.json")
}

func LoadAppState(path string) (AppState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if legacyState, ok, err := loadLegacyState(path); ok {
				return legacyState, err
			}
			return emptyAppState(), nil
		}
		return AppState{}, err
	}

	state, err := decodeAppState(data)
	if err != nil {
		return AppState{}, err
	}
	return state, nil
}

func SaveAppState(path string, state AppState) error {
	state = NormalizeAppState(state)
	data, err := encodeAppState(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func EnsureStartupState() error {
	path := AppStatePath()
	state, err := LoadAppState(path)
	if err != nil {
		return err
	}
	return SaveAppState(path, NormalizeAppState(state))
}

func NormalizeAppState(state AppState) AppState {
	state.Projects = normalizeProjectList(state.Projects)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.SessionTypes == nil {
		state.SessionTypes = map[string]string{}
	}
	if state.ProjectConfigs == nil {
		state.ProjectConfigs = map[string]ProjectConfig{}
	}
	for _, name := range sortedStringKeys(state.SessionProjects) {
		project := normalizeProjectName(state.SessionProjects[name])
		if project == "" {
			delete(state.SessionProjects, name)
		} else {
			state.SessionProjects[name] = project
			if !containsString(state.Projects, project) {
				state.Projects = append(state.Projects, project)
			}
		}
		if _, ok := state.SessionTypes[name]; !ok {
			state.SessionTypes[name] = "terminal"
		}
	}

	normalizedConfigs := map[string]ProjectConfig{}
	for _, project := range sortedStringKeys(state.ProjectConfigs) {
		cfg := state.ProjectConfigs[project]
		normalized := normalizeProjectName(project)
		if cfg.Name != "" {
			normalized = normalizeProjectName(cfg.Name)
		}
		if normalized == "" {
			continue
		}
		cfg.Name = normalized
		normalizedConfigs[normalized] = NormalizeProjectConfig(cfg)
		if !containsString(state.Projects, normalized) {
			state.Projects = append(state.Projects, normalized)
		}
	}
	state.ProjectConfigs = normalizedConfigs
	state.Projects = normalizeProjectList(state.Projects)
	for _, project := range state.Projects {
		cfg := NormalizeProjectConfig(state.ProjectConfigs[project])
		cfg.Name = project
		state.ProjectConfigs[project] = cfg
	}
	return state
}

func normalizeProjectList(projects []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		if _, ok := seen[project]; ok {
			continue
		}
		seen[project] = struct{}{}
		result = append(result, project)
	}
	return result
}

func normalizeProjectName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == '-', r == '_', unicode.IsSpace(r), r == '/', r == '.':
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func sortedStringKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	if len(keys) > 1 {
		slicesSort(keys)
	}
	return keys
}

func slicesSort(values []string) {
	for i := 1; i < len(values); i++ {
		current := values[i]
		j := i - 1
		for j >= 0 && values[j] > current {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = current
	}
}

func emptyAppState() AppState {
	return AppState{
		Projects:        []string{},
		SessionProjects: map[string]string{},
		SessionTypes:    map[string]string{},
		ProjectConfigs:  map[string]ProjectConfig{},
	}
}

func decodeAppState(data []byte) (AppState, error) {
	var envelope struct {
		Projects json.RawMessage `json:"projects"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return AppState{}, err
	}

	projectsValue := bytes.TrimSpace(envelope.Projects)
	if len(projectsValue) == 0 || projectsValue[0] == '{' {
		return decodeStoredState(data)
	}
	if projectsValue[0] != '[' {
		return AppState{}, fmt.Errorf("invalid projects field")
	}

	elementKind, err := arrayElementKind(projectsValue)
	if err != nil {
		return AppState{}, err
	}
	switch elementKind {
	case '"', 0:
		return decodeLegacyStringListState(data)
	case '{':
		return decodeLegacySessionSnapshotState(data)
	default:
		return AppState{}, fmt.Errorf("unsupported projects array format")
	}
}

func encodeAppState(state AppState) ([]byte, error) {
	stored := storedState{
		ProjectOrder:    append([]string(nil), state.Projects...),
		Projects:        map[string]storedProject{},
		SessionProjects: cloneStringMap(state.SessionProjects),
		SessionTypes:    cloneStringMap(state.SessionTypes),
	}

	for _, project := range state.Projects {
		cfg := NormalizeProjectConfig(state.ProjectConfigs[project])
		projectState := storedProject{
			Workdir:     cfg.Workdir,
			Protect:     cfg.Protect,
			AgentBinary: cfg.AgentBinary,
		}
		if cfg.Cluster.Path != "" || cfg.Cluster.ConnectionCmd != "" {
			projectState.Cluster = &storedCluster{
				Path:          cfg.Cluster.Path,
				ConnectionCmd: cfg.Cluster.ConnectionCmd,
			}
		}
		stored.Projects[project] = projectState
	}

	return json.MarshalIndent(stored, "", "  ")
}

func decodeStoredState(data []byte) (AppState, error) {
	var stored storedState
	if err := json.Unmarshal(data, &stored); err != nil {
		return AppState{}, err
	}

	state := emptyAppState()
	state.Projects = append(state.Projects, stored.ProjectOrder...)

	extraProjects := make([]string, 0, len(stored.Projects))
	for project, cfg := range stored.Projects {
		project = normalizeProjectName(project)
		if project == "" {
			continue
		}
		extraProjects = append(extraProjects, project)
		projectConfig := ProjectConfig{
			Name:        project,
			Workdir:     cfg.Workdir,
			Protect:     cfg.Protect,
			AgentBinary: cfg.AgentBinary,
		}
		if cfg.Cluster != nil {
			projectConfig.Cluster = ClusterConfig{
				Path:          cfg.Cluster.Path,
				ConnectionCmd: cfg.Cluster.ConnectionCmd,
			}
		}
		state.ProjectConfigs[project] = NormalizeProjectConfig(projectConfig)
	}
	slicesSort(extraProjects)
	for _, project := range extraProjects {
		if !containsString(state.Projects, project) {
			state.Projects = append(state.Projects, project)
		}
	}

	for name, project := range stored.SessionProjects {
		state.SessionProjects[name] = project
	}
	for name, sessionType := range stored.SessionTypes {
		state.SessionTypes[name] = sessionType
	}

	return NormalizeAppState(state), nil
}

func decodeLegacyStringListState(data []byte) (AppState, error) {
	var legacy legacyStringListState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return AppState{}, err
	}
	return NormalizeAppState(AppState{
		Projects:        append([]string(nil), legacy.Projects...),
		SessionProjects: cloneStringMap(legacy.SessionProjects),
		SessionTypes:    cloneStringMap(legacy.SessionTypes),
		ProjectConfigs:  legacyProjectConfigs(legacy.ProjectDirs),
	}), nil
}

func decodeLegacySessionSnapshotState(data []byte) (AppState, error) {
	var legacy legacySessionSnapshotState
	if err := json.Unmarshal(data, &legacy); err != nil {
		return AppState{}, err
	}

	state := emptyAppState()
	for _, project := range legacy.Projects {
		name := normalizeProjectName(project.Name)
		if name == "" {
			continue
		}
		state.Projects = append(state.Projects, name)
		for _, session := range project.Sessions {
			sessionKey := strings.TrimSpace(session.TmuxName)
			if sessionKey == "" {
				sessionKey = strings.TrimSpace(session.Name)
			}
			if sessionKey == "" {
				continue
			}
			state.SessionProjects[sessionKey] = name
			if strings.TrimSpace(session.Type) == "" {
				state.SessionTypes[sessionKey] = "terminal"
				continue
			}
			state.SessionTypes[sessionKey] = strings.TrimSpace(session.Type)
		}
	}

	return NormalizeAppState(state), nil
}

func arrayElementKind(raw json.RawMessage) (byte, error) {
	var elements []json.RawMessage
	if err := json.Unmarshal(raw, &elements); err != nil {
		return 0, err
	}
	for _, element := range elements {
		element = bytes.TrimSpace(element)
		if len(element) == 0 {
			continue
		}
		return element[0], nil
	}
	return 0, nil
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func legacyProjectConfigs(projectDirs map[string]string) map[string]ProjectConfig {
	configs := map[string]ProjectConfig{}
	for project, dir := range projectDirs {
		project = normalizeProjectName(project)
		dir = strings.TrimSpace(dir)
		if project == "" || dir == "" {
			continue
		}
		configs[project] = NormalizeProjectConfig(ProjectConfig{
			Name:    project,
			Workdir: dir,
		})
	}
	return configs
}

func loadLegacyState(path string) (AppState, bool, error) {
	if path != AppStatePath() {
		return AppState{}, false, nil
	}

	legacyPath := legacyAppStatePath()
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return AppState{}, false, nil
		}
		return AppState{}, true, err
	}

	state, err := decodeAppState(data)
	if err != nil {
		return AppState{}, true, fmt.Errorf("decode legacy state %q: %w", legacyPath, err)
	}
	return state, true, nil
}

func stateHomeDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".local", "state")
	}
	return filepath.Join(".", ".local", "state")
}

func legacyAppStatePath() string {
	return filepath.Join(configHomeDir(), "tflow", "state.json")
}

func configHomeDir() string {
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".config")
	}
	return "."
}
