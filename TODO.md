# TODO

_Last verified against the repository on 2026-07-06._

## Open

### Bugs
- [ ] Remove the remaining `default` project fallback.
  - Persistent sessions are still auto-assigned to `default` when no project is set.
  - `default` is still treated as a special built-in project in rename, delete, and sync flows.
  - The original intent was to remove the default project concept entirely.

- [ ] Deleting a project should delete the sessions that belong to it.
  - The current implementation reassigns those sessions back to `default` instead of removing them.

- [ ] Finish the session terminology cleanup.
  - Some user-facing rename and validation messages still say `Section` instead of `Session`.

### Planned changes
- [ ] Rework the startup and sidebar model.
  - Startup currently creates a random `<animal>-temp` session, not a project.
  - The sidebar is still a project tree, not the planned session-first view.
  - Sessions are project-scoped already, but the broader conceptual overhaul is not finished.

- [ ] Replace the current keymap with the planned workflow.
  - Current creation still uses the prefixed flow `np`, `nt`, `nk`, `nc`, and `na`.
  - There is no direct `nvim` session creation flow.
  - `r` still handles both project and session renaming instead of splitting `r` and `R`.
  - `Ctrl+Q` is not bound; quit-all currently exists only via `:qa` or `:qa!`.
  - There is no `P` project switcher overlay.

## Completed

### Bugs
- [x] Introduce explicit project/session terminology and typed sessions.
  - Sessions have explicit types: `terminal`, `k9s`, and `agent`.

- [x] Add deletion confirmation before removing a project or session.

- [x] Support project protection with `protect: true`.

### Features
- [x] Make project configuration editable as YAML with `e`.
  - Supported fields include `name`, `workdir`, `cluster`, `agent-binary`, and `protect`.

- [x] Persist projects on disk.

- [x] Support `cluster.path` and `cluster.connection-cmd`.

- [x] Change new-item creation to prefixed key sequences.
  - [x] `np` creates a project.
  - [x] `nt` creates a terminal session.
  - [x] `nk` creates a `k9s` session.
  - [x] `nc` creates an agent session.

- [x] Add move mode with project-prefix targeting.

- [x] Improve the general design and styling.

- [x] Add `README.md` and move the logo into `docs/`.

- [x] Replace the default startup session with a temporary `<animal>-temp` session.

- [x] Keep temporary sessions out of the project list until they are assigned.

- [x] Allow adding the current temp session to the selected project with `na`.

- [x] Destroy unassigned temporary sessions when their terminal detaches.

- [x] Add an app config file with a configurable projects directory.

- [x] Add theme and color configuration, defaulting to Catppuccin.

- [x] Start with no projects when the config is empty.

- [x] Add `flake.nix` packaging for `tflow`.

- [x] Allow configuring the agent binary per project.

- [x] Add a Home Manager module with project configuration support.
