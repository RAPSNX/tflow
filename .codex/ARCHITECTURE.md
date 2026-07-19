# tflow

`tflow` is a terminal session manager built on top of `tmux`.

The design stays tmux-native and deliberately small:

- the active terminal is a real `tmux` terminal
- the sidebar is a real `tmux` popup
- the top status shows the current project and session
- persistent metadata lives in one state file

`tflow` does not capture terminal output, replay VT state, run a background garbage-collection daemon, or wrap the live terminal in an outer TUI.

## Runtime model

`tflow` owns one dedicated `tmux` socket. Each managed session is an ordinary `tmux` session.

Starting `tflow` creates a volatile session and attaches one tmux client. That client receives a collision-resistant tflow instance ID. The marker stays on the client when it switches between volatile and persistent sessions so popup actions, fallback session creation, and cleanup can always resolve the owning instance.

Volatile sessions belong to one tflow instance. Their internal tmux identifiers include the instance ID so independently started tflow instances can use the same visible label without collision. The instance component is never displayed: the sidebar and top status show only the session label.

Persistent sessions are ordinary tmux sessions grouped into projects by tflow metadata. Their internal identifiers may also differ from their display labels so different projects can reuse a label.

Managed panes use tmux `remain-on-exit`. When a shell exits, the pane remains visibly exited instead of allowing tmux to move the client into another session. tflow does not automatically switch or respawn it. The user can delete it through the sidebar or quit the instance.

### Instance cleanup

Volatile sessions are removed when their owning client detaches, including normal exit, terminal closure, or confirmed `Ctrl+Q`. Persistent sessions and volatile sessions owned by other instances are never removed by that cleanup.

Cleanup is idempotent. Confirmed quit, the attaching process returning, and the tmux client-detach hook may all request the same cleanup safely.

### Popup cleanup

Each sidebar popup belongs to one tmux client. A client-scoped tmux environment record stores the PID of that popup's wrapper process.

The wrapper starts the menu as its child. On normal close, toggle, quit, detach, startup failure, or a termination signal, it terminates and reaps the menu child before clearing its record. The controller may clear a record only when the client or recorded process no longer exists. It must not affect another client's live popup.

This is process ownership, not a general process manager: no background reaper, registry, or periodic garbage-collection worker is required.

### Animal labels

`tflow` contains a reviewed, compiled list of exactly 25 animal names. The list is fetched once during development and is never requested at runtime.

The startup volatile session and the initial session of a newly created project receive a randomly selected animal display label. When all single-animal labels are already used in the relevant namespace, tflow uses an unused two-animal label and adds a numeric suffix only after all combinations are used.

For volatile sessions, uniqueness is scoped to the visible labels of the owning instance; the hidden internal tmux ID provides global uniqueness across instances.

## Terminal UI

On startup, the user sees a normal live terminal session.

- `Ctrl+F` toggles a slim sidebar popup anchored to the left edge of the active client.
- Opening or closing the sidebar does not resize or move the live terminal.
- Successful sidebar actions close the popup and restore focus to the active terminal.
- Benign popup-close errors are not shown to the user.
- `Ctrl+Q` opens quit confirmation from the live terminal, even when the sidebar is closed.

The tmux top status shows `project` and `session` badges. The project value is empty in volatile mode. Status markers are updated when creation, rename, migration, switching, or reconciliation changes metadata. A volatile rename explicitly updates its session-label marker.

The sidebar contains:

- a centered `TFLOW` badge
- the current context's session list
- optional inline shortcut help below the list, separated by a gap
- a conditional status row at the bottom

Help is non-blocking. `?` toggles it without changing the current list or mode. The next recognized shortcut performs its normal action and automatically hides help. `Esc` hides help first.

Opening or refreshing the sidebar performs one authoritative session-list query against the dedicated tmux socket, filters the result locally to the active instance or project, computes selection once, and renders it. An unchanged refresh performs no per-session tmux writes. Marker repair happens only for actual metadata changes or required reconciliation, never merely because the sidebar opened.

Recoverable user-action problems are yellow warnings. Failed tmux, store, or other operations are red errors. The status row is hidden when there is no feedback.

## Visual design

The sidebar uses Bubble Tea for interaction and Lip Gloss for terminal-safe text, color, padding, and borders.

