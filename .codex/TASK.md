# Architecture implementation checklist

This checklist is derived from `.codex/ARCHITECTURE.md`. Checked items were verified against the implementation and tests on `main`; unchecked items are required to complete the architecture.

## Planned work sessions

Every previously open task is grouped below for a dedicated implementation session. Task wording is preserved; the sessions are ordered by dependency.

### Session 1: Animal naming foundation

- [x] Fetch, review, and compile a fixed list of exactly 25 animal names; do not make runtime API requests.
- [x] Give the startup volatile session and every initial project session a random plain animal name without a `-temp` suffix.
- [x] Use unique two-animal volatile-session names after the single-name pool is exhausted, and numeric suffixes only after all combinations are exhausted.
- [x] Create a project-scoped initial session displayed with a random animal name.
- [x] Create the project's initial randomly named session in that persisted directory.
- [x] Add deterministic tests for startup and project-default animal names, collisions, two-animal fallback, and the absence of runtime HTTP usage.

### Session 2: Canonical persistent state

- [x] Reduce the canonical schema to `project_order`, project `workdir` entries, `session_projects`, and `session_labels`.
- [x] Remove `session_types`, `protect`, `agent_binary`, and `cluster` from store types, normalization, encoding, and UI state.
- [x] Replace legacy format detection and migration with one strict canonical decoder.
- [x] Reject unknown fields and obsolete fields with a clear startup error that names the offending field.
- [x] Stop reading legacy state from `$XDG_CONFIG_HOME/tflow/state.json`.
- [x] Persist metadata only for project sessions; never store instance-owned volatile sessions.
- [x] Add a newly created project session to the model before saving, so project-session metadata is retained by the persistence filter.
- [x] Keep the state file writable and exclusively owned by the application.
- [x] Add strict-store tests for every removed field, unknown fields, and the canonical round trip.

### Session 3: Terminal UI and lifecycle polish

- [x] Render the centered `TFLOW` badge using the documented blue filled badge style.
- [x] Render a green filled `live` badge immediately before the active session label, including in a selected row.
- [x] Apply the documented structured-card layout to every input, rename, settings, and confirmation dialog.
- [x] Add dialog headers, dividers, context or bordered input areas, and `Enter`/`Esc` keycap footers.
- [x] Use red header and primary-action keycaps exclusively for deletion confirmations.
- [x] Add rendering tests for the `TFLOW` and active-session `live` badges, including selected-row contrast.
- [x] Add rendering tests covering the shared dialog structure and destructive-confirmation red accents.
- [x] Add regression coverage for dialog project switching, persistent status rendering, deletion navigation, volatile fallback, and focus restoration.

### Session 4: Packaging and release verification

- [ ] Remove Home Manager project settings and `home.file` generation for `store.json`; keep the module package-only.
- [ ] Fix the Nix build target to compile `cmd/main.go` and install the executable as `bin/tflow`.
- [ ] Add Home Manager evaluation coverage confirming it does not manage `store.json`.
- [ ] Run `nix build --no-link .#tflow` and verify the output contains `bin/tflow`.

### Session 5: Post-feature refactoring

- [x] Remove dead session-type, project-tree, metadata-header, and unused style helpers.
- [x] Split `internal/store/state.go` by schema, codec, and normalization responsibilities.
- [x] Split tmux popup/control, instance ownership, and quit behavior into focused files.
- [x] Split UI message handling, key dispatch, modal updates, and lifecycle orchestration into focused files.
- [x] Split oversized store, tmux, and UI tests by the behavior they cover.

## Completed checklist

## Tmux runtime and lifecycle

- [x] Run all managed sessions on the dedicated `tflow` tmux socket.
- [x] Use ordinary tmux sessions and attach the live terminal directly to tmux.
- [x] Create one volatile session on startup and tag it with a collision-resistant instance ID.
- [x] Clean up only volatile sessions owned by the exiting instance.
- [x] Keep persistent project sessions alive when the current instance exits.
- [x] Open the sidebar as a left-anchored tmux popup without resizing the active terminal.
- [x] Suppress benign tmux errors while toggling or closing the popup.
- [x] Validate or create `store.json` before creating the startup tmux session.
- [x] Roll back a newly created startup session if temporary tagging, control-mode setup, or later startup preparation fails.
- [x] Mark every session created outside a project as volatile and owned by the current instance.
- [x] Show and manage only the current instance's volatile sessions while outside a project.
- [x] Ensure normal exit and confirmed `Ctrl+Q` remove every volatile session created by that instance.
- [x] Bind `Ctrl+Q` in tmux so quit confirmation opens from the live terminal while the sidebar is closed.
- [x] Add an internal `open-quit` command that opens the popup directly in quit-confirmation mode.

