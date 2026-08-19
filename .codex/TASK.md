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
* [x] Reject semantically invalid state instead of silently dropping or synthesizing records during normalization.
* [x] Test empty and duplicate normalized project names, empty and duplicate session IDs, empty labels, and duplicate labels within one project.
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
* [x] Emit a diagnostic when releasing a state lock fails while preserving any primary operation error.
* [x] Test lock-release diagnostics in mutation, reconciliation, and background-worker paths.

### P0: Startup reconciliation

* [x] List tmux sessions once during startup.
* [ ] Retain persistent-session and project metadata when tmux sessions are absent at startup, including after a reboot.
* [ ] Remove the obsolete reconciliation cleanup that deletes missing persistent-session metadata and projects solely because they are absent from tmux.
* [ ] Keep marker repair limited to persistent sessions that exist in tmux, without rewriting unrelated sessions or persistent state.
* [ ] Test startup after an empty tmux server preserves projects and ordered persistent-session metadata without writing state.
* [x] Treat an absent dedicated tmux server as an empty session list.
* [x] Do not remove metadata when tmux returns another operational error.
* [x] Restore project and label markers for surviving persistent sessions and clear stale volatile ownership markers.
* [x] Keep marker repair limited to persistent sessions represented by state and avoid rewriting unrelated sessions.
* [x] Add startup marker-repair tests for partially completed creation, promotion, and move operations.
* [x] Do not reconcile or write state during ordinary sidebar refreshes.
* [x] Test explicit empty-project cleanup and tmux-error preservation.

### P0: Operation failure handling

* [x] Kill a newly created tmux session when its metadata cannot be persisted.
* [x] Kill a newly created tmux session when post-creation setup such as window renaming fails.
* [x] Preserve the original setup error and emit a diagnostic when orphan cleanup also fails.
* [x] Test post-creation setup failure leaves no unmarked tmux session behind.
* [x] Treat cleanup of an already missing session or popup as successful.
* [x] Return the original operation error to the user.
* [x] Remove project-wide rename rollback logic.
* [x] Remove generalized compensation or transaction helpers.
* [x] Leave non-critical inconsistencies for the next startup reconciliation.
* [x] Test failed session creation persistence and already-missing cleanup.

### P1: Volatile instance lifecycle

* [x] Keep one instance ID exclusively on the attached tmux client, tracked through a deliberately client-keyed entry when the current session carries no marker of its own; never set, inherit, or consult it through an ambient or unscoped variable.
* [x] Preserve the owning instance when the client switches into a persistent session and pass it explicitly to popups opened there.
* [x] Remove only the detached client's volatile sessions.
* [x] Never remove persistent sessions during instance cleanup.
* [x] Never remove volatile sessions belonging to another instance.
* [x] Keep cleanup idempotent.
* [x] Test multiple simultaneous tflow instances and repeated cleanup.
* [x] Test a popup opened from a persistent session retains the correct client-owned instance and can create a volatile fallback after deleting the final project.

### P1: Graceful signal shutdown

* [x] Cancel the runtime context on SIGHUP, SIGINT, and SIGTERM.
* [x] Pass cancellation only to the attached tmux client and Bubble Tea popup program.
* [x] Ask the attached tmux client to terminate gracefully before forcefully killing it after a bounded wait.
* [x] Clean the owning volatile instance once when the attached client is canceled or exits.
* [x] Keep signal cleanup scoped to the owning instance and preserve persistent and foreign volatile sessions.
* [x] Exit a canceled popup without dispatching the user-facing quit action.
* [x] Test canceled attach cleanup and canceled-popup behavior.
* [x] Test signal cancellation gives the tmux client a graceful termination opportunity before force termination.

### P1: Popup lifecycle

* [x] Let tmux own popup process lifetime.
* [x] Keep only a client-scoped popup-visible marker when needed for toggle behavior.
* [x] Close popups through `tmux display-popup -C`.
* [x] Clear stale popup markers when no popup exists.
* [x] Ignore benign `no popup` and `client not found` cleanup errors.
* [x] Do not store popup PIDs.
* [x] Do not implement child-process reaping or a popup process registry.
* [x] Test popup open, toggle, close, quit, and stale-marker cleanup.
* [x] Route tmux popup cleanup diagnostics through `internal/diag` instead of a separate stderr-only helper.
* [x] Emit a diagnostic when popup closing fails and marker cleanup also fails without replacing the close error.
* [x] Test popup cleanup diagnostics through the shared diagnostic output seam.

### P1: Mouse wheel scrollback

