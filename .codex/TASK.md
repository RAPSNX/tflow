# Alpha implementation checklist

## Bug fixes

- [x] Fix `Ctrl+F` sidebar toggle so opening and closing the sidebar does not shift the active terminal prompt.
- [x] Keep the active terminal stable while the sidebar opens as a real tmux popup.
- [x] Add regression coverage for the tmux popup sidebar behavior so the active terminal is not resized directly.

## Package split and file-size baseline

- [x] Move tmux process and session responsibilities into a focused package with small files.
- [x] Move persistent state loading and saving into a focused store package with small files.
- [x] Split `internal/ui/model.go` into smaller UI files grouped by responsibility.
- [x] Split oversized UI tests so they live next to the behavior they cover.
- [x] Remove package responsibilities that do not belong in `internal/ui`.

## Remove obsolete config and project YAML

- [x] Remove editable app config support.
- [x] Remove per-project YAML config support.
- [x] Remove code paths that read or write `config.yaml`.
- [x] Remove code paths that read or write project config files.
- [x] Remove tests that only cover the deleted config behavior.
- [x] Remove temp YAML-based project settings edit flow.
- [x] Remove YAML marshal and parse helpers kept only for project settings editing.
- [x] Keep project settings backed directly by `$XDG_STATE_HOME/tflow/store.json`.
- [x] Update the Home Manager module to render `$XDG_STATE_HOME/tflow/store.json` instead of `config.yaml` plus project YAML files.

## Remove obsolete UI and interaction behavior

- [x] Remove project and session tree behavior and replace it with a flat session-list assumption.
- [x] Remove session move-to-project behavior.
- [x] Remove key handling and prompts that exist only for the deleted move flow.
- [x] Remove UI states and rendering paths that no longer match the architecture.
- [x] Remove `:` command mode.
- [x] Remove `q` / `qa` command-style quit handling.
- [x] Remove rendering and status paths kept only for command mode.
- [x] Remove project-settings interaction that still depends on temp YAML editing.

## Introduce the new store foundation

- [x] Create a single store at `$XDG_STATE_HOME/tflow/store.json`.
- [x] Define store data for project order, project settings, and project membership for sessions.
- [x] Load an empty store when the file does not exist.
- [x] Fail startup with a clear error when the store file is invalid.
- [x] Add tests for store load, empty-store creation, and invalid-store failure.

## Tmux runtime baseline

- [x] Run `tflow` on its own tmux socket.
- [x] Start with one volatile tmux session and attach the user to it.
- [x] Keep the active terminal as a real tmux terminal.
- [x] Keep persistent sessions as ordinary tmux sessions grouped by metadata.
- [x] Track volatile sessions per `tflow` instance.
- [x] Remove only the current instance's volatile sessions on normal exit or confirmed `Ctrl+Q`.
- [x] Propagate the current `tflow` instance id through popup and menu commands so volatile-session cleanup targets the correct tmux sessions.

## Sidebar popup and top badges

- [x] Toggle the sidebar as a real tmux popup with `Ctrl+F`.
- [x] Show current project and session in the top UI.
- [x] Keep the project badge empty in volatile mode.
- [x] Render the sidebar with a `TFLOW` header, a flat session list, and a command/status area.
- [x] Close the sidebar after switching sessions.

## Session list navigation

- [ ] Show sessions as a flat list for the current context.
- [ ] Support `j` / `k` movement through the session list.
- [ ] Support `Enter` to switch to the selected session and close the sidebar.
- [ ] Support `Ctrl+C` to close the sidebar when it is open.
- [ ] Support `Esc` to cancel prompts or confirmations before closing the sidebar.

## Project creation and switching

- [ ] Create a default `code` session when creating a project.
- [ ] Support `p` to start project switching from the command line.
- [ ] Show all existing projects in a readable newline-separated list.
- [ ] Accept a unique typed prefix and switch on `Enter`.
- [ ] Switch to the first session of the selected project and close the sidebar.
- [ ] Require confirmation when switching from a volatile session to a project.
- [ ] Switch directly when moving from one project to another.

## Session and project management

- [ ] Support `n` to create a new session.
- [ ] Start project sessions in the project `workdir` when one is set.
- [ ] Start non-project sessions in the current working directory.
- [ ] Support `N` to create a new project.
- [ ] Support `r` to rename the selected session or project.
- [ ] Support `e` to update project settings.
- [ ] Support `d` to delete the selected session or project with confirmation.
- [ ] Require confirmation before deleting the last session of a project.

## Quit flow

- [ ] Support `Ctrl+Q` to open a quit confirmation flow.
- [ ] Remove only the current instance's volatile sessions on confirmed quit.
- [ ] Leave persistent project sessions untouched on quit.
- [ ] Keep quit behavior aligned with the tmux-native runtime model.

## Cleanup and verification

- [ ] Remove dead YAML/config and command-mode code and tests left behind by the deleted flows.
- [ ] Keep production files small and focused after the refactor.
- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Run `go build ./...`.
