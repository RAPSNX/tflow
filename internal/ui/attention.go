package ui

import (
	"os"
	"strings"
)

// SessionVisited records content baselines for the entered and outgoing
// sessions so text already viewed cannot trigger attention after a switch.
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

// AttentionScan samples pane text on the bounded tmux status timer.
func AttentionScan() error {
	return attentionScanWithManager(newSessionManager())
}

func attentionScanWithManager(manager tmuxController) error {
	current := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	sessions, err := manager.ListSessions()
	if err != nil {
		return ignoreMissingSession(err)
	}
	for i := range sessions {
		attention, err := manager.RefreshSessionAttention(sessions[i])
		if err != nil {
			if ignored := ignoreMissingSession(err); ignored != nil {
				return ignored
			}
			continue
		}
		sessions[i].Attention = attention
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
