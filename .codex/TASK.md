# Architecture implementation checklist

This checklist is derived from `.codex/ARCHITECTURE.md`. Checked items were verified against the implementation and tests on `main`; unchecked items are required to complete the architecture.

## Tmux runtime and lifecycle

- [x] Run all managed sessions on the dedicated `tflow` tmux socket.
- [x] Use ordinary tmux sessions and attach the live terminal directly to tmux.
- [x] Create one volatile session on startup and tag it with a collision-resistant instance ID.
- [x] Clean up only volatile sessions owned by the exiting instance.
- [x] Keep persistent project sessions alive when the current instance exits.
- [x] Open the sidebar as a left-anchored tmux popup without resizing the active terminal.
- [x] Suppress benign tmux errors while toggling or closing the popup.
- [ ] Validate or create `store.json` before creating the startup tmux session.
- [ ] Roll back a newly created startup session if temporary tagging, control-mode setup, or later startup preparation fails.
- [ ] Mark every session created outside a project as volatile and owned by the current instance.
- [ ] Show and manage only the current instance's volatile sessions while outside a project.
- [ ] Ensure normal exit and confirmed `Ctrl+Q` remove every volatile session created by that instance.
- [ ] Bind `Ctrl+Q` in tmux so quit confirmation opens from the live terminal while the sidebar is closed.
- [ ] Add an internal `open-quit` command that opens the popup directly in quit-confirmation mode.

## Top UI and sidebar

- [x] Show project and session badges through the tmux top status UI.
- [x] Keep the project badge value empty for volatile sessions.
- [x] Show project-scoped session display labels instead of internal tmux identifiers.
- [x] Render a flat session list for the active project context.
- [x] Support `j` and `k` navigation with `Enter` switching to the selected session and closing the popup.
- [x] Close the popup with `Ctrl+C` and cancel an active prompt or confirmation with `Esc`.
- [ ] Center the `TFLOW` header and remove project/session metadata from the popup header.
- [ ] Remove the always-visible shortcut line from the normal sidebar.
- [ ] Add a `?` help view containing every supported shortcut on its own row.
- [ ] Make `Esc` return from help to the session list without closing the popup.
- [ ] Keep the inline command/status area available without showing metadata or help by default.
- [ ] Remove undocumented normal-mode aliases and legacy confirmation shortcuts so key dispatch and help agree exactly.
- [ ] Add table-driven coverage for every documented key and for removed bindings.

## Projects and sessions

- [x] Keep project names unique and preserve project order.
- [x] Create a project-scoped default session displayed as `code`.
- [x] Use project-scoped tmux identifiers so different projects can reuse session labels.
- [x] Persist session display labels independently from tmux identifiers.
- [x] Reject duplicate display labels within one project.
- [x] Migrate unscoped project sessions to scoped tmux identifiers without losing project membership or selection.
- [x] Rename project session identifiers with rollback after a partial tmux rename failure.
- [x] Start project sessions in the configured project `workdir`.
- [x] Start sessions outside a project in the current working directory.
- [x] Switch projects from a newline-separated prefix search, require a unique match, and select the target project's first session.
- [x] Require confirmation when switching from a volatile session to a project and switch directly between projects.
- [x] Support `r` and `d` for the selected session and `R` and `D` for the current project.
- [x] Require confirmation before deleting a session, including the final session of a project.
- [ ] Persist the current working directory as a newly created project's default `workdir`.
- [ ] Create the project's default `code` session in that persisted directory.
- [ ] Keep sidebar context aligned with the active volatile session instead of selecting the first stored project automatically.
- [ ] Keep project creation from changing the active sidebar context until the user switches projects.
- [ ] Make `n` open the session-name prompt directly and create a plain terminal session.
- [ ] Remove the terminal/k9s/agent session-kind submenu and all session-type badges and startup commands.
- [ ] Make `e` edit and persist only the current project's `workdir`.
- [ ] Remove project protection, cluster configuration, and agent-binary behavior.
- [ ] On confirmation, delete the project metadata together with its final session so every remaining project stays switchable.
- [ ] Ensure volatile session rename and deletion never write persistent project metadata.

## State

- [x] Store persistent metadata at `$XDG_STATE_HOME/tflow/store.json`.
- [x] Create an empty state file when none exists.
- [x] Fail startup with a path-qualified error when the state file contains invalid JSON.
- [x] Persist project order, session membership, display labels, and project workdirs.
- [x] Avoid user-edited `config.yaml` and per-project YAML files.
- [ ] Reduce the canonical schema to `project_order`, project `workdir` entries, `session_projects`, and `session_labels`.
- [ ] Remove `session_types`, `protect`, `agent_binary`, and `cluster` from store types, normalization, encoding, and UI state.
- [ ] Replace legacy format detection and migration with one strict canonical decoder.
- [ ] Reject unknown fields and obsolete fields with a clear startup error that names the offending field.
- [ ] Stop reading legacy state from `$XDG_CONFIG_HOME/tflow/state.json`.
- [ ] Persist metadata only for project sessions; never store instance-owned volatile sessions.
- [ ] Keep the state file writable and exclusively owned by the application.

## Packaging and cleanup

- [x] Keep reusable code in focused `internal/store`, `internal/tmux`, and `internal/ui` packages.
- [x] Remove YAML dependencies, command mode, move-session flow, and obsolete editable-config code.
- [ ] Remove Home Manager project settings and `home.file` generation for `store.json`; keep the module package-only.
- [ ] Fix the Nix build target to compile `cmd/main.go` and install the executable as `bin/tflow`.
- [ ] Remove dead session-type, project-tree, metadata-header, and unused style helpers.
- [ ] Split `internal/store/state.go` by schema, codec, and normalization responsibilities.
- [ ] Split tmux popup/control, instance ownership, and quit behavior into focused files.
- [ ] Split UI message handling, key dispatch, modal updates, and lifecycle orchestration into focused files.
- [ ] Split oversized store, tmux, and UI tests by the behavior they cover.

## Verification

- [ ] Add regression tests for global quit invocation and instance-scoped volatile session creation and cleanup.
- [ ] Add rendering tests for the centered header, metadata-free default sidebar, and one-shortcut-per-row help view.
- [ ] Add context tests covering volatile startup with existing projects and project creation without implicit switching.
- [ ] Add tests for persisted project workdirs and final-session project deletion.
- [ ] Add strict-store tests for every removed field, unknown fields, and the canonical round trip.
- [ ] Add startup rollback tests for state validation and tmux setup failures.
- [ ] Add Home Manager evaluation coverage confirming it does not manage `store.json`.
- [ ] Run `gofmt` on every changed Go file.
- [ ] Run `go test ./...`.
- [ ] Run `go build ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `nix build --no-link .#tflow` and verify the output contains `bin/tflow`.
