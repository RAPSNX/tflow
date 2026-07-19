package store

import "reflect"

// ReconcileAppState removes persistent-session metadata that no longer has a
// matching tmux session and drops projects left without sessions. It writes
// state only when reconciliation changes it.
//
// The session snapshot is taken while the state lock is held. This prevents a
// concurrent mutation from adding metadata after the snapshot but before the
// reconciled state is saved.
func ReconcileAppState(path string, snapshotSessions func() (map[string]struct{}, error)) (bool, error) {
	unlock, err := AcquireAppStateLock(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = unlock() }()

	state, err := LoadAppState(path)
	if err != nil {
		return false, err
	}
	sessions, err := snapshotSessions()
	if err != nil {
		return false, err
	}
	reconciled := AppState{Projects: make([]Project, 0, len(state.Projects))}
	for _, project := range state.Projects {
		remaining := make([]PersistentSession, 0, len(project.Sessions))
		for _, session := range project.Sessions {
			if _, exists := sessions[session.ID]; exists {
				remaining = append(remaining, session)
			}
		}
		if len(remaining) == 0 {
			continue
		}
		project.Sessions = remaining
		reconciled.Projects = append(reconciled.Projects, project)
	}
	reconciled = NormalizeAppState(reconciled)
	if reflect.DeepEqual(state, reconciled) {
		return false, nil
	}
	if err := SaveAppState(path, reconciled); err != nil {
		return false, err
	}
	return true, nil
}
