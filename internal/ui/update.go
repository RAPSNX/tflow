package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = max(16, min(28, m.width-8))
		return m, nil
	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.err = msg.err
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		changed := m.ensureSessionProjects()
		m.syncSelection()
		renames, err := m.scopedSessionRenames()
		if err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		if len(renames) > 0 {
			return m, m.renameSessionsCmd(renames)
		}
		if changed {
			if err := m.saveState(); err != nil {
				m.err = err
				m.status = err.Error()
			}
		}
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
		}
		return m, nil
	case sessionNamesMigratedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		for _, rename := range msg.renames {
			m.applySessionRename(rename)
		}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, nil
	case sessionCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		m.assignSessionProject(msg.session.Name, msg.project)
		m.setSessionType(msg.session.Name, msg.kind)
		m.setSessionLabel(msg.session.Name, msg.label)
		m.selectedProject = msg.project
		m.selectedSession = msg.session.Name
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case sessionKilledMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		delete(m.sessionProjects, msg.name)
		delete(m.sessionTypes, msg.name)
		delete(m.sessionLabels, msg.name)
		if m.selectedSession == msg.name {
			m.selectedSession = ""
		}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case projectCreatedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		msg.config = normalizeProjectConfig(msg.config)
		m.addProject(msg.config.Name)
		m.setProjectConfig(msg.config)
		m.selectedProject = msg.config.Name
		m.selectedSession = ""
		if msg.session.Name != "" {
			if !containsSessionName(m.sessions, msg.session.Name) {
				m.sessions = append(m.sessions, msg.session)
			}
			m.assignSessionProject(msg.session.Name, msg.config.Name)
			m.setSessionType(msg.session.Name, sessionTypeTerminal)
			m.setSessionLabel(msg.session.Name, defaultProjectSessionName)
			if current, ok := m.currentSessionInfo(); !ok || !current.Temporary {
				m.selectedSession = msg.session.Name
			}
		}
		m.mode = inputNone
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case sessionRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		project := normalizeProjectName(m.sessionProjects[msg.oldName])
		delete(m.sessionProjects, msg.oldName)
		m.sessionProjects[msg.newName] = project
		if sessionType, ok := m.sessionTypes[msg.oldName]; ok {
			delete(m.sessionTypes, msg.oldName)
			m.sessionTypes[msg.newName] = sessionType
		}
		label := msg.label
		if label == "" {
			label = m.sessionLabel(msg.oldName)
		}
		delete(m.sessionLabels, msg.oldName)
		m.setSessionLabel(msg.newName, label)
		if m.selectedSession == msg.oldName {
			m.selectedSession = msg.newName
		}
		if m.currentSession == msg.oldName {
			m.currentSession = msg.newName
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		if err := m.saveState(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, m.loadSessionsCmd()
	case projectRenamedMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = msg.err.Error()
			return m, nil
		}
		for _, rename := range msg.sessionRenames {
			m.applySessionRename(rename)
		}
		for name, project := range m.sessionProjects {
			if normalizeProjectName(project) == msg.oldName {
				m.sessionProjects[name] = msg.newName
			}
		}
		cfg := m.projectConfig(msg.oldName)
		delete(m.projectConfigs, msg.oldName)
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
		if err := m.syncTmuxSessionProjects(); err != nil {
			m.err = err
			m.status = err.Error()
			return m, nil
		}
		m.err = nil
		m.status = ""
		return m, nil
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
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlF || msg.Type == tea.KeyCtrlC {
			return m, m.closeMenuCmd()
		}
		if msg.Type == tea.KeyCtrlQ {
			return m.beginQuit()
		}
		if m.mode != inputNone {
			return m.updateModal(msg)
		}
		return m.updateNormal(msg)
	}

	return m, nil
}

func (m *model) applySessionRename(rename sessionRename) {
	project := normalizeProjectName(m.sessionProjects[rename.oldName])
	delete(m.sessionProjects, rename.oldName)
	m.sessionProjects[rename.newName] = project
	if sessionType, ok := m.sessionTypes[rename.oldName]; ok {
		delete(m.sessionTypes, rename.oldName)
		m.sessionTypes[rename.newName] = sessionType
	}
	label := m.sessionLabel(rename.oldName)
	delete(m.sessionLabels, rename.oldName)
	m.setSessionLabel(rename.newName, label)
	for index := range m.sessions {
		if m.sessions[index].Name == rename.oldName {
			m.sessions[index].Name = rename.newName
		}
	}
	if m.selectedSession == rename.oldName {
		m.selectedSession = rename.newName
	}
	if m.currentSession == rename.oldName {
		m.currentSession = rename.newName
	}
}

