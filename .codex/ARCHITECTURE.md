# tflow architecture

`tflow` is a small terminal session manager built on top of `tmux`.

The design is intentionally simple:

* the active terminal is a normal tmux session
* the sidebar is a tmux popup
* persistent metadata lives in one JSON file
* tmux remains the source of truth for running sessions
* no background daemon or terminal emulation layer is used

## Runtime model

`tflow` uses one dedicated tmux socket.

Each managed terminal is an ordinary tmux session. Starting `tflow` creates a volatile session and attaches a tmux client to it.

Persistent sessions belong to projects. Volatile sessions belong to the current tflow instance and are removed when that instance exits or its client detaches.

Cleanup must only affect volatile sessions owned by the relevant instance. Persistent sessions and sessions owned by other instances remain untouched.

An instance ID is scoped to its attached tmux client and passed explicitly to that client's popup processes. The owning instance remains associated with the client when it switches into a persistent session, tracked through a deliberately client-keyed entry rather than a session marker, since persistent sessions carry none. Popups opened from persistent sessions must receive that same client-scoped instance ID explicitly, recovered from that entry when the current session carries no marker of its own. tflow must not set, inherit, or consult an instance ID through an ambient or unscoped variable, such as a single shared key or a value read without confirming it belongs to the current client.

Client-scoped operations must target the originating tmux client. If that client no longer exists, tflow may select a replacement only when it can prove that the replacement belongs to the same tflow instance. It must not fall back to an arbitrary client.

Managed panes use tmux `remain-on-exit`. tflow does not automatically respawn exited shells or switch the client to another session.

Before executing an explicit sidebar-initiated switch, tflow determines whether every pane in the outgoing session has exited. If the target switch succeeds, the outgoing session differs from the switch target, and every outgoing pane was exited, tflow removes only that outgoing session. This applies to direct session selection and project selection, which switches to the selected project's first session. Sessions with one or more live panes remain intact, and tflow never removes the session it just switched to, including when a switch resolves to the session that was already current. tflow does not monitor for exited panes or remove sessions outside an explicit sidebar switch.

### Graceful signal shutdown

The tflow executable creates one runtime context that is canceled by SIGHUP, SIGINT, or SIGTERM. Cancellation is passed only to long-running runtime boundaries: the attached tmux client and the Bubble Tea popup program.

After startup has created and tagged a volatile session, the owning tflow process always performs one best-effort cleanup of that instance before it exits. Signal cancellation first asks the attached tmux client to terminate gracefully and allows a bounded wait before forcefully terminating it, then follows that same cleanup path. Cleanup remains strictly ownership-scoped: it never removes persistent sessions or volatile sessions owned by another instance.

A canceled popup exits its Bubble Tea program without invoking the user-facing quit action. Its existing shell wrapper then clears the client-scoped popup marker. tflow continues to let tmux own popup process lifetime and does not add popup PID tracking or a process supervisor.

Signal-driven cleanup does not replace another operation error. If cleanup is the only failure, tflow reports it. Crash recovery, forced termination such as SIGKILL, machine crashes, and power loss remain outside the design.

## Session identity

Tmux session names are internal identifiers and are not used as user-facing project or session names.

Persistent sessions use generated identifiers such as:

```text
tflow-p-8f42ac91
```

Volatile sessions include their owning instance:

```text
tflow-v-<instance-id>-8f42ac91
```

Project names and session labels are stored as metadata. Renaming a project or session therefore does not require renaming tmux sessions.

Generated display labels should be readable and unique within their visible scope. User-entered session labels preserve their casing and must be unique by their exact displayed value within their project or owning volatile instance.

## Terminal UI

The user works directly inside the active tmux session.

`Ctrl+F` toggles a sidebar popup for managing sessions and projects. Successful actions close the popup and return focus to the active terminal.

`Ctrl+Q` opens confirmation for quitting the current tflow instance and removing its volatile sessions.

Tmux owns popup process lifetime. tflow does not track popup PIDs or implement a separate process supervisor. Valid session and project creation submissions close the popup after tmux accepts a short-lived background worker; the current terminal remains usable until that worker switches to the completed target.

The sidebar displays persistent sessions belonging to the current project. When the active session is volatile, it displays only volatile sessions owned by the current tflow instance.

For a persistent project, the sidebar uses the ordered session records in
persistent state as its complete session list, whether or not those tmux
sessions are currently running. A missing persistent session is materialized
lazily only when the user selects it. Selecting a project resolves its first
stored session and materializes that session if needed; selecting another
stored session materializes that selected session if needed. Materialization
creates the stored internal tmux session ID in the project's working
directory, restores its project and label markers, and then switches the
originating client to it. It does not mutate persistent state.

Tmux prefix followed by `h` and `l` switches to the previous and next
session, respectively. Navigation wraps within the same context and uses the
same order as the sidebar: persistent-session order for the active project,
or tmux list order for volatile sessions owned by the current instance. It
never crosses projects or tflow instances. A missing persistent target is
materialized lazily as for sidebar selection. Navigation is client-scoped and
does not perform sidebar-only exited-session cleanup.

