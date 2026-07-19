# tflow implementation checklist

This checklist is derived from `.codex/ARCHITECTURE.md`. Open work is ordered by the priority required to reach the first usable release. Completed work is kept separately so the active roadmap contains no mixed checked and unchecked sections.

## Alpha 0.0.1 roadmap

### P0: Runtime ownership and cleanup

- [ ] Store one client-scoped popup wrapper PID and ensure close, toggle, quit, detach, signals, and startup failure terminate and reap the menu child before clearing it.
- [ ] Remove popup records whose client or process is confirmed missing without disturbing another client's live popup.
- [ ] Test multiple volatile instances alongside persistent sessions, popup termination, and stale-record recovery.

### P0: State safety and reconciliation

- [ ] Route every state mutation through one lock-protected operation that reloads the latest store before applying the mutation.
- [ ] Replace direct writes with same-directory durable atomic replacement: mode `0600`, file sync, rename, and directory sync.
- [ ] Distinguish an absent dedicated tmux server from other session-list errors.
- [ ] Reconcile before startup attach and after successful sidebar refresh: remove missing-session metadata and then remove empty projects.
- [ ] Skip reconciliation writes when tmux cannot provide an authoritative session list.
- [ ] Keep failure recovery operation-specific: kill a newly created session, rename a session back, or reload/reconcile after delete failure or external disappearance.
- [ ] Test interrupted writes, concurrent disjoint updates, serial same-resource updates, absent-server cleanup, operational-error preservation, missing-session cleanup, empty-project cleanup, and direct rollback paths.

### P1: Sidebar correctness and performance

- [ ] Open and refresh the sidebar with one global session-list query and local context filtering.
- [ ] Remove unconditional marker synchronization and every per-session tmux write from an unchanged refresh.
- [ ] Compute the selected session index once per render instead of once per row.
- [ ] Keep metadata repair limited to creation, rename, migration, or required reconciliation.
- [ ] Test that an unchanged refresh performs one list query and no `set-option` calls.
- [ ] Test multiple instances and projects: only the current context is displayed and total server session count does not add writes.
- [ ] Test that opening the sidebar does not create or retain an extra popup/menu process.

### P1: Installation and release readiness

- [ ] Change the Go module path to `github.com/rapsnx/tflow` and update internal imports.
- [ ] Move the executable entry point from `cmd/main.go` to `cmd/tflow/main.go` so `go install github.com/rapsnx/tflow/cmd/tflow@latest` installs `tflow`.
- [ ] Update the Nix package for `cmd/tflow` without an executable rename workaround.
- [ ] Correct the README installation instructions and align the Go version badge with `go.mod`.
- [ ] Add CI that checks formatting, `go vet`, and `go test ./...`.
- [ ] Verify `go install` and `nix build --no-link .#tflow`, including an output executable at `bin/tflow`.

## After alpha 0.0.1

- [ ] Broaden tmux compatibility handling across supported 3.2-current error-message variants.
- [ ] Remove the unused session-creation command parameter.
- [ ] Replace custom integer min/max helpers with the standard library where supported.
- [ ] Add performance benchmarks only if command-count tests and real measurements show they are needed.

## Completed

### Runtime and session ownership

- [x] Run ordinary tmux sessions on a dedicated `tflow` socket and attach the live terminal directly.
- [x] Create a startup volatile session and tag it with a collision-resistant instance ID.
- [x] Keep volatile tmux identifiers globally unique across independently started tflow instances while displaying only their labels.
- [x] Keep volatile sessions from being overwritten or renamed when a new session is created.
- [x] Scope volatile listing, creation, rename, deletion, and explicit quit cleanup to the current instance.
- [x] Keep persistent sessions alive when an instance exits.
- [x] Enable `remain-on-exit` for managed panes.
- [x] Open the sidebar and quit confirmation as tmux popups without resizing the active terminal.
- [x] Validate state before startup session creation and roll back startup on later setup failure.
- [x] Preserve the instance marker on a newly attached client so it survives tmux-native session switches.
- [x] Clean up only the detached client's volatile sessions through an idempotent client-detach hook.
- [x] Refresh a volatile session's tmux display-label marker on rename and roll back if the marker update fails.
- [x] Cover attach-marker preservation, repeated detach cleanup, and volatile rename rollback with regression tests.

### State and project model

- [x] Use one strict JSON schema containing project order, project workdirs, session projects, and session labels.
- [x] Reject unknown and obsolete fields with a clear startup error.
- [x] Persist only project-session metadata; keep volatile ownership out of the store.
- [x] Use `$XDG_STATE_HOME/tflow/store.json`, falling back to `~/.local/state/tflow/store.json`.
- [x] Remove YAML, legacy migration, session types, project protection, cluster, and agent settings.
- [x] Keep project names unique, labels unique within a project, and project order stable.
- [x] Use project-scoped internal identifiers so different projects can reuse labels.
- [x] Create an initial random-animal session in each new project's persisted workdir.
- [x] Delete project metadata with its final session and select the next project or a volatile fallback.

### Sidebar and dialogs

- [x] Render a centered `TFLOW` badge and one legible `live` badge on the active row.
- [x] Render the active row with continuous highlighting across indentation, badge, spacing, and label.
- [x] Show project and session labels in tmux status without exposing volatile instance IDs.
- [x] Implement session navigation and project switching with the documented shortcuts.
- [x] Show non-blocking inline `?` help below the session list, toggle it with `?`, and hide it on the next shortcut.
- [x] Use consistent centered dialog cards with one divider, prefix-free inputs, and centered keycap-only `Enter` / `Esc` footers.
- [x] Use concise confirmation text and destructive red accents.
- [x] Keep dialogs clear of the conditional bottom status row and distinguish warnings from errors.
- [x] Close the sidebar and restore terminal focus after successful actions.

### Packaging and refactoring

- [x] Keep reusable code in focused `internal/store`, `internal/tmux`, and `internal/ui` packages.
- [x] Split oversized state, tmux, UI, and test files by responsibility.
- [x] Remove dead session-type, project-tree, metadata-header, command-mode, move-session, and unused style code.
- [x] Keep the Home Manager module package-only and stop it from managing `store.json`.
- [x] Build the current `cmd/main.go` with Nix and install it as `bin/tflow`.
- [x] Verify the package with Home Manager evaluation coverage and `nix build --no-link .#tflow`.

### Verification coverage

- [x] Cover animal naming, collisions, project defaults, and absence of runtime HTTP calls.
- [x] Cover strict store decoding and canonical round trips.
- [x] Cover instance-scoped volatile behavior, global quit invocation, and startup rollback.
- [x] Cover sidebar badges, help, dialogs, project context, deletion navigation, fallback creation, and focus restoration.
- [x] Run formatting, `go test ./...`, `go build ./...`, and `go vet ./...` for completed implementation sessions.
