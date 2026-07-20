# tflow implementation checklist

This checklist is derived from `.codex/ARCHITECTURE.md`. Work is ordered by priority for the first usable release.

## Alpha 0.0.1

### P0: Session identity

* [x] Use generated internal tmux session IDs instead of project and display names.
* [x] Use `tflow-p-<id>` for persistent sessions.
* [x] Use `tflow-v-<instance-id>-<id>` for volatile sessions.
* [x] Store project names and session labels only as metadata.
* [x] Stop renaming tmux sessions when a project or display label is renamed.
* [x] Ensure persistent IDs are globally unique.
* [x] Ensure volatile IDs are unique across independently started tflow instances.
* [x] Test project and session renames without tmux session renames.

### P0: Persistent state model

* [x] Replace parallel session maps with projects containing ordered session records.
* [x] Store only:

  * project name
  * project workdir
  * ordered persistent sessions
  * internal tmux session ID
  * session display label
* [x] Keep volatile sessions and instance ownership out of the store.
* [x] Ignore unknown JSON fields.
* [x] Reject malformed JSON with a clear path-qualified error.
* [x] Keep the state path at `$XDG_STATE_HOME/tflow/store.json` when `XDG_STATE_HOME` is set and non-empty, falling back to `~/.local/state/tflow/store.json`.
* [x] Set the state directory mode to `0700`.
* [x] Set the state file mode to `0600`.
* [x] Update state codec and normalization tests for the new schema.

### P0: Simple atomic state updates

* [x] Serialize mutations with one advisory store lock.
* [x] Reload the latest state while holding the lock.
* [x] Apply the requested mutation to the reloaded state.
* [x] Write the complete JSON document to a same-directory temporary file.
* [x] Close the temporary file before renaming it over `store.json`.
* [x] Do not use file `fsync`.
* [x] Do not use directory `fsync`.
* [x] Remove the temporary file when writing or renaming fails.
* [x] Test that failed writes leave the previous JSON file unchanged.
* [x] Test that concurrent disjoint mutations do not overwrite each other.

### P0: Startup reconciliation

* [x] List tmux sessions once during startup.
* [x] Remove metadata for persistent sessions that no longer exist.
* [x] Remove projects that have no remaining sessions.
* [x] Persist reconciled state only when it changed.
* [x] Treat an absent dedicated tmux server as an empty session list.
* [x] Do not remove metadata when tmux returns another operational error.
* [x] Do not retain missing persistent-session metadata for lazy restoration.
* [x] Do not reconcile or write state during ordinary sidebar refreshes.
* [x] Test missing-session cleanup without lazy restoration, empty-project cleanup, and tmux-error preservation.

### P0: Operation failure handling

* [x] Kill a newly created tmux session when its metadata cannot be persisted.
* [x] Treat cleanup of an already missing session or popup as successful.
* [x] Return the original operation error to the user.
* [x] Remove project-wide rename rollback logic.
* [x] Remove generalized compensation or transaction helpers.
* [x] Leave non-critical inconsistencies for the next startup reconciliation.
* [x] Test failed session creation persistence and already-missing cleanup.

### P1: Volatile instance lifecycle

* [ ] Keep one instance ID exclusively on the attached tmux client; never inherit or consult it through the tmux server environment.
* [x] Preserve the instance ID when the client switches between sessions.
* [ ] Remove only the detached client's volatile sessions.
* [x] Never remove persistent sessions during instance cleanup.
* [ ] Never remove volatile sessions belonging to another instance.
* [x] Keep cleanup idempotent.
* [x] Test multiple simultaneous tflow instances and repeated cleanup.

### P1: Graceful signal shutdown

* [x] Cancel the runtime context on SIGHUP, SIGINT, and SIGTERM.
* [x] Pass cancellation only to the attached tmux client and Bubble Tea popup program.
* [x] Clean the owning volatile instance once when the attached client is canceled or exits.
* [x] Keep signal cleanup scoped to the owning instance and preserve persistent and foreign volatile sessions.
* [x] Exit a canceled popup without dispatching the user-facing quit action.
* [x] Test canceled attach cleanup and canceled-popup behavior.

### P1: Popup lifecycle

* [x] Let tmux own popup process lifetime.
* [x] Keep only a client-scoped popup-visible marker when needed for toggle behavior.
* [x] Close popups through `tmux display-popup -C`.
* [x] Clear stale popup markers when no popup exists.
* [x] Ignore benign `no popup` and `client not found` cleanup errors.
* [x] Do not store popup PIDs.
* [x] Do not implement child-process reaping or a popup process registry.
* [x] Test popup open, toggle, close, quit, and stale-marker cleanup.

### P1: Sidebar performance

* [x] Open and refresh the sidebar with one global tmux session-list query.
* [x] Filter sessions locally by current project or volatile instance.
* [x] Show only the owning instance's volatile sessions when the active session is volatile.
* [x] Compute the selected session index once per render.
* [x] Remove unconditional session marker synchronization during sidebar refresh.
* [x] Ensure an unchanged refresh performs no per-session tmux writes.
* [x] Keep normal refreshes read-only toward persistent state.
* [x] Add command-count tests for one list query and zero marker writes.

