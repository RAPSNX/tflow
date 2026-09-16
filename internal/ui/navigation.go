package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/rapsnx/tflow/internal/diag"
	"github.com/rapsnx/tflow/internal/store"
)

// NavigatePrev switches the current client to the previous contextual session.
func NavigatePrev() error {
	return navigateWithManager(newSessionManager(), -1)
}

// NavigateNext switches the current client to the next contextual session.
func NavigateNext() error {
	return navigateWithManager(newSessionManager(), 1)
}

// resolveNavigationContext resolves the current client's session, whether
// it is a volatile (no-project) session, and its owning instance ID --
// shared by every navigation entry point that starts from the current
// session and needs to know which context (project or volatile instance) to
// search within.
func resolveNavigationContext(sessions []session) (currentSession string, isVolatile bool, instanceID string) {
	currentSession = strings.TrimSpace(os.Getenv(menuCurrentEnv))
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
		return "", false, ""
	}

	var currentInfo *session
	for i := range sessions {
		if sessions[i].Name == currentSession {
			currentInfo = &sessions[i]
			break
		}
	}

	if currentInfo != nil && (currentInfo.Temporary || strings.HasPrefix(currentInfo.Name, "tflow-v-")) {
		isVolatile = true
	} else if strings.HasPrefix(currentSession, "tflow-v-") {
		isVolatile = true
	}
	if isVolatile && currentInfo != nil {
		instanceID = strings.TrimSpace(currentInfo.Instance)
	}
	return currentSession, isVolatile, instanceID
}

func navigateWithManager(manager tmuxController, direction int) error {
	path := appStatePath()
	unlock, err := lockAppState(path)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release state lock %q after navigation: %v", path, unlockErr)
		}
	}()

	sessions, err := manager.ListSessions()
	if err != nil {
		return err
	}

	currentSession, isVolatile, instanceID := resolveNavigationContext(sessions)
	if currentSession == "" {
		return nil
	}
	if isVolatile && instanceID == "" {
		return nil
	}

	state := appState{}
	if !isVolatile {
		state, err = loadAppState(path)
		if err != nil {
			return err
		}
	}

	var contextSessions []session
	var project string

	if isVolatile {
		for _, s := range sessions {
			if s.Temporary && s.Instance == instanceID {
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

	targetIdx := currentIdx + direction
	if targetIdx < 0 || targetIdx >= len(contextSessions) {
		// Bounded navigation: halts at start or end without wrapping.
		refreshTargetTopBar(manager, currentSession, project, state, sessions, instanceID)
		return nil
	}
	target := contextSessions[targetIdx]
	return switchToContextTarget(manager, state, project, target, sessions, instanceID)
}

// switchToContextTarget lazily materializes target when it is a persistent
// session not currently running in tmux, then switches the client to it and
// refreshes the top bar -- the shared tail end of every navigation entry
// point once a target session has been chosen.
func switchToContextTarget(manager tmuxController, state appState, project string, target session, sessions []session, instanceID string) error {
	if !containsSessionName(sessions, target.Name) {
		var pWorkdir string
		for _, p := range state.Projects {
			if normalizeProjectName(p.Name) == normalizeProjectName(project) {
				pWorkdir = p.Workdir
				break
			}
		}
		workdir := store.NormalizeCWD(pWorkdir)
		sessionType, command := lookupSessionTypeCommand(state, project, target.Name)
		resolvedCommand := materializeCommand(sessionType, command)
		if err := validateMaterializeExecutable(sessionType, resolvedCommand, workdir); err != nil {
			return err
		}
		newS, err := manager.CreateSession(target.Name, workdir, shellQuoteLaunchCommand(resolvedCommand))
		if err != nil {
			return fmt.Errorf("materialize target session %q: %w", target.Name, err)
		}
		cleanup := func(operation string, setupErr error) error {
			if killErr := ignoreMissingSession(manager.KillSession(target.Name)); killErr != nil {
				diag.Warnf("kill lazily materialized session %q after %s failure: %v", target.Name, operation, killErr)
			}
			return fmt.Errorf("%s materialized target session %q: %w", operation, target.Name, setupErr)
		}
		if err := manager.SetSessionProject(target.Name, project); err != nil {
			return cleanup("project marker setup", err)
		}
		label := strings.TrimSpace(target.Label)
		if label != "" {
			if err := manager.SetSessionLabel(target.Name, label); err != nil {
				return cleanup("label marker setup", err)
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

// NavigateToGitSession switches the current client to the first git-typed
// session in its current project, or leaves a short status message when
// none exists. Session type is tracked only in the persisted project state
// (see lookupSessionTypeCommand), never mirrored as a tmux session option
// the way project/label/instance/attention are -- so a volatile (no-project)
// context, which has no persisted per-session state to consult, can never
// have a git session to jump to.
func NavigateToGitSession() error {
	return navigateToTypeWithManager(newSessionManager(), sessionTypeGit)
}

func navigateToTypeWithManager(manager tmuxController, targetType string) error {
	path := appStatePath()
	unlock, err := lockAppState(path)
	if err != nil {
		return err
	}
	defer func() {
		if unlockErr := unlock(); unlockErr != nil {
			diag.Warnf("release state lock %q after navigation: %v", path, unlockErr)
		}
	}()

	sessions, err := manager.ListSessions()
	if err != nil {
		return err
	}

	currentSession, isVolatile, instanceID := resolveNavigationContext(sessions)
	if currentSession == "" {
		return nil
	}
	if isVolatile {
		return manager.DisplayMessage(noSessionOfTypeMessage(targetType))
	}

	state, err := loadAppState(path)
	if err != nil {
		return err
	}

	var project string
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
	if project == "" {
		return manager.DisplayMessage(noSessionOfTypeMessage(targetType))
	}

	var target session
	found := false
	for _, p := range state.Projects {
		if normalizeProjectName(p.Name) != normalizeProjectName(project) {
			continue
		}
		for _, ps := range p.Sessions {
			if strings.TrimSpace(ps.Type) != targetType {
				continue
			}
			if sInfo, running := findSessionInList(sessions, ps.ID); running {
				target = sInfo
			} else {
				target = session{Name: ps.ID, Label: ps.Label}
			}
			found = true
			break
		}
		break
	}
	if !found {
		return manager.DisplayMessage(noSessionOfTypeMessage(targetType))
	}
	if target.Name == currentSession {
		refreshTargetTopBar(manager, currentSession, project, state, sessions, instanceID)
		return nil
	}

	return switchToContextTarget(manager, state, project, target, sessions, instanceID)
}

func noSessionOfTypeMessage(sessionType string) string {
	if sessionType == sessionTypeGit {
		return "No git session in this project."
	}
	return "No " + sessionType + " session in this project."
}

func findSessionInList(sessions []session, name string) (session, bool) {
	for _, s := range sessions {
		if s.Name == name {
			return s, true
		}
	}
	return session{}, false
}
