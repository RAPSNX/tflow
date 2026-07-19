# tflow implementation checklist

This checklist is derived from `.codex/ARCHITECTURE.md`. Work is ordered by priority for the first usable release.

## Alpha 0.0.1

### P0: Session identity

* [ ] Use generated internal tmux session IDs instead of project and display names.
* [ ] Use `tflow-p-<id>` for persistent sessions.
* [ ] Use `tflow-v-<instance-id>-<id>` for volatile sessions.
* [ ] Store project names and session labels only as metadata.
* [ ] Stop renaming tmux sessions when a project or display label is renamed.
* [ ] Ensure persistent IDs are globally unique.
* [ ] Ensure volatile IDs are unique across independently started tflow instances.
* [ ] Test project and session renames without tmux session renames.

### P0: Persistent state model

* [ ] Replace parallel session maps with projects containing ordered session records.
* [ ] Store only:

  * project name
  * project workdir
  * ordered persistent sessions
  * internal tmux session ID
  * session display label
* [ ] Keep volatile sessions and instance ownership out of the store.
* [ ] Ignore unknown JSON fields.
* [ ] Reject malformed JSON with a clear path-qualified error.
* [ ] Keep the state path at `$XDG_STATE_HOME/tflow/store.json`, falling back to `~/.local/state/tflow/store.json`.
* [ ] Set the state directory mode to `0700`.
* [ ] Set the state file mode to `0600`.
* [ ] Update state codec and normalization tests for the new schema.

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
* [ ] Do not reconcile or write state during ordinary sidebar refreshes.
* [ ] Test missing-session cleanup, empty-project cleanup, and tmux-error preservation.

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

* [ ] Open and refresh the sidebar with one global tmux session-list query.
* [ ] Filter sessions locally by current project or volatile instance.
* [ ] Compute the selected session index once per render.
* [ ] Remove unconditional session marker synchronization.
* [ ] Ensure an unchanged refresh performs no per-session tmux writes.
* [ ] Keep normal refreshes read-only toward persistent state.
* [ ] Add command-count tests for one list query and zero marker writes.

### P1: Project and session behavior

* [ ] Create new project sessions in the project's configured workdir.
* [ ] Create volatile sessions in the current working directory.
* [ ] Keep session labels unique inside a project.
* [ ] Keep volatile labels unique inside their owning instance.
* [ ] Allow different projects to reuse the same session label.
* [ ] Allow different tflow instances to reuse the same volatile label.
* [ ] Delete a project when its final session is deleted.
* [ ] Create a volatile fallback when no project session remains.
* [ ] Keep project and session order stable.
* [ ] Test creation, rename, deletion, switching, and fallback behavior.

### P1: Generated labels

* [ ] Keep generated labels human-readable.
* [ ] Ensure generated labels are unique within their visible scope.
* [ ] Keep the animal list compiled into the binary.
* [ ] Perform no runtime network requests for label generation.
* [ ] Keep detailed collision strategy as an implementation detail.
* [ ] Test label collisions and fallback generation.

### P1: Installation and verification

* [ ] Use module path `github.com/rapsnx/tflow`.
* [ ] Keep the executable entry point at `cmd/tflow`.
* [ ] Verify `go install github.com/rapsnx/tflow/cmd/tflow@latest`.
* [ ] Verify `nix build --no-link .#tflow`.
* [ ] Ensure the Nix package installs `bin/tflow`.
* [ ] Add CI for formatting, `go vet`, and `go test ./...`.
* [ ] Update README installation and keybinding documentation.

## Remove obsolete implementation

* [ ] Remove project names encoded into tmux session names.
* [ ] Remove session labels encoded into tmux session names.
* [ ] Remove project-wide tmux rename migration code.
* [ ] Remove parallel `SessionProjects` and `SessionLabels` state maps.
* [ ] Remove strict unknown-field rejection.
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