### P1: Project and session behavior

* [x] Close the sidebar immediately after tmux accepts valid session or project creation work, while the short-lived worker completes creation and switching.
* [ ] Update tmux markers only for sessions directly affected by a mutation; do not rewrite unrelated sessions.
* [x] Report background creation failures through the tmux status message.
* [x] Create new project sessions in the project's configured workdir.
* [x] Create volatile sessions in the active pane's working directory.
* [x] When creating a project from a volatile session, promote every volatile session owned by the current tflow instance into the new project.
* [x] Give each promoted session a generated persistent `tflow-p-<id>` identity while preserving its display label and visible order.
* [x] Clear volatile ownership markers from every promoted session.
* [x] Do not create an additional initial session when a project is created through volatile-session promotion.
* [x] Switch directly to the promoted successor of the active volatile session, close the sidebar, and refresh the tmux project and session status indicators.
* [x] Never promote volatile sessions owned by another tflow instance.
* [x] On promotion failure, report the original error without claiming a successful switch or sidebar close.
* [x] Keep session labels unique inside a project.
* [x] Keep volatile labels unique inside their owning instance.
* [x] Allow different projects to reuse the same session label.
* [x] Allow different tflow instances to reuse the same volatile label.
* [ ] Move persistent sessions between projects without changing their tmux session IDs.
* [ ] Reject moves whose labels already exist in the target project.
* [ ] Delete a project when its final session is moved out.
* [x] Delete a project when its final session is deleted.
* [x] Delete all persistent sessions and metadata when a project is deleted.
* [ ] Switch only when the active project is deleted, selecting the first session in the next project.
* [ ] Create a volatile fallback in the active pane's working directory when no project session remains.
* [x] Keep project and session order stable.
* [x] Test volatile-session project promotion, foreign-instance preservation, persistent ID replacement, volatile-marker clearing, active-session switching, sidebar closure, status refresh, and failure handling.
* [ ] Test creation, rename, moves, deletion, switching, active-project deletion, and fallback behavior.

### P1: Generated labels

* [x] Keep generated labels human-readable.
* [x] Ensure generated labels are unique within their visible scope.
* [x] Keep the animal list compiled into the binary.
* [x] Perform no runtime network requests for label generation.
* [x] Keep detailed collision strategy as an implementation detail.
* [x] Test label collisions and fallback generation.

### P1: Issue #55 consistency corrections

* [ ] Preserve user-entered session-label casing and use exact displayed-label uniqueness within each scope.
* [ ] Restore already-renamed volatile sessions and their ownership markers when promotion fails before state persistence; clean up an affected session only when restoration fails.
* [ ] Emit a diagnostic for best-effort cleanup failures while returning the original operation error.
* [ ] Test non-active session and project deletion without client switching, active-project deletion switching, and no-project fallback creation.
* [ ] Test a mid-promotion rename failure leaves no persistent-name orphan or stale ownership marker.
* [ ] Test popup opening from a persistent session cannot inherit a stale instance ID from the tmux server environment.
* [ ] Test fallback working-directory selection uses the active pane rather than the popup or server working directory.
* [ ] Test label case preservation and exact-scope duplicate handling.
* [ ] Add mutation command-count tests proving unrelated tmux session markers are not rewritten.

### P1: Installation and verification

* [ ] Use module path `github.com/rapsnx/tflow`.
* [ ] Move the executable entry point from `cmd/main.go` to `cmd/tflow/main.go`.
* [ ] Verify `go install github.com/rapsnx/tflow/cmd/tflow@latest`.
* [ ] Verify `nix build --no-link .#tflow`.
* [ ] Ensure the Nix package installs `bin/tflow`.
* [ ] Add CI for formatting, `go vet`, and `go test ./...`.
* [ ] Update README installation documentation after the module and entry-point work is complete.
* [x] Align README keybinding and persistence documentation with the implemented behavior.

## Remove obsolete implementation

* [x] Remove project names encoded into tmux session names.
* [x] Remove session labels encoded into tmux session names.
* [x] Remove project-wide tmux rename migration code.
* [x] Remove parallel `SessionProjects` and `SessionLabels` state maps.
* [x] Remove strict unknown-field rejection.
* [x] Remove sidebar-triggered reconciliation writes.
* [x] Remove file and directory sync requirements.
* [x] Remove popup PID ownership requirements.
* [x] Remove generalized rollback and compensation tests.
* [ ] Remove obsolete architecture-specific helpers after their callers are migrated.
* [ ] Remove the empty session-identity file, unreachable in-popup creation paths and messages, and unused startup/state helpers.
* [ ] Remove unused helpers and exported test-only APIs, unreachable theme/key handling, deprecated Lip Gloss style copying, and local `min`/`max` helpers shadowing Go builtins.

## After alpha 0.0.1

* [ ] Add state schema migration only when an actual released schema requires it.
* [ ] Broaden tmux error compatibility only for observed versions and errors.
* [ ] Add benchmarks only when command-count tests or measurements reveal a problem.
* [ ] Add crash-recovery features only when real usage demonstrates a need.
* [ ] Add new session types only after the core terminal-session model is stable.