func (m model) updateNormal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, m.closeMenuCmd()
	case tea.KeyEsc:
		return m, m.closeMenuCmd()
	}

	switch msg.String() {
	case "j", "down":
		m.shiftSession(1)
		return m, nil
	case "k", "up":
		m.shiftSession(-1)
		return m, nil
	case "enter":
		return m.switchSelectedSession()
	case "n":
		m.mode = inputNew
		m.status = "New: p project, t terminal, k k9s, c agent."
		return m, nil
	case "N":
		return m.beginProjectCreate()
	case "p":
		return m.beginProjectSwitch()
	case "d":
		return m.beginDelete()
	case "D":
		return m.beginProjectDelete()
	case "r":
		return m.beginRename()
	case "R":
		return m.beginProjectRename()
	case "e":
		return m.editProject()
	}

	return m, nil
}

func (m model) updateModal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case inputNew:
		return m.updateNew(msg)
	case inputCreateSession:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Session creation cancelled."
			return m, nil
		case tea.KeyEnter:
			label := sanitizeSessionName(m.input.Value())
			if label == "" {
				m.status = "Session name is empty."
				return m, nil
			}
			project := m.contextProject()
			if m.hasSessionLabel(project, label, "") {
				m.status = "Session name already exists in this project."
				return m, nil
			}
			name := label
			if project != "" {
				name = projectSessionName(project, label)
			}
			if _, ok := m.findSession(name); ok {
				m.status = "Session name already exists."
				return m, nil
			}
			command, err := m.sessionStartupCommand(m.createSessionKind)
			if err != nil {
				m.err = err
				m.status = err.Error()
				return m, nil
			}
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			dir := m.createSessionDir()
			kind := sessionTypeFromKind(m.createSessionKind)
			return m, func() tea.Msg {
				s, err := m.tmux.CreateSession(name, dir, command)
				if err != nil {
					return sessionCreatedMsg{err: err}
				}
				return sessionCreatedMsg{session: s, kind: kind, project: project, label: label}
			}
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputCreateProject:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Project creation cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitProjectCreate()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputSwitchProject:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.input.Blur()
			m.input.Prompt = ""
			m.input.SetValue("")
			m.status = "Project switch cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitProjectSwitch()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputConfirmDelete:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.deleteTarget = deleteTarget{}
			m.status = "Delete cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmDelete()
		}
		switch msg.String() {
		case "d", "y":
			return m.confirmDelete()
		}
	case inputConfirmProjectSwitch:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.switchProjectTarget = ""
			m.status = "Project switch cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmProjectSwitch()
		}
		switch msg.String() {
		case "y":
			return m.confirmProjectSwitch()
		}
	case inputConfirmQuit:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.status = "Quit cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.confirmQuit()
		}
		switch msg.String() {
		case "y":
			return m.confirmQuit()
		}
	case inputEditProject:
		switch msg.Type {
		case tea.KeyEsc:
			return m.cancelProjectEdit()
		case tea.KeyEnter:
			return m.commitProjectEditField()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	case inputRename:
		switch msg.Type {
		case tea.KeyEsc:
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = "Rename cancelled."
			return m, nil
		case tea.KeyEnter:
			return m.commitRename()
		}
		next, cmd := m.input.Update(msg)
		m.input = next
		return m, cmd
	}
	return m, nil
}

func (m model) updateNew(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = inputNone
		m.status = "New cancelled."
		return m, nil
	}

	switch msg.String() {
	case "p":
		return m.beginProjectCreate()
	case "t":
		return m.startSessionCreate(sessionKindTerminal)
	case "k":
		return m.startSessionCreate(sessionKindK9s)
	case "c":
		return m.startSessionCreate(sessionKindAgent)
	default:
		m.status = "New: use p, t, k, or c."
		return m, nil
	}
}
