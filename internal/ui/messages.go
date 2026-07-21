package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	runtmux "tflow/internal/tmux"
)

func (m model) finishSessionCreationFollowUpError(err error) (tea.Model, tea.Cmd) {
	m.mode = inputNone
	m.err = err
	m.status = err.Error()
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
		m.syncSelection()
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			if m.mode == inputCreatingSession {
				m.mode = inputCreateSession
				m.input.Focus()
			}
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		if !containsSessionName(m.sessions, msg.session.Name) {
			msg.session.Temporary = msg.volatile
			if msg.volatile {
				msg.session.Instance = m.instanceID
			}
			m.sessions = append(m.sessions, msg.session)
		}
		if msg.volatile {
			if m.clearSessionMetadata(msg.session.Name) {
				if err := m.saveState(); err != nil {
					m.err = err
					m.status = err.Error()
					return m.finishSessionCreationFollowUpError(err)
				}
			}
			m.selectedProject = ""
			m.selectedSession = msg.session.Name
		} else {
			m.assignSessionProject(msg.session.Name, msg.project)
			m.setSessionLabel(msg.session.Name, msg.label)
			m.selectedProject = msg.project
			m.selectedSession = msg.session.Name
			if err := m.saveState(); err != nil {
				return m.finishSessionCreationFollowUpError(err)
			}
		}
		if err := m.syncSessionMarkers(msg.session.Name); err != nil {
			m.err = err
			m.status = err.Error()
			return m.finishSessionCreationFollowUpError(err)
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
		deleted, found := m.findSession(msg.name)
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
		nextProject := m.nextProjectAfter(project, deletingProject)
		if nextProject != "" {
			return m.switchToProject(nextProject)
		}
		return m.createVolatileFallback()
	case projectCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		msg.config = normalizeProjectConfig(msg.config)
		m.addProject(msg.config.Name)
		m.setProjectConfig(msg.config)
		keepContext := m.selectedProject != "" || m.selectedSession != "" || m.volatileContext()
		if !keepContext {
			m.selectedProject = msg.config.Name
			m.selectedSession = ""
		}
		if msg.session.Name != "" {
			if !containsSessionName(m.sessions, msg.session.Name) {
				m.sessions = append(m.sessions, msg.session)
			}
			m.assignSessionProject(msg.session.Name, msg.config.Name)
			m.setSessionLabel(msg.session.Name, msg.label)
			if !keepContext {
				m.selectedSession = msg.session.Name
			}
		}
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		if err := m.syncSessionMarkers(msg.session.Name); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.closeMenuCmd()
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		session, found := m.findSession(msg.name)
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
					m.err = err
					m.status = err.Error()
					return m.finishSessionCreationFollowUpError(err)
				}
			}
		} else {
			m.setSessionLabel(msg.name, msg.label)
			if err := m.saveState(); err != nil {
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
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
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
	case menuActionMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		if msg.quit {
			m.exitAction = menuExitQuit
			m.exitSessionName = ""
		} else if strings.TrimSpace(msg.switchSession) != "" {
			m.exitAction = menuExitSwitchSession
			m.exitSessionName = msg.switchSession
		} else {
			m.exitAction = menuExitNone
			m.exitSessionName = ""
		}
		m.err = nil
		m.status = ""
		return m, tea.Quit
	}

	return m, nil
}
