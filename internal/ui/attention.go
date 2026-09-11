package ui

import (
	"os"
	"strings"
)

// SessionActivity is invoked by tmux's alert-activity hook. It sets the
// runtime-only attention marker for the alerting session, but only when that
// session is currently unvisited (no client attached) -- a session the
// client is actively looking at never needs the marker, and any later visit
// clears it via SessionVisited regardless.
func SessionActivity() error {
	return sessionActivityWithManager(newSessionManager())
}

func sessionActivityWithManager(manager tmuxController) error {
	name := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	if name == "" {
		return nil
	}
	attached, err := manager.SessionAttached(name)
	if err != nil {
		return ignoreMissingSession(err)
	}
	if attached {
		return nil
	}
	return ignoreMissingSession(manager.SetSessionAttention(name, true))
}

// SessionVisited is invoked by tmux's client-session-changed hook. Any
// client visit unconditionally clears the visited session's attention
// marker.
func SessionVisited() error {
	return sessionVisitedWithManager(newSessionManager())
}

func sessionVisitedWithManager(manager tmuxController) error {
	name := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	if name == "" {
		return nil
	}
	return ignoreMissingSession(manager.SetSessionAttention(name, false))
}

// AttentionScan is invoked on every tmux status-line redraw (an invisible
// #() job embedded in status-right, ticking on the bounded, tmux-native
// status-interval timer -- see EnsureControlMode) because alert-activity's
// run-shell hook does not fire on the tested tmux 3.7c build (verified via
// tmux -vv server tracing; recorded in .codex/TASK.md), leaving
// SessionActivity uninvoked in that environment. It sets the attention
// marker for any unvisited session whose window has produced fresh output,
// then refreshes this client's own visible top bar so a sibling session's
// attention reaches it without waiting for an unrelated switch, rename, or
// other mutation to trigger a refresh.
func AttentionScan() error {
	return attentionScanWithManager(newSessionManager())
}

func attentionScanWithManager(manager tmuxController) error {
	sessions, err := manager.ListSessions()
	if err != nil {
		return ignoreMissingSession(err)
	}
	// list-sessions only samples each session's active window, so a
	// background window in a multi-window session would be missed; scan
	// every window and aggregate per session instead.
	activity, err := manager.WindowActivityBySession()
	if err != nil {
		return ignoreMissingSession(err)
	}
	for i := range sessions {
		s := &sessions[i]
		if activity[s.Name] && !s.Attached && !s.Attention {
			if err := ignoreMissingSession(manager.SetSessionAttention(s.Name, true)); err != nil {
				return err
			}
			s.Attention = true
		}
	}

	current := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	if current == "" {
		return nil
	}
	state, err := loadAppState(appStatePath())
	if err != nil {
		return err
	}
	// The status #() job only exports TFLOW_CURRENT_SESSION, not an instance
	// ID, so the instance is resolved from the current session's own tmux
	// marker (already in sessions from ListSessions) rather than an env var
	// that was never set -- an empty instanceID would make the volatile
	// branch of computeTargetTopBar treat every instance's sessions as its
	// own, bleeding other instances' volatile sessions into this top bar.
	instanceID := ""
	for _, s := range sessions {
		if s.Name == current {
			instanceID = s.Instance
			break
		}
	}
	refreshTargetTopBar(manager, current, "", state, sessions, instanceID)
	return nil
}
