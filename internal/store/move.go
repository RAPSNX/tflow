package store

import (
	"fmt"
	"strings"
)

// MoveSession moves the persistent session identified by sessionID out of
// its current project and into targetProject, preserving its ID and label.
// It is a pure state transformation: callers persist the result through the
// existing advisory-lock mutation path (MutateAppState / MutateAppStateLocked).
//
// The move is rejected, and state is returned unchanged aside from the zero
// value, when:
//   - sessionID does not exist in state
//   - targetProject does not exist in state
//   - sessionID already belongs to targetProject
//   - targetProject already has a session whose label exactly matches the
//     moved session's label
//
// When removing the session leaves its source project without any sessions,
// the source project is dropped entirely, matching the convention already
// used by reconciliation for projects left without sessions.
func MoveSession(state AppState, sessionID, targetProject string) (AppState, error) {
	sessionID = strings.TrimSpace(sessionID)
	targetProject = NormalizeProjectName(targetProject)
	if sessionID == "" {
		return AppState{}, fmt.Errorf("session id is empty")
	}
	if targetProject == "" {
		return AppState{}, fmt.Errorf("target project is empty")
	}

	state = NormalizeAppState(state)

	var (
		sourceProject string
		moved         PersistentSession
		found         bool
	)
	for _, project := range state.Projects {
		for _, session := range project.Sessions {
			if session.ID == sessionID {
				sourceProject, moved, found = project.Name, session, true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return AppState{}, fmt.Errorf("session %q not found", sessionID)
	}
	if sourceProject == targetProject {
		return AppState{}, fmt.Errorf("session %q already belongs to project %q", sessionID, targetProject)
	}

	targetExists := false
	for _, project := range state.Projects {
		if project.Name != targetProject {
			continue
		}
		targetExists = true
		for _, session := range project.Sessions {
			if session.Label == moved.Label {
				return AppState{}, fmt.Errorf("session name already exists in project %q", targetProject)
			}
		}
	}
	if !targetExists {
		return AppState{}, fmt.Errorf("project %q does not exist", targetProject)
	}

	next := AppState{Projects: make([]Project, 0, len(state.Projects))}
	for _, project := range state.Projects {
		switch project.Name {
		case sourceProject:
			remaining := make([]PersistentSession, 0, len(project.Sessions))
			for _, session := range project.Sessions {
				if session.ID != sessionID {
					remaining = append(remaining, session)
				}
			}
			if len(remaining) == 0 {
				continue
			}
			project.Sessions = remaining
		case targetProject:
			appended := make([]PersistentSession, 0, len(project.Sessions)+1)
			appended = append(appended, project.Sessions...)
			appended = append(appended, moved)
			project.Sessions = appended
		}
		next.Projects = append(next.Projects, project)
	}
	return NormalizeAppState(next), nil
}