* [x] Enable tmux mouse reporting so the scroll wheel pages through a pane's history via copy-mode.
* [x] Unbind click, drag, double-click, triple-click, and middle/right-click mouse bindings in the root and copy-mode key tables so tmux takes no action on them; native text selection still requires the terminal's selection-override modifier (e.g. Shift in Alacritty) since mouse reporting being on puts the terminal into mouse-tracking mode regardless of tmux's own bindings.
* [x] Test that `EnsureControlMode` issues the mouse-on option and the full set of non-wheel unbind commands.

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
* [x] Update tmux markers only for sessions directly affected by a mutation; do not rewrite unrelated sessions.
* [x] Before a sidebar-initiated switch, detect whether every pane in the outgoing session has exited.
* [x] After a successful sidebar switch, remove only an outgoing session whose every pane had exited and which differs from the switch target; preserve sessions with at least one live pane and never remove the session just switched to, including no-op reselection of the current session.
* [x] Apply dead-session cleanup to direct session selection and project selection, after switching the client to the user-selected target.
* [x] Remove persistent metadata and an empty project when dead-session cleanup removes a persistent session; do not persist volatile-session cleanup.
* [x] Keep the selected target active and report a diagnostic when post-switch cleanup or its persistent-state update fails.
* [x] Report background creation failures through the tmux status message.
* [x] Create new project sessions in the project's configured workdir.
* [ ] Resolve the originating active tmux pane directory before creating a project and pass it explicitly to the creation worker as the project's initial workdir.
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
* [x] Move persistent sessions between projects without changing their tmux session IDs.
* [x] Reject moves whose labels already exist in the target project.
* [x] Delete a project when its final session is moved out.
* [x] Delete a project when its final session is deleted.
* [x] Delete all persistent sessions and metadata when a project is deleted.
* [x] Keep project and session order stable.
* [x] Test volatile-session project promotion, foreign-instance preservation, persistent ID replacement, volatile-marker clearing, active-session switching, sidebar closure, status refresh, and failure handling.
* [x] Test creation, rename, moves, deletion, switching, active-project deletion, fallback behavior, and dead-session cleanup after direct-session and project switches.
* [x] Test dead-session cleanup for all-dead, live, and mixed-pane outgoing sessions; persistent and volatile sources; failed target switches; failed tmux cleanup; failed metadata persistence; and no-op switches where the outgoing session equals the switch target.
* [ ] Show every persisted project and its ordered persistent sessions in the sidebar even when some or all are absent from tmux.
* [ ] Lazily create a missing selected persistent session with its stored internal ID, label, project marker, and project's workdir, then switch the originating client to it without changing persistent state.
* [ ] Lazily create the first stored session when explicitly switching to a project whose sessions are absent from tmux.
* [ ] Test lazy restoration after restart, direct missing-session selection, project selection, correct workdir and marker setup, persistence preservation, and create/switch failure handling.
* [ ] After a successful session move, switch the originating client to the moved session in its target project without changing its tmux identity.
* [ ] Test project creation uses the originating active pane directory and a successful move switches to its moved session.
* [ ] Remove deletion paths that switch into a different persistent project; retain switching to another session in the same project and use a volatile fallback before deleting the final active session or active project.
* [ ] Test non-active deletion leaves the client unchanged, active non-final deletion remains in its project, and final active session/project deletion never selects another project.

### P1: Project settings editor

* [ ] Replace the single-line project-workdir prompt with a temporary YAML document opened in $EDITOR, falling back to nvim, within the existing sidebar popup.
* [ ] Strictly parse the editor document after exit, accept only supported project settings (currently workdir), and persist valid settings through the existing JSON store mutation path.
* [ ] Create the temporary editor document securely, remove it on every exit path, and never add a persistent project-settings file.
* [ ] Test editor selection, valid settings updates, unknown-key and invalid-document rejection without state changes, temporary-file cleanup, and project workdir normalization including home-directory expansion on save.

### P1: Generated labels

* [x] Keep generated labels human-readable.
* [x] Ensure generated labels are unique within their visible scope.
* [x] Keep the animal list compiled into the binary.
* [x] Perform no runtime network requests for label generation.
* [x] Keep detailed collision strategy as an implementation detail.
* [x] Test label collisions and fallback generation.

### P1: Issue #55 consistency corrections

* [x] Preserve user-entered session-label casing and use exact displayed-label uniqueness within each scope.
* [x] Restore already-renamed volatile sessions and their ownership markers when promotion fails before state persistence; clean up an affected session only when restoration fails.
* [x] Emit a diagnostic for best-effort cleanup failures while returning the original operation error.
* [x] Test non-active session and project deletion without client switching, active-project deletion switching, and no-project fallback creation.
* [x] Test a mid-promotion rename failure leaves no persistent-name orphan or stale ownership marker.
* [x] Test popup opening from a persistent session cannot inherit a stale instance ID from the tmux server environment and retains the correct owning instance.
* [x] Test fallback working-directory selection uses the active pane rather than the popup or server working directory.
* [x] Test label case preservation and exact-scope duplicate handling.
* [x] Add mutation command-count tests proving unrelated tmux session markers are not rewritten.

