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
// client visit unconditionally clears the visited (destination) session's
// attention marker and stamps its new activity watermark. It also stamps
// the outgoing session (tmux's #{client_last_session}, the session being
// switched away from) at this exact moment, rather than leaving it to
// AttentionScan's next tick -- output produced in the gap between the last
// tick and the switch would otherwise still look unseen once the session is
// detached.
func SessionVisited() error {
	return sessionVisitedWithManager(newSessionManager())
}

func sessionVisitedWithManager(manager tmuxController) error {
	name := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	if name == "" {
		return nil
	}
	if err := ignoreMissingSession(manager.MarkSessionVisited(name)); err != nil {
		return err
	}
	if last := strings.TrimSpace(os.Getenv(menuLastVisitedEnv)); last != "" && last != name {
		return ignoreMissingSession(manager.MarkSessionVisited(last))
	}
	return nil
}

// AttentionScan is invoked on every tmux status-line redraw (an invisible
// #() job embedded in status-right, ticking on the bounded, tmux-native
// status-interval timer -- see EnsureControlMode) and is the mechanism the
// attention feature actually depends on; SessionActivity is a best-effort
// supplement. It refreshes the current session's own activity watermark (so
// output produced while it is being viewed can't look unseen the instant the
// client leaves it), sets the attention marker for any unvisited session
// whose window has produced fresh output since its watermark, then
// refreshes this client's own visible top bar so a sibling session's
// attention reaches it without waiting for an unrelated switch, rename, or
// other mutation to trigger a refresh.
func AttentionScan() error {
	return attentionScanWithManager(newSessionManager())
}

func attentionScanWithManager(manager tmuxController) error {
	current := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	if current != "" {
		// client-session-changed only stamps the session being entered,
		// never the one being left, so output produced while this session
		// is being actively viewed would otherwise still look unseen the
		// instant the client switches away (its watermark would still be
		// its entry-time stamp, while window_activity kept advancing
		// throughout the whole visit). Refreshing it here every scan tick
		// keeps the watermark within one tick of "now" for as long as the
		// visit lasts.
		if err := ignoreMissingSession(manager.MarkSessionVisited(current)); err != nil {
			return err
		}
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		return ignoreMissingSession(err)
	}
	// list-sessions only samples each session's active window, so a
	// background window in a multi-window session would be missed; scan
	// every window and aggregate per session instead.
	activity, err := manager.SessionActivityTimestamps()
	if err != nil {
		return ignoreMissingSession(err)
	}
	for i := range sessions {
		s := &sessions[i]
		// A background (non-active) window's activity flag is only cleared
		// by individually selecting that window, not by visiting the
		// session -- so it can stay set long after the last visit. Comparing
		// against the visited-at watermark instead of trusting any nonzero
		// activity time tells genuinely fresh output from a stale flag left
		// over from before that visit. VisitedAt is stamped from this same
		// session's own peak window_activity at visit time (not wall-clock
		// time), so a strict ">" here is unambiguous even when a visit and
		// some activity land in the same one-second tmux clock tick: if
		// window_activity hasn't advanced past what it already was at visit
		// time, nothing new has happened.
		if activityAt, ok := activity[s.Name]; !ok || activityAt <= s.VisitedAt || s.Attached || s.Attention {
			continue
		}
		// s.Attached is a snapshot from the ListSessions call above; a client
		// can attach between that snapshot and this write (including via the
		// client-session-changed hook clearing the marker concurrently), and
		// nothing later re-clears attention for an attached session. Recheck
		// attachment immediately before writing to shrink that race to a
		// single round-trip instead of this whole scan's duration.
		attached, err := manager.SessionAttached(s.Name)
		if err != nil {
			if ignored := ignoreMissingSession(err); ignored != nil {
				return ignored
			}
			continue
		}
		if attached {
			continue
		}
		if err := ignoreMissingSession(manager.SetSessionAttention(s.Name, true)); err != nil {
			return err
		}
		s.Attention = true
	}

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
