package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type AppState struct {
	Projects        []string
	SessionProjects map[string]string
	SessionLabels   map[string]string
	ProjectConfigs  map[string]ProjectConfig
}

type storedState struct {
	ProjectOrder    []string                 `json:"project_order"`
	Projects        map[string]storedProject `json:"projects"`
	SessionProjects map[string]string        `json:"session_projects"`
	SessionLabels   map[string]string        `json:"session_labels"`
}

type storedProject struct {
	Workdir string `json:"workdir"`
}

func AppStatePath() string {
	return filepath.Join(stateHomeDir(), "tflow", "store.json")
}

func LoadAppState(path string) (AppState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyAppState(), nil
		}
		return AppState{}, err
	}
	return decodeAppState(data)
}

func SaveAppState(path string, state AppState) error {
	data, err := encodeAppState(NormalizeAppState(state))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func EnsureStartupState() error {
	path := AppStatePath()
	state, err := LoadAppState(path)
	if err != nil {
		return err
	}
	return SaveAppState(path, state)
}

func NormalizeAppState(state AppState) AppState {
	state.Projects = NormalizeProjectList(state.Projects)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.SessionLabels == nil {
		state.SessionLabels = map[string]string{}
	}
	if state.ProjectConfigs == nil {
		state.ProjectConfigs = map[string]ProjectConfig{}
	}

	normalizedProjects := map[string]ProjectConfig{}
	for _, project := range sortedStringKeys(state.ProjectConfigs) {
		cfg := NormalizeProjectConfig(state.ProjectConfigs[project])
		name := NormalizeProjectName(project)
		if cfg.Name != "" {
			name = cfg.Name
		}
		if name == "" {
			continue
		}
		cfg.Name = name
		normalizedProjects[name] = cfg
		if !ContainsString(state.Projects, name) {
			state.Projects = append(state.Projects, name)
		}
	}

	normalizedSessions := map[string]string{}
	normalizedLabels := map[string]string{}
	for _, name := range sortedStringKeys(state.SessionProjects) {
		name = strings.TrimSpace(name)
		project := NormalizeProjectName(state.SessionProjects[name])
		if name == "" || project == "" {
			continue
		}
		normalizedSessions[name] = project
		if !ContainsString(state.Projects, project) {
			state.Projects = append(state.Projects, project)
		}
		label := strings.TrimSpace(state.SessionLabels[name])
		if label == "" {
			label = sessionLabelFromKey(name, project)
		}
		normalizedLabels[name] = label
	}

	state.Projects = NormalizeProjectList(state.Projects)
	for _, project := range state.Projects {
		cfg := normalizedProjects[project]
		cfg.Name = project
		normalizedProjects[project] = NormalizeProjectConfig(cfg)
	}
	state.ProjectConfigs = normalizedProjects
	state.SessionProjects = normalizedSessions
	state.SessionLabels = normalizedLabels
	return state
}

func sessionLabelFromKey(name, project string) string {
	name = strings.TrimSpace(name)
	project = NormalizeProjectName(project)
	if project == "" {
		return name
	}
	prefix := project + "--"
	if strings.HasPrefix(name, prefix) {
		return strings.TrimPrefix(name, prefix)
	}
	return name
}

func NormalizeProjectList(projects []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(projects))
	for _, project := range projects {
		project = NormalizeProjectName(project)
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

func NormalizeProjectName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	var builder strings.Builder
	lastDash := false
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastDash = false
		case r == 45, r == 95, unicode.IsSpace(r), r == 47, r == 46:
			if !lastDash && builder.Len() > 0 {
				builder.WriteByte(45)
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func ContainsString(values []string, want string) bool {
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
	sort.Strings(keys)
	return keys
}

func emptyAppState() AppState {
	return AppState{
		Projects:        []string{},
		SessionProjects: map[string]string{},
		SessionLabels:   map[string]string{},
		ProjectConfigs:  map[string]ProjectConfig{},
	}
}

func encodeAppState(state AppState) ([]byte, error) {
	stored := storedState{
		ProjectOrder:    append([]string(nil), state.Projects...),
		Projects:        make(map[string]storedProject, len(state.Projects)),
		SessionProjects: cloneStringMap(state.SessionProjects),
		SessionLabels:   cloneStringMap(state.SessionLabels),
	}
	for _, project := range state.Projects {
		stored.Projects[project] = storedProject{Workdir: state.ProjectConfigs[project].Workdir}
	}
	return json.MarshalIndent(stored, "", "  ")
}

func decodeAppState(data []byte) (AppState, error) {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var stored storedState
	if err := decoder.Decode(&stored); err != nil {
		return AppState{}, fmt.Errorf("invalid state: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return AppState{}, fmt.Errorf("invalid state: expected one JSON object")
		}
		return AppState{}, fmt.Errorf("invalid state: %w", err)
	}

	state := emptyAppState()
	state.Projects = append(state.Projects, stored.ProjectOrder...)
	for project, cfg := range stored.Projects {
		state.ProjectConfigs[project] = ProjectConfig{Name: project, Workdir: cfg.Workdir}
	}
	state.SessionProjects = cloneStringMap(stored.SessionProjects)
	state.SessionLabels = cloneStringMap(stored.SessionLabels)
	return NormalizeAppState(state), nil
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

func stateHomeDir() string {
	if dir := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(home, ".local", "state")
	}
	return filepath.Join(".", ".local", "state")
}