### P1: Review #68 and #69 corrections

* [x] Retry a client-scoped tmux command only after positively identifying a missing-client error.
* [x] Resolve any replacement client within the same tflow instance; never retry against an arbitrary client.
* [x] Test `SwitchClient`, `DisplayMessage`, and `CurrentPaneDir` preserve client ownership on errors and in multi-client servers.
* [x] After a successful move, update the popup model's session label from the state observed by the locked mutation.
* [x] Test a concurrent rename followed by a move updates persisted state, tmux markers, in-memory labels, and the success message consistently.

### P1: Issue #86 remaining review follow-ups

* [x] Stop setting the unscoped instance ID in the long-lived tflow process before tmux startup.
* [x] Pass `TFLOW_INSTANCE_ID` explicitly to every popup, including `TFLOW_INSTANCE_ID=` when resolution is empty.
* [x] Keep the deliberately client-keyed instance slot as the only persistent-session fallback.
* [x] Ensure create-worker invocation receives only the instance explicitly resolved for the originating popup/client.
* [x] Test that a popup opened from a persistent session with a stale server-environment instance ID and no remembered client instance receives an explicit, empty instance flag rather than no flag at all.
* [x] Retain existing multi-instance tests proving a valid client-owned instance is preserved and no foreign instance is selected.
* [x] Remove `projectCreatedMsg` and its handler; project creation is handled entirely by `create-worker`.
* [x] Remove the non-volatile branch of `sessionCreatedMsg`'s handler; its only production producer (`createVolatileFallback`) always sets it volatile.
* [x] Remove the never-produced `err` fields from `menuActionMsg` and `projectRenamedMsg`.
* [x] Unexport `ui.NewMenu` to `ui.newMenu`; it has no caller outside the package's own tests.
* [x] Keep `tmux.ContainsAnimalName` exported: `internal/ui` tests call it across the package boundary to check generated labels against the unexported animal-name list, which only this exported function can provide without duplicating that list.

### P1: Installation and verification

* [x] Use module path `github.com/rapsnx/tflow`.
* [x] Move the executable entry point from `cmd/main.go` to `cmd/tflow/main.go`.
* [ ] Verify go install github.com/rapsnx/tflow/cmd/tflow@latest and tflow version against the current published release through the module proxy.
* [x] Verify `nix build --no-link .#tflow`.
* [x] Ensure the Nix package installs `bin/tflow`.
* [x] Add CI for formatting, `go vet`, and `go test ./...`.
* [x] Restrict push-triggered CI to `main` so pull-request branches do not run the same workflow twice.
* [x] Update README installation documentation after the module and entry-point work is complete.
* [x] Add the `m` session-move keybinding to README and align the Go version badge with `go.mod`.
* [x] Align README keybinding and persistence documentation with the implemented behavior.
* [x] Update `AGENTS.md` entry-point and local-run instructions to `cmd/tflow`.

### P1: CLI versioning and releases

* [x] Add public `tflow version` and `tflow --version` commands.
* [x] Add concise public help for `tflow help`, `tflow -h`, and `tflow --help`.
* [x] Keep internal tmux worker commands out of the public help text.
* [x] Resolve release versions from linker-injected tags and Go module build metadata, with a development-build fallback.
* [x] Align Nix development-build metadata with the executable version output.
* [x] Add GoReleaser configuration for Linux and macOS amd64/arm64 release archives and SHA-256 checksums.
* [x] Validate release configuration and snapshot artifacts in CI.
* [x] Add a tag-triggered GitHub Release workflow restricted to tags that point to `main`.
* [x] Publish v0.1.0-alpha.1 after its branch is merged into main.
* [x] Add a push-to-`main`-triggered Release Drafter workflow that maintains one draft release, updating its changelog body on every push and resolving its proposed alpha version from the latest *published* release; the proposed version stays fixed across multiple pushes and only advances to the next alpha once the current draft is published, without creating a git tag until then.

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
* [x] Remove obsolete architecture-specific helpers after their callers are migrated.
* [x] Remove the empty session-identity file, unreachable in-popup creation modes and messages, unused startup/state helpers, and the dead in-popup `Ctrl+F` branch.
* [x] Remove unused helpers and exported test-only APIs, unreachable theme handling, deprecated Lip Gloss style copying, and local `min`/`max` helpers shadowing Go builtins.

## After alpha 0.0.1

* [ ] Add state schema migration only when an actual released schema requires it.
* [ ] Broaden tmux error compatibility only for observed versions and errors.
* [ ] Add benchmarks only when command-count tests or measurements reveal a problem.
* [ ] Add crash-recovery features only when real usage demonstrates a need.
* [ ] Add new session types only after the core terminal-session model is stable.
