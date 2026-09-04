package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/rapsnx/tflow/internal/store"
)

// NavigatePrev switches the current client to the previous contextual session with wraparound.
func NavigatePrev() error {
	return navigateWithManager(newSessionManager(), -1)
}

// NavigateNext switches the current client to the next contextual session with wraparound.
func NavigateNext() error {
	return navigateWithManager(newSessionManager(), 1)
}

func navigateWithManager(manager tmuxController, direction int) error {
	currentSession := strings.TrimSpace(os.Getenv(menuCurrentEnv))
	sessions, err := manager.ListSessions()
	if err != nil {
		return err
	}

	if currentSession == "" {
		for _, s := range sessions {
			if s.Attached {
				currentSession = s.Name
				break
			}
		}
	}
	if currentSession == "" && len(sessions) > 0 {
		currentSession = sessions[0].Name
	}
	if currentSession == "" {
		return nil
	}

	path := appStatePath()
	state, err := loadAppState(path)
	if err != nil {
		return err
	}

	// Resolve instance ID
	var currentInfo *session
	for i := range sessions {
		if sessions[i].Name == currentSession {
			currentInfo = &sessions[i]
			break
		}
	}

	instanceID := strings.TrimSpace(os.Getenv(menuInstanceEnv))
	if instanceID == "" && currentInfo != nil && currentInfo.Instance != "" {
		instanceID = currentInfo.Instance
	}
	if instanceID == "" {
		for _, s := range sessions {
			if s.Temporary && s.Instance != "" {
				instanceID = s.Instance
				break
			}
		}
	}

	// Determine if current session is volatile or belongs to a project
	isVolatile := false
	if currentInfo != nil && (currentInfo.Temporary || strings.HasPrefix(currentInfo.Name, "tflow-v-")) {
		isVolatile = true
	} else if strings.HasPrefix(currentSession, "tflow-v-") {
		isVolatile = true
	}

	var contextSessions []session
	var project string

	if isVolatile {
		for _, s := range sessions {
			if s.Temporary && (instanceID == "" || s.Instance == instanceID) {
				contextSessions = append(contextSessions, s)
			}
		}
	} else {
		// Find project in state
		for _, p := range state.Projects {
			for _, s := range p.Sessions {
				if s.ID == currentSession {
					project = p.Name
					break
				}
			}
			if project != "" {
				break
			}
		}
		if project != "" {
			for _, p := range state.Projects {
				if normalizeProjectName(p.Name) == normalizeProjectName(project) {
					for _, ps := range p.Sessions {
						sInfo, running := findSessionInList(sessions, ps.ID)
						if running {
							contextSessions = append(contextSessions, sInfo)
						} else {
							contextSessions = append(contextSessions, session{
								Name:  ps.ID,
								Label: ps.Label,
							})
						}
					}
					break
				}
			}
		}
	}

	if len(contextSessions) <= 1 {
		// Single session in context: navigation is a no-op, but refresh top bar if needed.
		refreshTargetTopBar(manager, currentSession, project, state, sessions, instanceID)
		return nil
	}

	currentIdx := -1
	for i, s := range contextSessions {
		if s.Name == currentSession {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		return nil
	}

	targetIdx := (currentIdx + direction + len(contextSessions)) % len(contextSessions)
	target := contextSessions[targetIdx]

	// Lazily materialize if target is a persistent session that is not running in tmux
	if !containsSessionName(sessions, target.Name) {
		var pWorkdir string
		for _, p := range state.Projects {
			if normalizeProjectName(p.Name) == normalizeProjectName(project) {
				pWorkdir = p.Workdir
				break
			}
		}
		workdir := store.NormalizeCWD(pWorkdir)
		newS, err := manager.CreateSession(target.Name, workdir, "")
		if err != nil {
			return fmt.Errorf("materialize target session %q: %w", target.Name, err)
		}
		if err := manager.SetSessionProject(target.Name, project); err != nil {
			return err
		}
		label := strings.TrimSpace(target.Label)
		if label != "" {
			if err := manager.SetSessionLabel(target.Name, label); err != nil {
				return err
			}
		}
		sessions = append(sessions, newS)
	}

	if err := manager.SwitchClient(target.Name); err != nil {
		return err
	}

	refreshTargetTopBar(manager, target.Name, project, state, sessions, instanceID)
	return nil
}

func findSessionInList(sessions []session, name string) (session, bool) {
	for _, s := range sessions {
		if s.Name == name {
			return s, true
		}
	}
	return session{}, false
}