The top status bar shows the previous, active, and next contextual sessions;
when there is only one, it shows the active session alone. tflow maintains
only the selected target's session-scoped, derived status metadata. That
metadata is not persistent state and does not require a daemon or a refresh
loop.

Every session has a visual type chip. Terminal sessions use blue `>_ CODE`,
git sessions use teal `⎇ GIT`, and agent sessions use yellow `✦ AGENT`.
The same chips appear in the sidebar and the top bar. A selected row retains
its type chip, so selection color never replaces type identity. The teal
`live` state and the red attention state are separate indicators and may
appear with any type chip.

tflow enables tmux mouse reporting only for wheel-scroll: the wheel pages a pane's history via copy-mode like a normal terminal's scrollback, while every other mouse interaction (click, drag, double/triple-click, middle/right-click) is unbound in the root and copy-mode key tables. Because mouse reporting being on at all puts the terminal into mouse-tracking mode, text selection still requires the terminal's own override modifier (e.g. Shift in Alacritty).

## Projects and sessions

A project contains:

* a unique name
* a default working directory
* an optional agent-binary default
* an ordered list of persistent sessions

Creating a project uses the originating active tmux pane's current directory
as its initial working directory. That directory is resolved for the
originating client before the short-lived creation worker starts and is passed
to that worker explicitly.

Projects and their ordered persistent-session metadata remain available even
when none of their sessions currently exists in tmux, such as after a reboot.
Missing persistent sessions are recreated only through the explicit selection
described above. A project is removed only by an explicit project deletion or
when an explicit session deletion or move removes its final persistent session.

A persistent session contains:

* an internal tmux session ID
* a user-facing label
* a type: `terminal`, `git`, or `agent`
* the agent executable captured for an agent session

Records written before session types existed are terminal sessions. Reading
such records does not require a store migration or a state rewrite.

An ordinary newly created project receives two lazy persistent records in this
order: a terminal session labeled `code` and a git session labeled `git`.
The git session runs `lazygit` in the project workdir when it is materialized.
The existing `n` action creates normal terminal sessions only. Existing
projects and projects created by volatile-session promotion retain their
current sessions and receive no presets.

The project settings editor may set `agent-binary` to one executable name or
absolute path, with no arguments. Saving a non-empty value adds a lazy agent
session labeled `agent` when the project has none. Updating the setting updates
the captured executable for that agent session; a currently running agent
session continues unchanged. Clearing the setting retains any existing agent
session and its captured executable, but prevents future automatic agent
provisioning. A missing `lazygit` or agent executable is a clear,
non-mutating materialization error.

New project sessions start in the project's working directory. Every volatile session, including a fallback created after project deletion, starts in the active pane's working directory.

Creating a project while the active session is volatile promotes every volatile session owned by the current tflow instance into the new project instead of creating an additional initial session. Each promoted session receives a generated persistent `tflow-p-<id>` identifier while retaining its display label and visible order, and clears its volatile ownership markers. Volatile sessions owned by other tflow instances remain untouched.

After a successful promotion, tflow switches the client directly to the promoted successor of the previously active volatile session. The sidebar closes and the normal tmux project and session status indicators immediately show the new project and active session. If promotion fails before the project state is persisted, tflow restores any already-renamed sessions to their original volatile identities and ownership markers; if restoration fails, it performs direct best-effort cleanup of the affected session. It reports the original error and does not claim a successful switch or sidebar close.

Session labels must be unique inside their project. Volatile labels must be unique inside their owning instance.

Moving a persistent session to another project preserves its tmux session. A move fails when the target project already has the session label. Moving the final session out of a project deletes that project. After a successful move, tflow switches the originating client to the moved session in its target project.

Deleting the final session of a project also deletes the project. Deleting a project removes its persistent sessions and metadata. Deletion must never switch the client into another persistent project: deleting a non-active session or project leaves the client unchanged; deleting the active session selects another session from the same project when one remains; and deleting the final active session or active project creates and switches to a volatile fallback in the active pane's working directory before removal. Project creation, explicit project switching, and session moves retain their existing switching behavior.

When sidebar-switch cleanup removes an exited persistent session, it also removes that session's metadata and removes its project when it becomes empty. Removing an exited volatile session does not change persistent state. Sidebar-switch cleanup always uses the target selected by the user; it does not choose a replacement session or create a fallback.

tflow enables tmux activity monitoring for every managed session. Tmux alert
hooks set a session-scoped, runtime-only attention indicator when output
arrives for a session that is not being visited. Any client visit clears that
indicator. Attention is visible beside the session in the sidebar and top bar,
is never stored in JSON, and is lost when tmux restarts.

## Persistent state

Persistent state is stored at:

* `$XDG_STATE_HOME/tflow/store.json` when `XDG_STATE_HOME` is set and non-empty
* `~/.local/state/tflow/store.json` otherwise

