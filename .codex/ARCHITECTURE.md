# tflow architecture

This document defines tflow's intended end state. `.codex/TASK.md` lists the
parts not implemented yet.

tflow is a small terminal session manager built on tmux:

* terminals are ordinary tmux sessions on one dedicated socket
* the sidebar is a tmux popup built with Bubble Tea and Lip Gloss
* tmux is the source of truth for running sessions
* one JSON file stores persistent metadata
* there is no daemon, terminal emulator, or popup process supervisor

## Ownership and lifetime

Starting tflow creates a volatile session and attaches a client. Persistent
sessions belong to projects; volatile sessions belong to one tflow instance
and are removed when that instance exits or its client detaches. Cleanup never
removes persistent sessions or another instance's volatile sessions.

An instance ID belongs to the attached client. Popups receive it explicitly
from the active volatile session or a deliberately client-keyed entry retained
after switching to a persistent session. tflow never uses an ambient,
unscoped instance value. Client-scoped operations target the originating
client; if it disappears, a replacement is allowed only when it is proven to
belong to the same instance.

Managed panes use `remain-on-exit`. tflow neither respawns exited shells nor
automatically switches away from them. Before an explicit sidebar switch,
tflow checks whether every pane in the outgoing session has exited. After a
successful switch to a different target, it removes that outgoing session;
sessions with a live pane and no-op reselections remain. This cleanup occurs
only for sidebar session or project selection. Removing an exited persistent
session also removes its metadata and an empty project; removing a volatile
session does not touch persistent state. The selected target is never replaced
by an implicit fallback during this cleanup.

### Signal shutdown

One runtime context is canceled by SIGHUP, SIGINT, or SIGTERM and is passed
only to the attached tmux client and Bubble Tea popup. After creating and
tagging a volatile session, the owning process always attempts instance-scoped
cleanup before exit. Signal handling first requests graceful client
termination, waits for a bounded interval, then forces termination and follows
the same cleanup path.

A canceled popup exits without invoking the user-facing quit action; its shell
wrapper clears the client-scoped popup marker. Signal cleanup never replaces
an operation error, although a cleanup-only failure is reported. SIGKILL,
process or machine crashes, and power loss are outside the design.

## Projects and sessions

Tmux session names are opaque internal IDs. Persistent sessions use
`tflow-p-<id>`; volatile sessions use `tflow-v-<instance-id>-<id>`. Project
names and display labels live in metadata, so renaming them does not rename a
tmux session. Generated labels are readable and unique in their visible
scope. User labels preserve casing and must be exactly unique within their
project or owning volatile instance.

A project contains a unique name, default working directory, optional agent
binary, and ordered persistent sessions. A persistent session contains its
tmux ID, display label, type (`terminal`, `git`, or `agent`), and a captured
executable for agent sessions.

Persistent project and session records survive missing tmux sessions. The
sidebar treats stored order as the complete project session list. Selecting a
missing session, or a project whose first session is missing, creates that
stored ID in the project workdir, restores its markers, and switches the
originating client without changing persistent state.

Ordinary project creation captures the originating pane's directory before
starting its short-lived worker, then adds two lazy records in order: `code`
(`terminal`) and `git` (`git`). The `code` session is materialized and selected
as the creation target; the originating client switches to it. Git sessions run
`lazygit` when materialized. The `n` action creates terminal sessions only.
Existing projects and projects created by volatile-session promotion receive no
presets. New project sessions use the project workdir; all volatile sessions,
including deletion fallbacks, use the active pane's directory.

Creating a project from a volatile session promotes every volatile session
owned by that instance. Promotion preserves labels and order, assigns new
persistent IDs, clears volatile markers, ignores foreign instances, and
switches directly to the promoted successor of the previously active session.
Before state is persisted, failure restores renamed sessions and ownership;
failed restoration triggers direct best-effort cleanup. The original error is
reported and the sidebar does not claim success.

Project settings accept an agent executable name or absolute path without
arguments. Saving a non-empty `agent-binary` adds one lazy agent session when
none exists, using `agent` if the label is free or the first unused label in
`agent-2`, `agent-3`, and so on. A project holds at most one agent session.
Later saves update that session's captured executable but not a currently
running process. Clearing the setting retains the session and captured
executable while disabling future automatic provisioning. Missing `lazygit` or
agent binaries produce clear, non-mutating materialization errors.

Moving a persistent session preserves its tmux session and ID, appends it to
the target project, and switches the originating client to it. A move fails if
the target already has the same label or, for an agent session, any agent
session. Moving the source project's final session deletes that project.

Explicit deletion follows these rules:

* deleting a project removes all its persistent sessions and metadata
* deleting a non-active session or project leaves the client unchanged
* deleting an active session selects another session only from the same project
* deleting the final active session or active project creates, configures, and
  switches to a volatile fallback before removing sessions or metadata
* deleting a project's final session deletes the project

## Terminal interface

`Ctrl+Space` enters a fixed, one-command tmux key table: `h` selects the
previous contextual session, `l` selects the next, and `o` opens the sidebar
overview. One key returns to normal input; an unknown key, `Esc`, or `Ctrl+C`
cancels. No configuration, timer, or key replay is involved, and tflow does
not bind `Ctrl+F`. `Ctrl+Q` opens confirmation for quitting the current
instance and removing its volatile sessions.

