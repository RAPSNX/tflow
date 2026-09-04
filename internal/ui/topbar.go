package ui

import (
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
)

// refreshTargetTopBar computes and sets the derived top-bar status for the
// target session. Per the architecture, this computes derived status metadata
// for its selected target from post-mutation state and updates only that
// single target session without rewriting inactive or unrelated sessions.
func refreshTargetTopBar(manager tmuxController, targetSession, project string, state appState, sessions []session, instanceID string) {
	targetSession = strings.TrimSpace(targetSession)
	if targetSession == "" {
		return
	}
	content := computeTargetTopBar(targetSession, project, state, sessions, instanceID)
	if content == "" {
		return
	}
	if err := ignoreMissingSession(manager.SetSessionTopBar(targetSession, content)); err != nil {
		diag.Warnf("set top bar for session %q: %v", targetSession, err)
	}
}

// computeTargetTopBar computes the formatted top-bar status string for targetSession.
func computeTargetTopBar(targetSession, project string, state appState, sessions []session, instanceID string) string {
	project = normalizeProjectName(project)
	if project == "" {
		for _, p := range state.Projects {
			for _, s := range p.Sessions {
				if s.ID == targetSession {
					project = p.Name
					break
				}
			}
			if project != "" {
				break
			}
		}
	}

	var labels []string
	activeIdx := -1

	if project != "" {
		for _, p := range state.Projects {
			if normalizeProjectName(p.Name) == project {
				labels = make([]string, len(p.Sessions))
				for i, s := range p.Sessions {
					labels[i] = strings.TrimSpace(s.Label)
					if labels[i] == "" {
						labels[i] = s.ID
					}
					if s.ID == targetSession {
						activeIdx = i
					}
				}
				break
			}
		}
	} else {
		// Volatile context
		var volatile []session
		for _, s := range sessions {
			if s.Temporary && (instanceID == "" || s.Instance == instanceID) {
				volatile = append(volatile, s)
			}
		}
		labels = make([]string, len(volatile))
		for i, s := range volatile {
			labels[i] = strings.TrimSpace(s.Label)
			if labels[i] == "" {
				labels[i] = s.Name
			}
			if s.Name == targetSession {
				activeIdx = i
			}
		}
	}

	if len(labels) == 0 || activeIdx == -1 {
		return ""
	}

	palette := catppuccinTmuxPalette()
	return palette.FormatTopBar(labels, activeIdx)
}

func (m model) refreshActiveTopBarIfContext(affectedSession string) {
	if m.currentSession == "" {
		return
	}
	currentProject := normalizeProjectName(m.sessionProjects[m.currentSession])
	affectedProject := normalizeProjectName(m.sessionProjects[affectedSession])
	sameProject := currentProject != "" && currentProject == affectedProject
	sameVolatile := currentProject == "" && affectedProject == ""
	if sameProject || sameVolatile {
		state := m.currentState()
		refreshTargetTopBar(m.tmux, m.currentSession, currentProject, state, m.sessions, m.instanceID)
	}
}

func (m model) refreshActiveTopBar() {
	if m.currentSession == "" {
		return
	}
	project := normalizeProjectName(m.sessionProjects[m.currentSession])
	state := m.currentState()
	refreshTargetTopBar(m.tmux, m.currentSession, project, state, m.sessions, m.instanceID)
}
