package store

import (
	"sort"
	"strings"
	"unicode"
)

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