Navigation wraps through the same order shown by the sidebar: stored order in
the active project or tmux list order for the current instance's volatile
sessions. It never crosses projects or instances, lazily materializes missing
persistent targets, remains client-scoped, and does not run sidebar-only
exited-session cleanup.

The top bar shows the previous, active, and next contextual sessions, or only
the active session when it is alone. A switch computes derived, session-scoped
status metadata for its selected target from post-mutation state. A successful
rename, non-active deletion, or settings change that alters the originating
client's displayed context refreshes only its active session. Moves and
creation use their required target switch; inactive and unrelated sessions are
never rewritten. Post-switch cleanup that removes an outgoing session refreshes
the selected target again. Derived metadata is neither persistent nor
maintained by a daemon or refresh loop.

Every sidebar row and top-bar entry has a type chip: blue `>_ CODE`, teal
`⎇ GIT`, or yellow `✦ AGENT`. Selection never replaces the chip. Teal `live`
and red attention indicators remain independent of type and selection.

Tmux owns popup lifetime. Successful actions close the sidebar and return
focus to the terminal. Valid session and project creation closes it once tmux
accepts the short-lived worker, while creation and switching continue in the
background.

Mouse reporting is enabled only for wheel scrolling through pane history.
Every other mouse interaction is unbound in root and copy-mode tables.
Terminal-native text selection therefore needs the terminal's override
modifier, such as Shift in Alacritty.

Tmux activity hooks set a runtime-only session attention marker when an
unvisited session produces output. Any client visit clears it. The marker is
shown in the sidebar and top bar, is never written to JSON, and may disappear
when tmux restarts.

## Persistent state

State is stored at `$XDG_STATE_HOME/tflow/store.json` when
`XDG_STATE_HOME` is non-empty, otherwise at
`~/.local/state/tflow/store.json`. It contains project and persistent-session
metadata only; runtime ownership and indicators stay in tmux.

The intended schema is:

```json
{
  "projects": [{
    "name": "example",
    "workdir": "/home/user/example",
    "agentBinary": "codex",
    "sessions": [
      {"id": "tflow-p-8f42ac91", "label": "code", "type": "terminal"},
      {"id": "tflow-p-96ad4c10", "label": "agent", "type": "agent", "command": "codex"}
    ]
  }]
}
```

`agentBinary` and session `command` fields may be omitted where inapplicable.
Missing `type` on an older record means `terminal` without migration or
rewrite. Present types must be `terminal`, `git`, or `agent`; agent sessions
require a command and other types forbid one. State is rejected, with a
path-qualified error, for empty or duplicate normalized project names, empty
or duplicate session IDs, empty or duplicate labels within a project,
duplicate agent sessions, or other schema violations. Unknown JSON fields may
be ignored.

Every mutation holds one advisory lock, reloads current state, applies the
change, encodes the complete state, writes a temporary file in the state
directory with mode `0600`, closes it, renames it over `store.json`, and then
unlocks. File and directory `fsync` are intentionally omitted; sudden
power-loss durability is out of scope.

Startup reconciles once while holding the state lock. Missing tmux sessions
retain their metadata for lazy materialization. Existing persistent sessions
have project and label markers restored and stale volatile markers cleared;
unrelated sessions are untouched. A tmux listing error performs no repair.
Normal sidebar refresh lists sessions once, filters locally, and never mutates
persistent state.

The `e` action opens a temporary YAML document in `$EDITOR`, or `nvim` when
unset. The file is removed on every exit path and is not user configuration.
Only `workdir` and `agent-binary` are accepted; unknown keys or invalid YAML
are rejected. Valid changes use the normal JSON mutation path. Editor,
validation, or persistence failures leave state unchanged and are reported
when the sidebar resumes.

## Errors and performance

Operations report their original error and do not use a generalized
transaction or rollback framework. Local cleanup may kill a newly created
session after setup or persistence failure, ignore resources already gone,
and leave other inconsistencies for startup reconciliation. Best-effort
cleanup failures emit diagnostics without replacing the original error.

If a sidebar target switch fails, the outgoing session remains. After a
successful switch, cleanup failure leaves the client on the selected target.
Failed tmux deletion retains persistent metadata; failed metadata removal
after tmux deletion retains the record for later lazy materialization.

Opening or refreshing the sidebar performs one session-list query and no
per-session writes when unchanged. Metadata changes only through explicit
operations or startup reconciliation. An operation updates markers only for
sessions it creates, promotes, renames, moves, or deletes; a switch may update
derived status only for its target. Optimizations require command counts or
measurements, and implementation favors direct testable code over new
lifecycle, persistence, or recovery frameworks.

## CLI and releases

The public commands are `tflow`, `tflow version`, `tflow --version`, and the
`help`, `-h`, and `--help` forms. Internal tmux workers remain absent from
public help.

Release versions are Semantic Versioning tags prefixed with `v`. Tagged builds
report the tag; module installs use identifying Go build metadata; other builds
report a development version, optionally with a Nix revision. A `v*` tag
pointing to `main` publishes a GitHub Release with checksummed `tar.gz`
archives for Linux and macOS on amd64 and arm64. Nix and Home Manager remain
source-based; no additional package feed is managed.
