package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type AppState struct {
	Projects         []string          `json:"projects"`
	SessionProjects  map[string]string `json:"session_projects"`
	SessionTypes     map[string]string `json:"session_types"`
	ProjectDirs      map[string]string `json:"project_dirs"`
	ExpandedProjects map[string]bool   `json:"expanded_projects"`
}

func AppStatePath() string {
	return filepath.Join(ConfigDir(), "state.json")
}

func LoadAppState(path string) (AppState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AppState{
				Projects:         []string{},
				SessionProjects:  map[string]string{},
				SessionTypes:     map[string]string{},
				ProjectDirs:      map[string]string{},
				ExpandedProjects: map[string]bool{},
			}, nil
		}
		return AppState{}, err
	}
	var state AppState
	if err := json.Unmarshal(data, &state); err != nil {
		return AppState{}, err
	}
	return NormalizeAppState(state), nil
}

func SaveAppState(path string, state AppState) error {
	state = NormalizeAppState(state)
	data, err := json.MarshalIndent(state, "", "  ")
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
	state = NormalizeAppState(state)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.ExpandedProjects == nil {
		state.ExpandedProjects = map[string]bool{}
	}
	return SaveAppState(path, state)
}

func NormalizeAppState(state AppState) AppState {
	state.Projects = normalizeProjectList(state.Projects)
	if state.SessionProjects == nil {
		state.SessionProjects = map[string]string{}
	}
	if state.SessionTypes == nil {
		state.SessionTypes = map[string]string{}
	}
	if state.ProjectDirs == nil {
		state.ProjectDirs = map[string]string{}
	}
	if state.ExpandedProjects == nil {
		state.ExpandedProjects = map[string]bool{}
	}
	for _, project := range state.Projects {
		if _, ok := state.ExpandedProjects[project]; !ok {
			state.ExpandedProjects[project] = true
		}
	}
	for name, project := range state.SessionProjects {
		normalized := normalizeProjectName(project)
		state.SessionProjects[name] = normalized
		if !containsString(state.Projects, normalized) {
			state.Projects = append(state.Projects, normalized)
		}
		if _, ok := state.ExpandedProjects[normalized]; !ok {
			state.ExpandedProjects[normalized] = true
		}
		if _, ok := state.SessionTypes[name]; !ok {
			state.SessionTypes[name] = "terminal"
		}
	}
	for project, dir := range state.ProjectDirs {
		normalized := normalizeProjectName(project)
		if normalized == "" || strings.TrimSpace(dir) == "" {
			delete(state.ProjectDirs, project)
			continue
		}
		delete(state.ProjectDirs, project)
		state.ProjectDirs[normalized] = normalizeCWD(dir)
		if !containsString(state.Projects, normalized) {
			state.Projects = append(state.Projects, normalized)
		}
	}
	state.Projects = normalizeProjectList(state.Projects)
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
	if len(result) > 1 {
		sort.Strings(result)
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
