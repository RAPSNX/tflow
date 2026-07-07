# tflow TASK

Implement the target model from `DESIGN.md`.

Use `DESIGN.md` as the source of truth for architecture, persistence, UI behavior, lifecycle rules, and keybindings.

## Implementation Tasks

### Persistence

- [x] Treat `config.yaml` as the only source of truth for persistent projects.
- [x] Store persistent projects directly under `projects`.
- [x] Use `agent-cmd` as the public config field.
- [x] Remove the target dependency on per-project YAML files.
- [x] Keep `state.json` for runtime and restore state only.
- [x] Store persistent session restore state in `state.json`.
- [x] Store the last known cwd for persistent project sessions.
- [x] Restore persistent project sessions after logout, reboot, or tmux server loss.
- [x] Never restore volatile projects.

### Project Lifecycle

- [x] Create a volatile random animal project on every new `tflow` start.
- [x] Create a default `code` session inside the volatile project.
- [x] Track whether a project is volatile or persistent.
- [x] Hide volatile projects from the project list.
- [x] Prevent switching into volatile projects.
- [x] Clean up volatile projects and sessions when the owning terminal exits.
- [x] Ensure persistent projects and sessions survive terminal exit unchanged.
- [x] Implement `P` to persist the current volatile project.
- [x] Rename the volatile project to the chosen persistent project name during persist.
- [x] Keep existing sessions when persisting.

### Session Lifecycle

- [x] Ensure sessions always belong to exactly one project.
- [x] Ensure logical session names are unique only inside one project.
- [x] Allow the same logical session name in different projects.
- [x] Keep tmux session names internal and globally unique.
- [x] Implement `t` for new terminal session.
- [x] Implement `a` for new agent session.
- [x] Implement `r` for renaming the selected session.
- [x] Implement `m` for moving a session to a persistent project.
- [x] Track cwd with tmux `#{pane_current_path}`.
- [x] Restore persistent sessions in their last known cwd.

### Sidebar Layout

- [x] Replace the project/session tree with separate `Sessions` and `Projects` sections.
- [x] Show only current-project sessions in `Sessions`.
- [x] Show only persistent projects in `Projects`.
- [x] Mark current session and current persistent project with `[live]`.
- [ ] Fix selected-row highlighting when `[live]` or `[agent]` badges are present.
- [x] Keep the sessions section at minimum visible height of five rows unless fewer sessions exist.
- [x] Keep styling consistent with the existing TUI borders.
- [x] Keep help hidden until `?` is pressed.

### Keybindings

- [x] Remove `n`-prefixed command mode.
- [x] Remove `R` project rename behavior.
- [x] Remove explicit project termination keybindings such as `Ctrl+Q`.
- [x] Implement direct `t` key.
- [x] Implement direct `a` key.
- [x] Implement direct `r` key.
- [x] Implement direct `p` key.
- [x] Implement direct `m` key.
- [x] Implement direct `P` key.
- [x] Implement `?` help toggle.

### Interaction Flow

- [x] Remove extra menus from the target flow.
- [x] Replace normal interactions with inline sidebar states.
- [x] Keep only the minimal centered `P` persist-project overlay.
- [x] Implement hints mode inside the sidebar for `p` and `m`.
- [x] Ensure `Esc` cancels inline input, hints mode, or overlay.
- [x] Ensure `Ctrl+C` closes the sidebar.

### Tests and Cleanup

- [x] Add/update tests for config parsing.
- [x] Add/update tests for `agent-cmd`.
- [x] Add/update tests for volatile vs persistent project behavior.
- [x] Add/update tests for terminal-exit cleanup rules.
- [x] Add/update tests for persistent session restore.
- [x] Add/update tests for cwd tracking state.
- [ ] Add/update tests for project/session uniqueness.
- [ ] Add/update tests for sidebar rendering with badges.
- [x] Add/update tests for hints mode.
- [x] Add/update tests for keybindings.
- [x] Update `README.md` after implementation.
- [x] Run formatting.
- [x] Run the existing test suite.
