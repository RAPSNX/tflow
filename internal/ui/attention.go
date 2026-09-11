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
	for i := range sessions {
		s := &sessions[i]
		if s.Activity && !s.Attached && !s.Attention {
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
	instanceID := strings.TrimSpace(os.Getenv(menuInstanceEnv))
	refreshTargetTopBar(manager, current, "", state, sessions, instanceID)
	return nil
}
