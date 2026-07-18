package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) loadSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sessions, err := m.tmux.ListSessions()
		return sessionsLoadedMsg{sessions: sessions, err: err}
	}
}

func (m model) switchSelectedSession() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return menuActionMsg{switchSession: s.Name}
	}
}

func (m *model) beginProjectSwitch() (tea.Model, tea.Cmd) {
	if len(m.projects) == 0 {
		m.status = "No projects available."
		return m, nil
	}
	m.mode = inputSwitchProject
	m.switchProjectTarget = ""
	m.input.Prompt = "project: "
	m.input.SetValue("")
	m.input.Focus()
	m.status = "Type a project prefix to switch."
	return m, nil
}

func (m *model) beginProjectCreate() (tea.Model, tea.Cmd) {
	m.mode = inputCreateProject
	m.input.Prompt = "project: "
	m.input.SetValue("")
	m.input.Focus()
	m.status = "Create a new project."
	return m, nil
}

func (m *model) commitProjectCreate() (tea.Model, tea.Cmd) {
	name := sanitizeProjectName(m.input.Value())
	if name == "" {
		m.status = "Project name is empty."
		return m, nil
	}
	if containsString(m.projects, name) {
		m.status = "Project already exists."
		return m, nil
	}

	m.mode = inputNone
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	cfg := projectConfig{Name: name, Workdir: m.cwd}
	cwd := cfg.Workdir
	tmuxName := projectSessionName(name, defaultProjectSessionName)
	return m, func() tea.Msg {
		s, err := m.tmux.CreateSession(tmuxName, cwd, "")
		if err != nil {
			return projectCreatedMsg{
				config: cfg,
				err:    fmt.Errorf("create default project session %q: %w", defaultProjectSessionName, err),
			}
		}
		return projectCreatedMsg{config: cfg, session: s}
	}
}

func (m *model) commitProjectSwitch() (tea.Model, tea.Cmd) {
	project, ok := m.uniqueMatchingProject(m.input.Value())
	if !ok {
		m.status = "No unique project match."
		return m, nil
	}
	if current, ok := m.currentSessionInfo(); ok && current.Temporary {
		m.mode = inputConfirmProjectSwitch
		m.switchProjectTarget = project
		m.input.Blur()
		m.input.Prompt = ""
		m.input.SetValue("")
		m.status = fmt.Sprintf("Confirm switch from volatile session to project %s.", project)
		return m, nil
	}
	return m.switchToProject(project)
}

func (m *model) confirmProjectSwitch() (tea.Model, tea.Cmd) {
	project := m.switchProjectTarget
	m.switchProjectTarget = ""
	return m.switchToProject(project)
}

func (m *model) switchToProject(project string) (tea.Model, tea.Cmd) {
	project = normalizeProjectName(project)
	m.mode = inputNone
	m.switchProjectTarget = ""
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	if project == "" {
		m.status = "No project selected."
		return m, nil
	}
	sessions := m.projectSessions(project)
	if len(sessions) == 0 {
		m.status = fmt.Sprintf("Project %s has no sessions.", project)
		return m, nil
	}
	m.selectedProject = project
	m.selectedSession = sessions[0].Name
	m.status = ""
	return m.switchSelectedSession()
}

func (m model) killSession(name string) (tea.Model, tea.Cmd) {
	name = strings.TrimSpace(name)
	if name == "" {
		m.status = "No session selected."
		return m, nil
	}
	return m, func() tea.Msg {
		return sessionKilledMsg{name: name, err: m.tmux.KillSession(name)}
	}
}

func (m *model) beginDelete() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	m.mode = inputConfirmDelete
	m.deleteTarget = deleteTarget{session: s.Name}
	m.status = fmt.Sprintf("Confirm delete for session %s.", m.sessionLabel(s.Name))
	return m, nil
}

func (m model) confirmDelete() (tea.Model, tea.Cmd) {
	target := m.deleteTarget
	m.mode = inputNone
	m.deleteTarget = deleteTarget{}
	if target.session != "" {
		return m.killSession(target.session)
	}
	return m.deleteProject(target.project)
}

func (m model) closeMenuCmd() tea.Cmd {
	return func() tea.Msg {
		return menuActionMsg{}
	}
}

func (m *model) beginQuit() (tea.Model, tea.Cmd) {
	m.mode = inputConfirmQuit
	m.input.Blur()
	m.input.Prompt = ""
	m.input.SetValue("")
	m.status = "Confirm shutdown of this tflow instance."
	return m, nil
}

func (m model) confirmQuit() (tea.Model, tea.Cmd) {
	m.mode = inputNone
	return m, func() tea.Msg {
		return menuActionMsg{quit: true}
	}
}

func (m *model) beginRename() (tea.Model, tea.Cmd) {
	s, ok := m.selectedSessionInfo()
	if !ok {
		m.status = "No session selected."
		return m, nil
	}
	m.mode = inputRename
	m.renameTarget = renameTarget{session: s.Name}
	m.input.SetValue(m.sessionLabel(s.Name))
	m.input.CursorEnd()
	m.input.Prompt = "session: "
	m.input.Focus()
	m.status = "Rename the selected session."
	return m, nil
}

func (m *model) commitRename() (tea.Model, tea.Cmd) {
	target := m.renameTarget
	switch {
	case target.project != "":
		name := sanitizeProjectName(m.input.Value())
		if name == "" {
			m.status = "Project name is empty."
			return m, nil
		}
		if name == target.project {
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		if containsString(m.projects, name) {
			m.status = "Project already exists."
			return m, nil
		}
		renames, err := m.projectSessionRenames(target.project, name)
		if err != nil {
			m.status = err.Error()
			return m, nil
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			applied := make([]sessionRename, 0, len(renames))
			for _, rename := range renames {
				if err := m.tmux.RenameSession(rename.oldName, rename.newName); err != nil {
					for i := len(applied) - 1; i >= 0; i-- {
						_ = m.tmux.RenameSession(applied[i].newName, applied[i].oldName)
					}
					return projectRenamedMsg{oldName: target.project, newName: name, err: fmt.Errorf("rename project session: %w", err)}
				}
				applied = append(applied, rename)
			}
			return projectRenamedMsg{oldName: target.project, newName: name, sessionRenames: renames}
		}
	case target.session != "":
		label := sanitizeSessionName(m.input.Value())
		if label == "" {
			m.status = "Session name is empty."
			return m, nil
		}
		if label == m.sessionLabel(target.session) {
			m.mode = inputNone
			m.renameTarget = renameTarget{}
			m.input.Blur()
			m.input.Prompt = ""
			m.status = ""
			return m, nil
		}
		project := ""
		if selected, ok := m.findSession(target.session); !ok || !selected.Temporary {
			project = normalizeProjectName(m.sessionProjects[target.session])
		}
		if m.hasSessionLabel(project, label, target.session) {
			m.status = "Session name already exists in this project."
			return m, nil
		}
		name := label
		if project != "" {
			name = projectSessionName(project, label)
		}
		if existing, ok := m.findSession(name); ok && existing.Name != target.session {
			m.status = "Session name already exists."
			return m, nil
		}
		m.mode = inputNone
		m.renameTarget = renameTarget{}
		m.input.Blur()
		m.input.Prompt = ""
		return m, func() tea.Msg {
			return sessionRenamedMsg{oldName: target.session, newName: name, label: label, err: m.tmux.RenameSession(target.session, name)}
		}
	default:
		m.status = "Select a session or project to rename."
		return m, nil
	}
}
