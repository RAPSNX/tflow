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
- [x] Ignore or clean stale runtime state for persistent projects removed from `config.yaml`.
- [x] Ensure `state.json` is never required to recreate the persistent project list.

### Startup and Exit

- [x] Load persistent projects from `config.yaml` on startup.
- [x] Load runtime and restore state from `state.json` on startup.
- [x] Clean up stale runtime state on startup.
- [x] Create a volatile random animal project on every new `tflow` start.
- [x] Create a default `code` session inside the volatile project.
- [x] Attach the startup terminal to the volatile project's `code` session.
- [x] Ensure the sidebar can be toggled with `Ctrl+F`.
- [x] Delete the current volatile project and all sessions inside it when the owning terminal exits.
- [x] Keep terminal-exit cleanup idempotent.
- [x] Never delete persistent projects or persistent sessions during volatile cleanup.
- [x] Do not stop or rename persistent projects or sessions during terminal exit handling.

### Project Lifecycle

- [x] Track whether a project is volatile or persistent.
- [x] Ensure random animal projects are always volatile.
- [x] Hide volatile projects from the project list.
- [x] Prevent switching into volatile projects.
- [x] Define persistent projects only in `~/.config/tflow/config.yaml`.
- [x] List persistent projects in the `Projects` section.
- [x] Ensure persistent projects and sessions survive terminal exit.
- [x] Implement `P` to persist the current volatile project.
- [x] Ask for `name`, `workdir`, and `agent-cmd` when persisting a project.
- [x] Rename the volatile project to the chosen persistent project name during persist.
- [x] Keep existing sessions when persisting.
- [x] Mark the project as persistent after persisting.
- [x] Add the persisted project to the `Projects` section.
- [x] Show a no-op status message when `P` is used on an already persistent project.

### Session Lifecycle

- [x] Ensure sessions always belong to exactly one project.
- [x] Ensure logical session names are unique only inside one project.
- [x] Allow the same logical session name in different projects.
- [x] Keep tmux session names internal and globally unique.
- [x] Ask for a new terminal session name inside the sidebar.
- [x] Use the project `workdir` for new terminal sessions when configured.
- [x] Fall back to the current or default directory for new terminal sessions when no `workdir` is configured.
- [x] Switch to the newly created terminal session after `t`.
- [x] Ask for a new agent session name inside the sidebar.
- [x] Use the project `agent-cmd` for new agent sessions.
- [x] Show a clear error when creating an agent session without an agent command.
- [x] Switch to the newly created agent session after `a`.
- [x] Rename the selected session from an inline sidebar prompt.
- [x] Validate session-name uniqueness inside the current project during rename.
- [x] Update runtime state after renaming a session.
- [x] Update the internal tmux backing name when renaming a session.
- [x] Move the selected session only to another persistent project.
- [x] Use hints mode for move-session flows.
- [x] Allow moving the last session out of a project.
- [x] Keep the source project valid when its last session is moved out.
- [x] Update runtime state after moving a session.
- [x] Do not switch projects automatically after moving a session.
- [x] Track cwd with tmux `#{pane_current_path}`.
- [x] Update cwd when the sidebar opens or closes, before switching sessions, and before shutdown where possible.
- [x] Restore persistent sessions in their last known cwd.
- [x] Create a default `code` session in the project `workdir` when a persistent project has no restore state.

### Sidebar Layout

- [x] Replace the project/session tree with separate `Sessions` and `Projects` sections.
- [x] Show only current-project sessions in `Sessions`.
- [x] Show only persistent projects in `Projects`.
- [x] Keep volatile projects out of `Projects`.
- [x] The Session and the Project sections are distinct of each. So they are two seperate lists with boarders.
- [x] The switch between project section and sessions section is via <TAB>.
- [x] Mark the current session and current persistent project with `[live]`.
- [x] Support selecting project rows inside the sidebar.
- [x] Allow `Enter` to activate the selected project row as well as the selected session row.
- [x] Show sessions for the selected project during project-selection flows.
- [x] Fix selected-row highlighting when `[live]` or `[agent]` badges are present.
- [x] Render `[live]`, `[agent]`, and hint badges in a consistent badge style.
- [x] Highlight selectable hint prefixes inside the sidebar.
- [x] Center the `TFLOW` badge and section headers to match the target layout.
- [x] Keep the sessions section at a minimum visible height of five rows unless fewer sessions exist.
- [x] Keep styling consistent with the existing bordered TUI style.

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
- [x] Keep killing the owning terminal as the cleanup mechanism for volatile projects.
- [x] Ensure `Ctrl+C` closes the sidebar from normal, inline-input, hints, and persist-overlay states.

### Interaction Flow

- [x] Remove extra menus from the target flow.
- [x] Replace normal interactions with inline sidebar states.
- [x] Keep create-session flow inside inline sidebar input.
- [x] Keep rename-session flow inside inline sidebar input.
- [x] Keep switch-project and move-session flows inside sidebar hints mode.
- [x] Render hints inside the existing sidebar.
- [x] Limit hints to persistent projects.
- [x] Support multi-character hint disambiguation.
- [x] Keep hints deterministic while active.
- [x] Execute the action when a unique hint is selected.
- [x] Keep only the minimal centered `P` persist-project overlay.
- [x] Ensure `Esc` cancels inline input, hints mode, or the persist overlay.

### Help and Overlay

- [x] Keep help hidden until `?` is pressed.
- [x] Make `?` toggle help on and off.
- [x] Render help with one action per line.
- [x] Keep help wording short.
- [x] Keep the `P` persist-project overlay minimal and centered.
- [x] Keep the persist overlay visually consistent with the Catppuccin TUI style.
- [x] Write persisted projects to `~/.config/tflow/config.yaml`.
- [x] Use persisted `workdir` as the default cwd for future new sessions.
- [x] Use persisted `agent-cmd` for future new agent sessions.

### Resolved Design Decisions

- [x] Track selected sessions globally when multiple terminals attach to the same persistent project.
- [x] Write moved-session restore state no later than sidebar close/open or session switch.
- [x] Allow `P` to persist a project with empty `workdir` or `agent-cmd`.

## !Bugs
- [x] The highlighting when session has a badge is broken or looks odd.
- [x] after implementation of `?` the legacy docs still displayed
- [x] hint mode should Highlight or change the color of the hint character, rather then prefix it.
- [x] Not all badges have the same type of style, all badges should look like the `TFLOW` badge at the top. Exceppt for color.
      Color should match in a pretty way.

### Tests and Cleanup

- [x] Add or update tests for config parsing.
- [x] Add or update tests for `agent-cmd`.
- [x] Add or update tests for volatile vs persistent project behavior.
- [x] Add or update tests for terminal-exit cleanup rules.
- [x] Add or update tests for persistent session restore.
- [x] Add or update tests for cwd tracking state.
- [x] Add or update tests for project and session uniqueness constraints.
- [x] Add or update tests for sidebar rendering with badges, centered headers, and project-row selection.
- [x] Add or update tests for hints mode.
- [x] Add or update tests for full keybinding behavior, including `Ctrl+C` in modal states and auto-switch after creating sessions.
- [x] Update `README.md` after implementation.
- [x] Run formatting.
- [x] Run the existing test suite.
