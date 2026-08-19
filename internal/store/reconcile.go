package store

import "github.com/rapsnx/tflow/internal/diag"

// ReconcileAppState observes the tmux session list while holding the state
// lock. Persistent metadata is intentionally retained when a session is not
// running so it can be materialized later through an explicit user selection.
//
// The session snapshot is taken while the state lock is held so startup
// observes tmux and persistent state within the same mutation boundary.
func ReconcileAppState(path string, snapshotSessions func() (map[string]struct{}, error)) (bool, error) {
	unlock, err := AcquireAppStateLock(path)
	if err != nil {
		return false, err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release app-state lock %q after reconciliation: %v", path, unlockErr)
		}
	}()

	if _, err := LoadAppState(path); err != nil {
		return false, err
	}
	if _, err := snapshotSessions(); err != nil {
		return false, err
	}
	return false, nil
}
