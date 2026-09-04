package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rapsnx/tflow/internal/diag"
	runtmux "github.com/rapsnx/tflow/internal/tmux"
)

func (m model) finishSessionCreationFollowUpError(sessionName string, err error) (tea.Model, tea.Cmd) {
	m.mode = inputNone
	m.deferredDelete = nil
	m.deferredDeleteProject = ""
	m.fallbackSession = ""
	m.err = err
	m.status = err.Error()
	if strings.TrimSpace(sessionName) != "" {
		if killErr := ignoreMissingSession(m.tmux.KillSession(sessionName)); killErr != nil {
			diag.Warnf("kill unconfigured volatile session %q: %v", sessionName, killErr)
		}
	}
	return m, nil
}

func (m model) updateMessage(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.err = msg.err
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		if m.instanceID == "" {
			if current, ok := m.currentSessionInfo(); ok && current.Temporary {
				m.instanceID = current.Instance
			}
		}
		if _, ok := m.currentSessionInfo(); !ok {
			for _, session := range m.sessions {
				if session.Temporary && session.Attached && session.Instance == m.instanceID {
					m.currentSession = session.Name
					break
				}
			}
		}
		m.syncSelection()
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			m.deferredDelete = nil
			m.deferredDeleteProject = ""
			m.fallbackSession = ""
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		if msg.fallback {
			m.fallbackSession = msg.session.Name
		}
		if !containsSessionName(m.sessions, msg.session.Name) {
			msg.session.Temporary = msg.volatile
			if msg.volatile {
				msg.session.Instance = m.instanceID
			}
			m.sessions = append(m.sessions, msg.session)
		}
		if m.clearSessionMetadata(msg.session.Name) {
			if err := m.saveState(); err != nil {
				return m.finishSessionCreationFollowUpError(msg.session.Name, err)
			}
		}
		m.selectedProject = ""
		m.selectedSession = msg.session.Name
		if err := m.syncSessionMarkers(msg.session.Name); err != nil {
			return m.finishSessionCreationFollowUpError(msg.session.Name, err)
		}
		m.err = nil
		m.status = ""
		return m.switchSelectedSession()
	case sessionKilledMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		project := msg.project
		if project == "" {
			project = normalizeProjectName(m.sessionProjects[msg.name])
		}
		deleted, found := m.sessionInfo(msg.name)
		deletingProject := found && !deleted.Temporary && project != "" && m.isLastProjectSession(project, msg.name)
		// A blank m.currentSession means the client's active session is
		// unknown to this model instance; treat that as if the deleted
		// session were active so a valid attachment still gets
		// established, matching prior behavior for that (test-only)
		// case. Otherwise, when deletingProject is true, msg.name is
		// guaranteed to be the sole session left in that project, so
		// comparing against msg.name also covers "the active session
		// belonged to the deleted project".
		activeSessionDeleted := m.currentSession == "" || m.currentSession == msg.name
		m.sessions = filterSessions(m.sessions, func(s session) bool { return s.Name != msg.name })
		delete(m.sessionProjects, msg.name)
		delete(m.sessionLabels, msg.name)
		if m.selectedSession == msg.name {
			m.selectedSession = ""
		}
		if deletingProject {
			m.projects = removeProject(m.projects, project)
			delete(m.projectConfigs, project)
			if m.selectedProject == project {
				m.selectedProject = ""
			}
		}
		if found && !deleted.Temporary {
			if err := m.saveState(); err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
		}
		// The killed session no longer exists in tmux and no other session's
		// markers are affected by removing it, so there is nothing to write.
		m.err = nil
		m.status = ""
		if found && deleted.Temporary {
			m.syncSelection()
			if _, currentExists := m.currentSessionInfo(); currentExists {
				return m, m.closeMenuCmd()
			}
			return m.createVolatileFallback()
		}
		if !activeSessionDeleted {
			return m, nil
		}
		remaining := m.projectSessions(project)
		if len(remaining) > 0 {
			m.selectedProject = project
			m.selectedSession = remaining[0].Name
			return m.switchSelectedSession()
		}
		return m.createVolatileFallback()
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		session, found := m.sessionInfo(msg.name)
		if !found {
			m.err = fmt.Errorf("renamed session %q is missing", msg.name)
			m.status = m.err.Error()
			return m, nil
		}
		if msg.volatile || session.Temporary {
			for index := range m.sessions {
				if m.sessions[index].Name == msg.name {
					m.sessions[index].Label = msg.label
				}
			}
			if m.clearSessionMetadata(msg.name) {
				if err := m.saveState(); err != nil {
					return m.finishSessionCreationFollowUpError("", err)
				}
			}
		} else {
			m.setSessionLabel(msg.name, msg.label)
			if err := m.saveState(); err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
			if err := ignoreMissingSession(m.tmux.SetSessionLabel(msg.name, msg.label)); err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		// The rename command already wrote this session's label marker
		// synchronously (see commitRename), and a label rename never
		// changes project membership, so no further tmux writes are
		// needed here.
		m.err = nil
		m.status = ""
		return m, m.closeMenuCmd()
	case projectRenamedMsg:
		for name, project := range m.sessionProjects {
			if normalizeProjectName(project) == msg.oldName {
				m.sessionProjects[name] = msg.newName
			}
		}
		cfg := m.projectConfig(msg.oldName)
		delete(m.projectConfigs, msg.oldName)
		if sessions, ok := m.persistentSessionOrder[msg.oldName]; ok {
			delete(m.persistentSessionOrder, msg.oldName)
			m.persistentSessionOrder[msg.newName] = sessions
		}
		cfg.Name = msg.newName
		m.setProjectConfig(cfg)
		m.projects = replaceProject(m.projects, msg.oldName, msg.newName)
		if m.selectedProject == msg.oldName {
			m.selectedProject = msg.newName
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.syncSelection()
		// Only the renamed project's own sessions carry its name in their
		// tmux project marker; every other project's sessions are
		// unaffected and must not be rewritten. A session may have been
		// killed in tmux outside tflow after the sidebar loaded; skip it
		// and keep updating the rest instead of treating a vanished
		// session as a hard failure.
		for _, s := range m.projectSessions(msg.newName) {
			if err := m.tmux.SetSessionProject(s.Name, msg.newName); err != nil {
				if runtmux.IsNoSession(err) || runtmux.IsNoServer(err) {
					continue
				}
				m.err = err
				m.status = err.Error()
				return m, nil
			}
		}
		m.err = nil
		m.status = ""
		return m, m.closeMenuCmd()
	case projectDeletedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		return m.applyProjectDeletion(msg.project)
	case projectEditorFinishedMsg:
		return m.handleProjectEditorFinished(msg)
	case menuActionMsg:
		if msg.quit {
			m.exitAction = menuExitQuit
			m.exitSessionName = ""
			m.exitDeleteSessions = nil
			m.exitDeleteProject = ""
			m.exitFallbackSession = ""
		} else if strings.TrimSpace(msg.switchSession) != "" {
			m.exitAction = menuExitSwitchSession
			m.exitSessionName = msg.switchSession
			m.exitDeleteSessions = append([]string(nil), msg.deleteSessions...)
			m.exitDeleteProject = msg.deleteProject
			m.exitFallbackSession = msg.exitFallbackSession
		} else {
			m.exitAction = menuExitNone
			m.exitSessionName = ""
			m.exitDeleteSessions = nil
			m.exitDeleteProject = ""
			m.exitFallbackSession = ""
		}
		m.err = nil
		m.status = ""
		return m, tea.Quit
	}

	return m, nil
}
