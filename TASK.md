# tflow TASK

Implement the target model from `DESIGN.md`.

Use `DESIGN.md` as the source of truth for architecture, persistence, UI behavior, lifecycle rules, and keybindings.

## Implementation Tasks

### Persistence

- [ ] Treat `config.yaml` as the only source of truth for persistent projects.
- [ ] Store persistent projects directly under `projects`.
- [ ] Use `agent-cmd` as the public config field.
- [ ] Remove the target dependency on per-project YAML files.
- [ ] Keep `state.json` for runtime and restore state only.
- [ ] Store persistent session restore state in `state.json`.
- [ ] Store the last known cwd for persistent project sessions.
- [ ] Restore persistent project sessions after logout, reboot, or tmux server loss.
- [ ] Never restore volatile projects.

### Project Lifecycle

- [ ] Create a volatile random animal project on every new `tflow` start.
- [ ] Create a default `code` session inside the volatile project.
- [ ] Track whether a project is volatile or persistent.
- [ ] Hide volatile projects from the project list.
- [ ] Prevent switching into volatile projects.
- [ ] Clean up volatile projects and sessions when the owning terminal exits.
- [ ] Ensure persistent projects and sessions survive terminal exit unchanged.
- [ ] Implement `P` to persist the current volatile project.
- [ ] Rename the volatile project to the chosen persistent project name during persist.
- [ ] Keep existing sessions when persisting.

### Session Lifecycle

- [ ] Ensure sessions always belong to exactly one project.
- [ ] Ensure logical session names are unique only inside one project.
- [ ] Allow the same logical session name in different projects.
- [ ] Keep tmux session names internal and globally unique.
- [ ] Implement `t` for new terminal session.
- [ ] Implement `a` for new agent session.
- [ ] Implement `r` for renaming the selected session.
- [ ] Implement `m` for moving a session to a persistent project.
- [ ] Track cwd with tmux `#{pane_current_path}`.
- [ ] Restore persistent sessions in their last known cwd.

### Sidebar Layout

- [ ] Replace the project/session tree with separate `Sessions` and `Projects` sections.
- [ ] Show only current-project sessions in `Sessions`.
- [ ] Show only persistent projects in `Projects`.
- [ ] Mark current session and current persistent project with `[live]`.
- [ ] Fix selected-row highlighting when `[live]` or `[agent]` badges are present.
- [ ] Keep the sessions section at minimum visible height of five rows unless fewer sessions exist.
- [ ] Keep styling consistent with the existing TUI borders.
- [ ] Keep help hidden until `?` is pressed.

### Keybindings

- [ ] Remove `n`-prefixed command mode.
- [ ] Remove `R` project rename behavior.
- [ ] Remove explicit project termination keybindings such as `Ctrl+Q`.
- [ ] Implement direct `t` key.
- [ ] Implement direct `a` key.
- [ ] Implement direct `r` key.
- [ ] Implement direct `p` key.
- [ ] Implement direct `m` key.
- [ ] Implement direct `P` key.
- [ ] Implement `?` help toggle.

### Interaction Flow

- [ ] Remove extra menus from the target flow.
- [ ] Replace normal interactions with inline sidebar states.
- [ ] Keep only the minimal centered `P` persist-project overlay.
- [ ] Implement hints mode inside the sidebar for `p` and `m`.
- [ ] Ensure `Esc` cancels inline input, hints mode, or overlay.
- [ ] Ensure `Ctrl+C` closes the sidebar.

### Tests and Cleanup

- [ ] Add/update tests for config parsing.
- [ ] Add/update tests for `agent-cmd`.
- [ ] Add/update tests for volatile vs persistent project behavior.
- [ ] Add/update tests for terminal-exit cleanup rules.
- [ ] Add/update tests for persistent session restore.
- [ ] Add/update tests for cwd tracking state.
- [ ] Add/update tests for project/session uniqueness.
- [ ] Add/update tests for sidebar rendering with badges.
- [ ] Add/update tests for hints mode.
- [ ] Add/update tests for keybindings.
- [ ] Update `README.md` after implementation.
- [ ] Run formatting.
- [ ] Run the existing test suite.