The store contains only project and persistent-session metadata. Volatile ownership and other runtime information are not persisted.

Conceptually, the state contains:

```json
{
  "projects": [
    {
      "name": "example",
      "workdir": "/home/user/example",
      "agentBinary": "codex",
      "sessions": [
        {
          "id": "tflow-p-8f42ac91",
          "label": "code",
          "type": "terminal"
        },
        {
          "id": "tflow-p-96ad4c10",
          "label": "agent",
          "type": "agent",
          "command": "codex"
        }
      ]
    }
  ]
}
```

`agentBinary` and an agent session's `command` are optional. A session `type`
is optional only for records written before typed sessions; when present it
must be `terminal`, `git`, or `agent`. Agent sessions require a non-empty
executable command and non-agent sessions must not carry one. Malformed or
semantically invalid state causes startup to fail with a clear path-qualified
error and is not silently normalized or rewritten. Invalid state includes
empty or duplicate normalized project names, empty or duplicate session IDs,
empty session labels, duplicate labels within one project, and duplicate agent
sessions within one project. Unknown JSON fields may be ignored.

## State updates

State mutations use one advisory lock to prevent multiple tflow processes from overwriting each other's changes.

A mutation performs:

1. acquire the lock
2. load the latest state
3. apply the change
4. encode the complete state as JSON
5. write it to a temporary file in the state directory
6. set file mode `0600`
7. close the temporary file
8. rename it over `store.json`
9. release the lock

File and directory `fsync` are intentionally not required.

The rename prevents readers from seeing partially written JSON. Sudden power-loss durability is outside the design goals.

## Reconciliation

Startup performs one reconciliation against the current tmux session list.

Metadata for persistent sessions that no longer exist in tmux is retained for
lazy materialization. Startup never removes a project or persistent-session
record merely because its tmux session is absent. Explicit deletion and move
operations remain responsible for removing their own metadata.

For persistent sessions that still exist, startup restores their project and label markers from persistent state and clears stale volatile ownership markers. This repair is limited to sessions represented by persistent state and does not rewrite unrelated sessions.

Normal sidebar refreshes do not modify persistent state. They list tmux sessions once and filter the result locally.

If tmux cannot provide a session list because of an operational error, no metadata is removed or repaired.

## Project settings editor

`e` temporarily replaces the sidebar UI with a YAML project-settings document
in `$EDITOR`, falling back to `nvim` when `$EDITOR` is unset. The document is a
temporary internal file, not a persistent user-edited configuration file. When
the editor exits, tflow removes the temporary file, strictly validates its
supported settings (`workdir` and `agent-binary`), rejects unknown keys, and
persists valid settings through the existing JSON store mutation path. Editor,
validation, or persistence failures leave state unchanged and are reported
when the sidebar resumes.

## Error handling

tflow reports the original operation error and avoids generalized transaction or rollback frameworks.

Simple local cleanup is allowed:

* kill a newly created tmux session when its setup or metadata persistence fails
* ignore cleanup requests for resources that are already gone
* correct other inconsistencies during the next startup reconciliation

Best-effort cleanup failures emit a diagnostic without replacing the original operation error.

If a sidebar target switch fails, tflow does not remove the outgoing session. After a successful switch, failed cleanup leaves the client on its selected target and emits a diagnostic. If tmux cannot remove the exited source session, its persistent metadata remains unchanged. If tmux removes a persistent source session but its metadata update fails, tflow reports the error and retains the metadata for later lazy materialization.

The application does not attempt to guarantee consistency after process crashes, machine crashes, or power loss.

## Performance rules

Opening or refreshing the sidebar performs one tmux session-list query.

Unchanged refreshes must not perform per-session tmux writes.

Project and session metadata are changed only by explicit user operations or startup reconciliation.

An explicit operation updates tmux markers only for sessions it directly creates, promotes, renames, moves, or deletes; it does not rewrite markers for unrelated sessions. A switch may refresh derived status metadata for its selected target only.

Performance optimizations should be based on tmux command count or measurements rather than speculative abstractions.

## Dependencies

* tmux provides session management, popups, key bindings, and status output
* Bubble Tea provides sidebar interaction
* Lip Gloss provides terminal styling

Implementation should prefer direct, testable code over additional lifecycle, persistence, or recovery frameworks.

## CLI and releases

The public command surface is deliberately small:

* `tflow` starts a new tflow instance
* `tflow version` and `tflow --version` print the build version
* `tflow help`, `tflow -h`, and `tflow --help` describe the public commands

Internal tmux worker commands remain supported for tflow's own use but are not
part of the public help text.

Release versions use Semantic Versioning Git tags with a `v` prefix. Tagged
release builds report that exact tag. Module installs use embedded Go build
metadata when it identifies the requested module version. Other builds report a
development version, optionally including their Nix source revision.

Pushing a `v*` tag that points to `main` creates a GitHub Release containing
checksummed `tar.gz` archives for Linux and macOS on amd64 and arm64. Nix and
Home Manager remain source-based; no additional package feed is managed.
