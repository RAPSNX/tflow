package store

import (
	"strings"
	"unicode"
)

func NormalizeAppState(state AppState) AppState {
	normalized := AppState{Projects: make([]Project, 0, len(state.Projects))}
	seenProjects := map[string]struct{}{}
	seenSessions := map[string]struct{}{}
	for _, project := range state.Projects {
		name := NormalizeProjectName(project.Name)
		if name == "" {
			continue
		}
		if _, exists := seenProjects[name]; exists {
			continue
		}
		seenProjects[name] = struct{}{}
		normalizedProject := Project{Name: name, Workdir: strings.TrimSpace(project.Workdir), Sessions: make([]PersistentSession, 0, len(project.Sessions))}
		for _, session := range project.Sessions {
			id := strings.TrimSpace(session.ID)
			if id == "" {
				continue
			}
			if _, exists := seenSessions[id]; exists {
				continue
			}
			seenSessions[id] = struct{}{}
			label := strings.TrimSpace(session.Label)
			if label == "" {
				label = sessionLabelFromKey(id, name)
			}
			normalizedProject.Sessions = append(normalizedProject.Sessions, PersistentSession{ID: id, Label: label})
		}
		normalized.Projects = append(normalized.Projects, normalizedProject)
	}
	return normalized
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
