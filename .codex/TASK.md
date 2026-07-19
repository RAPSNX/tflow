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

* [ ] Serialize mutations with one advisory store lock.
* [ ] Reload the latest state while holding the lock.
* [ ] Apply the requested mutation to the reloaded state.
* [ ] Write the complete JSON document to a same-directory temporary file.
* [ ] Close the temporary file before renaming it over `store.json`.
* [ ] Do not use file `fsync`.
* [ ] Do not use directory `fsync`.
* [ ] Remove the temporary file when writing or renaming fails.
* [ ] Test that failed writes leave the previous JSON file unchanged.
* [ ] Test that concurrent disjoint mutations do not overwrite each other.

### P0: Startup reconciliation

* [ ] List tmux sessions once during startup.
* [ ] Remove metadata for persistent sessions that no longer exist.
* [ ] Remove projects that have no remaining sessions.
* [ ] Persist reconciled state only when it changed.
* [ ] Treat an absent dedicated tmux server as an empty session list.
* [ ] Do not remove metadata when tmux returns another operational error.
* [ ] Do not retain missing persistent-session metadata for lazy restoration.
* [ ] Do not reconcile or write state during ordinary sidebar refreshes.
* [ ] Test missing-session cleanup without lazy restoration, empty-project cleanup, and tmux-error preservation.

### P0: Operation failure handling

* [ ] Kill a newly created tmux session when its metadata cannot be persisted.
* [ ] Treat cleanup of an already missing session or popup as successful.
* [ ] Return the original operation error to the user.
* [ ] Remove project-wide rename rollback logic.
* [ ] Remove generalized compensation or transaction helpers.
* [ ] Leave non-critical inconsistencies for the next startup reconciliation.
* [ ] Test failed session creation persistence and already-missing cleanup.

### P1: Volatile instance lifecycle

* [ ] Keep one instance ID on the attached tmux client.
* [ ] Preserve the instance ID when the client switches between sessions.
* [ ] Remove only the detached client's volatile sessions.
* [ ] Never remove persistent sessions during instance cleanup.
* [ ] Never remove volatile sessions belonging to another instance.
* [ ] Keep cleanup idempotent.
* [ ] Test multiple simultaneous tflow instances and repeated cleanup.

### P1: Popup lifecycle

* [ ] Let tmux own popup process lifetime.
* [ ] Keep only a client-scoped popup-visible marker when needed for toggle behavior.
* [ ] Close popups through `tmux display-popup -C`.
* [ ] Clear stale popup markers when no popup exists.
* [ ] Ignore benign `no popup` and `client not found` cleanup errors.
* [ ] Do not store popup PIDs.
* [ ] Do not implement child-process reaping or a popup process registry.
* [ ] Test popup open, toggle, close, quit, and stale-marker cleanup.

### P1: Sidebar performance

* [x] Open and refresh the sidebar with one global tmux session-list query.
* [x] Filter sessions locally by current project or volatile instance.
* [x] Show only the owning instance's volatile sessions when the active session is volatile.
* [x] Compute the selected session index once per render.
* [x] Remove unconditional session marker synchronization.
* [x] Ensure an unchanged refresh performs no per-session tmux writes.
* [x] Keep normal refreshes read-only toward persistent state.
* [x] Add command-count tests for one list query and zero marker writes.

### P1: Project and session behavior

* [x] Close the sidebar immediately after tmux accepts valid session or project creation work, while the short-lived worker completes creation and switching.
* [x] Update only the created or promoted session's tmux markers; do not rewrite unrelated sessions during creation.
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
* [ ] Keep volatile labels unique inside their owning instance.
* [ ] Allow different projects to reuse the same session label.
* [ ] Allow different tflow instances to reuse the same volatile label.
* [ ] Move persistent sessions between projects without changing their tmux session IDs.
* [ ] Reject moves whose labels already exist in the target project.
* [ ] Delete a project when its final session is moved out.
* [ ] Delete a project when its final session is deleted.
* [ ] Delete all persistent sessions and metadata when a project is deleted.
* [ ] Switch from an active deleted project to the first session of the next project.
* [ ] Create a volatile fallback when no project session remains.
* [x] Keep project and session order stable.
* [x] Test volatile-session project promotion, foreign-instance preservation, persistent ID replacement, volatile-marker clearing, active-session switching, sidebar closure, status refresh, and failure handling.
* [ ] Test creation, rename, moves, deletion, switching, active-project deletion, and fallback behavior.

### P1: Generated labels

* [ ] Keep generated labels human-readable.
* [ ] Ensure generated labels are unique within their visible scope.
* [ ] Keep the animal list compiled into the binary.
* [ ] Perform no runtime network requests for label generation.
* [ ] Keep detailed collision strategy as an implementation detail.
* [ ] Test label collisions and fallback generation.

### P1: Installation and verification

* [ ] Use module path `github.com/rapsnx/tflow`.
* [ ] Move the executable entry point from `cmd/main.go` to `cmd/tflow/main.go`.
* [ ] Verify `go install github.com/rapsnx/tflow/cmd/tflow@latest`.
* [ ] Verify `nix build --no-link .#tflow`.
* [ ] Ensure the Nix package installs `bin/tflow`.
* [ ] Add CI for formatting, `go vet`, and `go test ./...`.
* [ ] Update README installation, keybinding, and persistence documentation.
* [ ] Remove the obsolete README claim that persistent sessions use lazy restoration.

## Remove obsolete implementation

* [x] Remove project names encoded into tmux session names.
* [x] Remove session labels encoded into tmux session names.
* [x] Remove project-wide tmux rename migration code.
* [x] Remove parallel `SessionProjects` and `SessionLabels` state maps.
* [x] Remove strict unknown-field rejection.
* [ ] Remove sidebar-triggered reconciliation writes.
* [ ] Remove file and directory sync requirements.
* [ ] Remove popup PID ownership requirements.
* [ ] Remove generalized rollback and compensation tests.
* [ ] Remove obsolete architecture-specific helpers after their callers are migrated.

## After alpha 0.0.1

* [ ] Add state schema migration only when an actual released schema requires it.
* [ ] Broaden tmux error compatibility only for observed versions and errors.
* [ ] Add benchmarks only when command-count tests or measurements reveal a problem.
* [ ] Add crash-recovery features only when real usage demonstrates a need.
* [ ] Add new session types only after the core terminal-session model is stable.
