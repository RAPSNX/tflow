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
	sessions, err := manager.ListSessions()
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if s.Name != name {
			continue
		}
		if s.Attached {
			return nil
		}
		break
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