## Top UI and sidebar

- [x] Show project and session badges through the tmux top status UI.
- [x] Keep the project badge value empty for volatile sessions.
- [x] Show project-scoped session display labels instead of internal tmux identifiers.
- [x] Render a flat session list for the active project context.
- [x] Support `j` and `k` navigation with `Enter` switching to the selected session and closing the popup.
- [x] Close the popup with `Ctrl+C` and cancel an active prompt or confirmation with `Esc`.
- [x] Center the `TFLOW` header and remove project/session metadata from the popup header.
- [x] Remove the always-visible shortcut line from the normal sidebar.
- [x] Add a `?` help view containing every supported shortcut on its own row.
- [x] Make `Esc` return from help to the session list without closing the popup.
- [x] Keep the inline command/status area available without showing metadata or help by default.
- [x] Remove undocumented normal-mode aliases and legacy confirmation shortcuts so key dispatch and help agree exactly.
- [x] Add table-driven coverage for every documented key and for removed bindings.

## Projects and sessions

- [x] Keep project names unique and preserve project order.
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
- [x] Persist the current working directory as a newly created project's default `workdir`.
- [x] Keep sidebar context aligned with the active volatile session instead of selecting the first stored project automatically.
- [x] Keep project creation from changing the active sidebar context until the user switches projects.
- [x] Make `n` open the session-name prompt directly and create a plain terminal session.
- [x] Remove the terminal/k9s/agent session-kind submenu and all session-type badges and startup commands.
- [x] Make `e` edit and persist only the current project's `workdir`.
- [x] Remove project protection, cluster configuration, and agent-binary behavior.
- [x] On confirmation, delete the project metadata together with its final session so every remaining project stays switchable.
- [x] Ensure volatile session rename and deletion never write persistent project metadata.

## State

- [x] Store persistent metadata at `$XDG_STATE_HOME/tflow/store.json` when set, otherwise at `~/.config/tflow/store.json`; never use `~/.local/share`.

- [x] Create an empty state file when none exists.
- [x] Fail startup with a path-qualified error when the state file contains invalid JSON.
- [x] Persist project order, session membership, display labels, and project workdirs.
- [x] Avoid user-edited `config.yaml` and per-project YAML files.

## Packaging and cleanup

- [x] Keep reusable code in focused `internal/store`, `internal/tmux`, and `internal/ui` packages.
- [x] Remove YAML dependencies, command mode, move-session flow, and obsolete editable-config code.

## Verification

- [x] Add regression tests for global quit invocation and instance-scoped volatile session creation and cleanup.
- [x] Add rendering tests for the centered header, metadata-free default sidebar, and one-shortcut-per-row help view.
- [x] Add context tests covering volatile startup with existing projects and project creation without implicit switching.
- [x] Add tests for persisted project workdirs and final-session project deletion.
- [x] Add startup rollback tests for state validation and tmux setup failures.
- [x] Run `gofmt` on every changed Go file.
- [x] Run `go test ./...`.
- [x] Run `go build ./...`.
- [x] Run `go vet ./...`.


## Dialog, status, and lifecycle follow-up

- [x] Replace inline project switching with a searchable dialog that supports `Up`/`Down` selection and `Enter` activation.
- [x] Render management flows as dialogs and keep the conditional bottom status row visible while a dialog is open.
- [x] Render recoverable action problems as yellow warnings and operation failures as red errors.
- [x] Start every new project with a renameable `code` session.
- [x] Explain in the final-session confirmation that the entire project will be deleted.
- [x] After a deletion, activate the next project's first session, wrapping by project order, or create a volatile fallback session when none remain.
- [x] Close the sidebar and restore terminal focus after successful actions, after synchronizing the top project and session badges.
