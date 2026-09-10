package store

import (
	"fmt"
	"strings"
	"unicode"
)

// ValidateAppState rejects semantically invalid state instead of letting
// NormalizeAppState silently drop or synthesize records for it: empty or
// duplicate normalized project names, empty or duplicate session IDs, empty
// session labels, and duplicate labels within one project.
func ValidateAppState(state AppState) error {
	seenProjects := map[string]struct{}{}
	seenSessions := map[string]struct{}{}
	for pi, project := range state.Projects {
		name := NormalizeProjectName(project.Name)
		if name == "" {
			return fmt.Errorf("invalid state: projects[%d]: project name is empty after normalization", pi)
		}
		if _, exists := seenProjects[name]; exists {
			return fmt.Errorf("invalid state: projects[%d]: duplicate project name %q", pi, name)
		}
		seenProjects[name] = struct{}{}
		seenLabels := map[string]struct{}{}
		seenAgent := false
		for si, session := range project.Sessions {
			id := strings.TrimSpace(session.ID)
			if id == "" {
				return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d]: session id is empty", pi, name, si)
			}
			if _, exists := seenSessions[id]; exists {
				return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d]: duplicate session id %q", pi, name, si, id)
			}
			seenSessions[id] = struct{}{}
			label := strings.TrimSpace(session.Label)
			if label == "" {
				return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d] (%q): session label is empty", pi, name, si, id)
			}
			if _, exists := seenLabels[label]; exists {
				return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d] (%q): duplicate label %q", pi, name, si, id, label)
			}
			seenLabels[label] = struct{}{}

			sessionType := strings.TrimSpace(session.Type)
			switch sessionType {
			case "", SessionTypeTerminal, SessionTypeGit:
				if strings.TrimSpace(session.Command) != "" {
					return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d] (%q): type %q must not have a command", pi, name, si, id, fallbackType(sessionType))
				}
			case SessionTypeAgent:
				if strings.TrimSpace(session.Command) == "" {
					return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d] (%q): agent session requires a command", pi, name, si, id)
				}
				if seenAgent {
					return fmt.Errorf("invalid state: projects[%d] (%q): duplicate agent session", pi, name)
				}
				seenAgent = true
			default:
				return fmt.Errorf("invalid state: projects[%d] (%q).sessions[%d] (%q): unknown session type %q", pi, name, si, id, sessionType)
			}
		}
	}
	return nil
}

func fallbackType(sessionType string) string {
	if sessionType == "" {
		return SessionTypeTerminal
	}
	return sessionType
}

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
		normalizedProject := Project{Name: name, Workdir: strings.TrimSpace(project.Workdir), AgentBinary: strings.TrimSpace(project.AgentBinary), Sessions: make([]PersistentSession, 0, len(project.Sessions))}
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
			sessionType := strings.TrimSpace(session.Type)
			command := strings.TrimSpace(session.Command)
			if sessionType != SessionTypeAgent {
				command = ""
			}
			normalizedProject.Sessions = append(normalizedProject.Sessions, PersistentSession{ID: id, Label: label, Type: sessionType, Command: command})
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
