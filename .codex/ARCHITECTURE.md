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

Managed panes use tmux `remain-on-exit`. tflow does not automatically respawn exited shells or switch the client to another session.

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

Generated display labels should be readable and unique within their visible scope.

## Terminal UI

The user works directly inside the active tmux session.

`Ctrl+F` toggles a sidebar popup for managing sessions and projects. Successful actions close the popup and return focus to the active terminal.

`Ctrl+Q` opens confirmation for quitting the current tflow instance and removing its volatile sessions.

Tmux owns popup process lifetime. tflow does not track popup PIDs or implement a separate process supervisor.

The sidebar displays only sessions belonging to the current project or volatile instance.

## Projects and sessions

A project contains:

* a unique name
* a default working directory
* an ordered list of persistent sessions

A persistent session contains:

* an internal tmux session ID
* a user-facing label

New project sessions start in the project's working directory. New volatile sessions start in the current working directory.

Session labels must be unique inside their project. Volatile labels must be unique inside their owning instance.

Deleting the final session of a project also deletes the project. If no project session remains available, tflow creates a volatile fallback session.

## Persistent state

Persistent state is stored at:

* `$XDG_STATE_HOME/tflow/store.json` when `XDG_STATE_HOME` is set
* `~/.local/state/tflow/store.json` otherwise

The store contains only project and persistent-session metadata. Volatile ownership and other runtime information are not persisted.

Conceptually, the state contains:

```json
{
  "projects": [
    {
      "name": "example",
      "workdir": "/home/user/example",
      "sessions": [
        {
          "id": "tflow-p-8f42ac91",
          "label": "otter"
        }
      ]
    }
  ]
}
```

Malformed state causes startup to fail with a clear error. Unknown JSON fields may be ignored.

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

Metadata for persistent sessions that no longer exist in tmux is removed. Projects without sessions are then removed.

Normal sidebar refreshes do not modify persistent state. They list tmux sessions once and filter the result locally.

If tmux cannot provide a session list because of an operational error, no metadata is removed.

## Error handling

tflow reports the original operation error and avoids generalized transaction or rollback frameworks.

Simple local cleanup is allowed:

* kill a newly created tmux session when its metadata cannot be persisted
* ignore cleanup requests for resources that are already gone
* correct other inconsistencies during the next startup reconciliation

The application does not attempt to guarantee consistency after process crashes, machine crashes, or power loss.

## Performance rules

Opening or refreshing the sidebar performs one tmux session-list query.

Unchanged refreshes must not perform per-session tmux writes.

Project and session metadata are changed only by explicit user operations or startup reconciliation.

Performance optimizations should be based on tmux command count or measurements rather than speculative abstractions.

## Dependencies

* tmux provides session management, popups, key bindings, and status output
* Bubble Tea provides sidebar interaction
* Lip Gloss provides terminal styling

Implementation should prefer direct, testable code over additional lifecycle, persistence, or recovery frameworks.