- `TFLOW` is a centered blue badge with a dark foreground.
- `live` is a green badge immediately before the actually active session label.
- The active row has one continuous selected background across its indentation, badge, spacing, and label.
- `live` appears exactly once and remains legible when its row is selected.

Every input, settings, and confirmation dialog uses one centered card:

- a subtle rounded outer border
- an accent badge and concise title, followed by one horizontal divider
- no accidental inner right or bottom border
- context only when it is necessary to make the decision
- a focused input without a `project:` or `session:` prefix
- a centered, single-line footer containing only separated `Enter` and `Esc` keycaps

Create, rename, settings, and normal confirmations use blue and teal accents. Destructive confirmations use a red header badge and red `Enter` keycap; `Esc` remains neutral. Confirmation copy is direct, for example: `Remove all instances and quit?`

## Projects and sessions

A project contains a unique name and a default `workdir`. A session belongs to at most one project.

Creating a project stores the current working directory as its default `workdir` and creates an initial session there with a random animal label. It does not display unrelated information about the current project and does not change the active sidebar context until the user switches to it.

New project sessions start in the project's `workdir`. New volatile sessions start in the current working directory and belong to the current tflow instance.

Project session labels are unique within a project. Volatile session labels are unique within their owning instance. Different projects and independently started tflow instances may reuse the same visible label.

Switching projects opens a focused search dialog. `Up` and `Down` select a match and `Enter` activates the selected project's first session. Switching from volatile mode requires confirmation; switching between projects is direct.

Deleting the final session of a project requires confirmation that the project will also be deleted. After deletion, tflow activates the first session of the next project in project order, wrapping when necessary. If no project remains, it creates and activates a volatile session.

Project settings contain only the project `workdir`.

## State

There is no user-edited tflow configuration file or per-project YAML.

Persistent metadata lives at:

- `$XDG_STATE_HOME/tflow/store.json` when `XDG_STATE_HOME` is set and non-empty
- `~/.local/state/tflow/store.json` otherwise

The canonical JSON store contains only:

- project order
- project `workdir` values
- project membership for sessions
- display labels for project sessions

Volatile session ownership is runtime state and is not persisted. Packaging integrations such as Home Manager must not render or manage `store.json`.

Unknown or obsolete fields make the store invalid and startup fails with a path-qualified error. A missing store is created empty when needed.

### Safe updates

All mutations use one locked read-modify-write operation:

1. acquire the store lock
2. load the latest store
3. apply the requested mutation
4. write a complete temporary file in the same directory
5. set mode `0600`, flush the file, rename it over `store.json`, and flush the directory
6. release the lock

Reloading while holding the lock preserves changes made by another tflow instance without a custom merge algorithm. Serial order decides concurrent changes to the same resource. Atomic replacement is required: interruption must leave the previous valid store available.

### Reconciliation and failures

Before startup attaches and after a successful sidebar refresh, tflow reconciles persistent metadata with an authoritative tmux session list. It removes metadata for missing sessions and then removes projects with no live sessions. A confirmed absent dedicated tmux server is an authoritative empty list; other tmux errors do not trigger cleanup or state writes.

Recovery rules stay local to each operation:

- if session creation cannot be persisted, kill the newly created tmux session
- if rename cannot be persisted, rename the tmux session back
- after delete failure or external disappearance, reload and reconcile from tmux
- surface the original operation error

A generalized transaction, compensation, or background reconciliation framework is not part of the design.

## Key interactions

- `Ctrl+F`: toggle the sidebar
- `Ctrl+Q`: confirm shutdown of the current instance and remove its volatile sessions
- `Ctrl+C`: close the sidebar
- `Esc`: cancel the active dialog, hide help, or close the sidebar
- `?`: toggle inline shortcut help
- `j` / `k`: move through the current list
- `Enter`: switch to the selected session or confirm a dialog
- `n`: create a session
- `N`: create a project
- `p`: switch project
- `r` / `R`: rename the selected session / current project
- `d` / `D`: delete the selected session / current project
- `e`: edit project settings

## Installation and dependencies

- `tmux` is required.
- Bubble Tea and Lip Gloss provide the sidebar UI.
- The first alpha supports installation through both Nix and `go install`.
- The Go command lives at `cmd/tflow` and installs as `tflow` once the Go install task is complete.
- Broader tmux 3.2-current error-message compatibility is post-alpha work.
- Prefer direct, testable session handling over speculative abstractions.
