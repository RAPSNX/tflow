package store

import "strings"

// RemoveSession removes the persistent session identified by sessionID and
// drops its project if that removal leaves it without any sessions,
// matching the convention already used by reconciliation and MoveSession
// for projects left without sessions. It is a pure state transformation:
// callers persist the result through the existing advisory-lock mutation
// path (MutateAppState / MutateAppStateLocked).
//
// It is a no-op, returning state unchanged, when sessionID does not exist
// -- matching "ignore cleanup requests for resources that are already gone."
func RemoveSession(state AppState, sessionID string) AppState {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return state
	}
	state = NormalizeAppState(state)

	found := false
	next := AppState{Projects: make([]Project, 0, len(state.Projects))}
	for _, project := range state.Projects {
		remaining := make([]PersistentSession, 0, len(project.Sessions))
		for _, session := range project.Sessions {
			if session.ID == sessionID {
				found = true
				continue
			}
			remaining = append(remaining, session)
		}
		if len(remaining) == 0 {
			continue
		}
		project.Sessions = remaining
		next.Projects = append(next.Projects, project)
	}
	if !found {
		return state
	}
	return NormalizeAppState(next)
}
