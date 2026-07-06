# TODO

_Last verified against the repository on 2026-07-06._

## Open

### Bugs
- [ ] Remove the remaining `default` project fallback.
  - The current code still uses `default` as the implicit project for persistent sessions.
  - Deleting or renaming behavior still treats `default` as a special built-in project.
  - The original intent was to remove the default project concept entirely.

- [ ] Deleting a project should delete the sessions that belong to it.
  - The current implementation reassigns those sessions back to `default` instead of removing them.

### Features
- [ ] Rework the session and project model.
  1. Starting `tflow` should create a random animal project.
  2. The sidebar should focus on sessions, with each project only showing its own sessions.
  3. Replace the current keymap with the planned workflow:
     - `t` creates a terminal session
     - `a` creates an agent session
     - `k` creates a `k9s` session
     - `n` creates a `nvim` session
     - `r` renames the session
     - `R` renames the project
     - `Ctrl+Q` kills everything
     - `P` opens a project switcher overlay and `Enter` switches into it

- [ ] Simplify the Nix packaging and Home Manager module.
  - Refactor `flake.nix` to match the lighter pattern used in the other RAPSNX repositories.
  - Reduce Home Manager module surface area.
  - Remove the module package override option if it is no longer needed.

## Done

### Bugs
- [x] Fix session terminology: `tflow` has projects and sessions, and sessions have explicit types.
- [x] Add deletion confirmation before removing a project or session.
- [x] Support project protection with `protect: true`.

### Features
- [x] Make project configuration editable as YAML with `name`, `workdir`, and `cluster`.
- [x] Persist projects on disk.
- [x] Support `cluster.path` and `cluster.connection-cmd`.
- [x] Change new-item creation to prefixed key sequences:
  - [x] `np` new project
  - [x] `nt` new terminal session
  - [x] `nk` new `k9s` session
  - [x] `nc` new agent session
- [x] Add move mode with project-prefix targeting.
- [x] Improve the general design and styling.
- [x] Add a `README.md` and move the logo into `docs/`.
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
