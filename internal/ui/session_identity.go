package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) projectSessionRenames(oldProject, newProject string) ([]sessionRename, error) {
	renames := make([]sessionRename, 0)
	for _, s := range m.projectSessions(oldProject) {
		name := projectSessionName(newProject, m.sessionLabel(s.Name))
		if name == "" {
			return nil, fmt.Errorf("project session %q has an empty label", s.Name)
		}
		if existing, ok := m.findSession(name); ok && existing.Name != s.Name {
			return nil, fmt.Errorf("session name %q already exists", name)
		}
		renames = append(renames, sessionRename{oldName: s.Name, newName: name})
	}
	return renames, nil
}

func (m model) scopedSessionRenames() ([]sessionRename, error) {
	renames := make([]sessionRename, 0)
	targets := map[string]string{}
	for _, s := range m.sessions {
		if s.Temporary {
			continue
		}
		project := normalizeProjectName(m.sessionProjects[s.Name])
		if project == "" {
			continue
		}
		name := projectSessionName(project, m.sessionLabel(s.Name))
		if name == "" {
			return nil, fmt.Errorf("project session %q has an empty label", s.Name)
		}
		if name == s.Name {
			continue
		}
		if existing, ok := m.findSession(name); ok && existing.Name != s.Name {
			return nil, fmt.Errorf("session name %q already exists", name)
		}
		if previous, ok := targets[name]; ok && previous != s.Name {
			return nil, fmt.Errorf("project sessions %q and %q share label %q", previous, s.Name, m.sessionLabel(s.Name))
		}
		targets[name] = s.Name
		renames = append(renames, sessionRename{oldName: s.Name, newName: name})
	}
	return renames, nil
}

func (m model) renameSessionsCmd(renames []sessionRename) tea.Cmd {
	return func() tea.Msg {
		applied := make([]sessionRename, 0, len(renames))
		for _, rename := range renames {
			if err := m.tmux.RenameSession(rename.oldName, rename.newName); err != nil {
				for i := len(applied) - 1; i >= 0; i-- {
					_ = m.tmux.RenameSession(applied[i].newName, applied[i].oldName)
				}
				return sessionNamesMigratedMsg{err: fmt.Errorf("migrate project session names: %w", err)}
			}
			applied = append(applied, rename)
		}
		return sessionNamesMigratedMsg{renames: renames}
	}
}
