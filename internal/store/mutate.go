package store

import "github.com/rapsnx/tflow/internal/diag"

// MutateAppState serializes a state mutation, reloads the latest state, and
// atomically persists the returned state.
func MutateAppState(path string, mutate func(AppState) (AppState, error)) (AppState, error) {
	unlock, err := AcquireAppStateLock(path)
	if err != nil {
		return AppState{}, err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release app-state lock %q after mutation: %v", path, unlockErr)
		}
	}()
	return MutateAppStateLocked(path, mutate)
}

// MutateAppStateLocked applies a mutation while the caller holds the state
// lock returned by AcquireAppStateLock.
func MutateAppStateLocked(path string, mutate func(AppState) (AppState, error)) (AppState, error) {
	latest, err := LoadAppState(path)
	if err != nil {
		return AppState{}, err
	}
	next, err := mutate(latest)
	if err != nil {
		return AppState{}, err
	}
	if err := SaveAppState(path, next); err != nil {
		return AppState{}, err
	}
	return next, nil
}
