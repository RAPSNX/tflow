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
binary, optional git binary, and ordered persistent sessions. A persistent
session contains its tmux ID, display label, type (`terminal`, `git`, or
`agent`), and a captured executable for agent sessions.

Persistent project and session records survive missing tmux sessions. The
sidebar treats stored order as the complete project session list. Selecting a
missing session, or a project whose first session is missing, creates that
stored ID in the project workdir, restores its markers, and switches the
originating client without changing persistent state.

Ordinary project creation captures the originating pane's directory before
starting its short-lived worker, then adds two lazy records in order: `code`
(`terminal`) and `git` (`git`). The `code` session is materialized and selected
as the creation target; the originating client switches to it. Git sessions
run the owning project's git binary, `lazygit` by default, when materialized.
The `n` action creates terminal sessions only. Existing projects and
projects created by volatile-session promotion receive no presets. New
project sessions use the project workdir; all volatile sessions, including
deletion fallbacks, use the active pane's directory.

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
executable while disabling future automatic provisioning. Missing agent
binaries produce clear, non-mutating materialization errors.

Project settings also accept a `git-binary` executable name or absolute path
without arguments, defaulting to `lazygit` when unset. A project holds at
most one git session, always labeled `git`. The standard project presets
already create one, but a project can still end up without one -- promotion
creates no presets at all, and, like the agent session, deleting or moving
away a project's only git session leaves it without one; nothing
re-provisions it automatically outside of saving a `git-binary` setting. The
git session's label can never change: a rename targeting it is rejected.
Unlike the agent session's captured command, a git session stores no command
of its own -- its launch command is the owning project's current
`git-binary` (or `lazygit` when unset), resolved fresh at materialization
time, so a later `git-binary` change takes effect on that session's next
materialization without a separate update step. Missing git binaries
produce clear, non-mutating materialization errors.

Moving a persistent session preserves its tmux session and ID, appends it to
the target project, and switches the originating client to it. A move fails
if the target already has the same label, already has an agent session and
the moved session is also an agent session, or already has a git session and
the moved session is also a git session. Moving the source project's final
session deletes that project.

Explicit deletion follows these rules:

* deleting a project removes all its persistent sessions and metadata
* deleting a non-active session or project leaves the client unchanged
* deleting an active session selects another session only from the same project
* deleting the final active session or active project creates, configures, and
  switches to a volatile fallback before removing sessions or metadata
* deleting a project's final session deletes the project

## Terminal interface

### Keys

`Ctrl+F` then `f` toggles the sidebar. `Ctrl+F` alone enters command mode: a
brief wait state for the chord's second key, mirroring how tmux's own prefix
key works, with a visible `COMMAND` indicator pill in the status bar for as
long as it lasts. Either `f` or a held `Ctrl+F` (someone holding Ctrl down
through both presses never releases it between them, so the second key can
arrive as `Ctrl+F` instead of a plain `f`) finishes the chord and opens the
sidebar; any other key, or no key at all, silently reverts to normal input
with no timer or explicit cancellation needed. The indicator disappears the
moment the sidebar opens -- command mode is specifically the `Ctrl+F` wait,
not "the sidebar is open"; the sidebar itself (badge and bordered session
list) is an unambiguous enough cue on its own once visible. While the
sidebar is open, `h` selects the previous contextual session and `l` selects
the next; either action closes the sidebar and returns the client to normal
input. Repeating `Ctrl+F, f`, or pressing `Esc` or `Ctrl+C`, closes the
command sidebar without navigating.

From command mode itself, before the sidebar ever opens, `h` and `l` also
switch directly to the previous or next contextual session, and `g` jumps
directly to the current project's git session; each acts immediately and
returns to normal input without displaying the sidebar at all. Creating a
session or project, switching projects, renaming or deleting a session,
moving a session, renaming or deleting a project, and editing project
settings are reachable the same way: pressing that action's key from
command mode opens the sidebar already inside that action's flow, skipping
its plain session list. The sidebar's own `j`/`k` selection and
`Enter` stay reachable only once the sidebar is visible, since there is
nothing to move through or select before its list exists. Other sidebar
shortcuts keep their normal behavior. `Ctrl+Q` opens confirmation for
quitting the current instance and removing its volatile sessions. No
configuration or key replay is involved.

### Command sidebar

The command sidebar never covers the status line, so the top bar stays readable
while command mode is active. Inside the popup, the tflow badge sits as a
filled, coloured pill on its own line, with the session list stacked below
it, offset to the right and framed in its own thin border. The whole component is
centered both horizontally and vertically in the popup rather than pinned to a
corner. The session list renders at a fixed width rather than stretching to
fill the popup, with a "Sessions" header, a blank line, then every contextual
session stacked one per row below it, each shown as its type icon (`>_` code,
`⎇` git, `✦` agent) plus its label; a session's icon turns green instead of
its type color when it is the live, attached one, independent of selection.
The badge is the one deliberate exception to the popup's single shared
background: a filled, coloured pill, so it reads as a static logo mark
rather than a list entry. Nothing else has a background of its own -- the
chips distinguish themselves by colour and weight alone. The selected row
is marked by a leading marker glyph (`▎`) plus
bold, mauve text for both the marker and the label, rather than a background
block or a "live" text badge -- selection and live status are shown
independently of each other, never conflated into one indicator:

```
┌────────────────────────────────────┐
│ TFLOW                              │
│          ╭────────────────────╮    │
│          │                    │    │
│          │    Sessions        │    │
│          │                    │    │
│          │  ▎ >_  feature-x   │    │
│          │    >_  fox         │    │
│          │                    │    │
│          ╰────────────────────╯    │
│                                    │
└────────────────────────────────────┘
```

(the outer box above is the tmux popup frame itself, not part of tflow's own
rendering; the inner box is the session list's own thin border, separate from
the unboxed badge above it; `feature-x` carries the leading `▎` marker and
renders in bold mauve in the real popup to mark it as the selected row --
independent of whether it is also live, which would show as its own `>_`
icon turning green regardless of selection; `fox` is a plain, unselected
row with no marker.)

Navigation moves through the same order shown by the sidebar: stored order in
the active project or tmux list order for the current instance's volatile
sessions. Navigation is bounded, halting at the start or end of the context
without wrapping. It never crosses projects or instances, lazily materializes
missing persistent targets, remains client-scoped, and does not run
sidebar-only exited-session cleanup.

Tmux owns popup lifetime. Successful actions close the sidebar and return
focus to the terminal. Valid session and project creation closes it once tmux
accepts the short-lived worker, while creation and switching continue in the
background.

### Top bar

The top bar is tflow's primary state view. It opens with a project section,
rendered as a filled, rounded-cap pill, then all contextual sessions in their
exact order, showing each session once as plain text with its type icon
(`>_` code, `⎇` git, `✦` agent) -- except the active session, which is
rendered the same filled, rounded-cap pill as the project section, with its
icon forced green regardless of type. There is no separate divider glyph
between the project pill and the sessions; the project pill's own closing
cap, followed by a gap, is the section split. The project section is always
present: it names the active project, and in a volatile context it renders
as an empty pill rather than being omitted, so the bar keeps the same shape
in every context.

A switch computes derived, session-scoped status metadata for its selected
target from post-mutation state. A successful rename, non-active deletion, or
settings change that alters the originating client's displayed context
refreshes only its active session. Moves and creation use their required target
switch; inactive and unrelated sessions are never rewritten. Post-switch
cleanup that removes an outgoing session refreshes the selected target again.
Derived metadata is never persisted. Every refresh above is a push from the
mutation that causes it, not a loop -- the lone exception is the attention
scan described below, the one bounded, tmux-native timer in the design.

### Session types and indicators

Every session carries a type identity: blue code, teal git, or yellow agent.
Both sidebar rows and top-bar entries render the icon and colour alone --
`>_`, `⎇`, or `✦` -- never the spelled-out type name, to keep the line short.
Selection never replaces the type identity. Green `live` and red attention
indicators remain independent of type and selection.

### Attention

A runtime-only session attention marker is set when an unvisited session
produces output; it is shown in the sidebar and top bar, is never written to
JSON, and may disappear when tmux restarts. Any client visit clears the
marker for the entered session and, via tmux's client-session-changed hook,
stamps a fresh watermark for both the entered session and the one being
switched away from (tmux's `client_last_session`) -- so output produced late
in a visit, in the gap before the next status tick, still can't look unseen
once that session is detached.

A session's watermark is that session's own peak window activity time at the
moment it is stamped, not wall-clock time: comparing two timestamps from the
same tmux clock stays unambiguous even when a visit and some output land in
the same one-second tick, since a plain "activity is later than the
watermark" check means exactly what it says -- wall-clock time would leave
that same-second case a coin flip no matter which way ties broke.

The mechanism the feature depends on is tmux's own status-interval timer --
set short and global -- driving an invisible `#()` job on every status-line
redraw. Each tick, that job refreshes the viewed session's own watermark (so
output produced while it is being viewed is never mistaken for unseen even
before the client leaves), scans every window of every session, sets the
marker for any unattached session whose latest window activity is later than
its watermark -- keeping a stale background-window flag, which is only
cleared by individually selecting that window, from re-triggering attention
on an already-visited session -- and refreshes the scanning client's own
visible top bar so a sibling session's attention reaches it without an
unrelated mutation to trigger a push. tmux's alert-activity hook is also
installed and may set the marker earlier on builds where it fires, but
nothing depends on it.

### Mouse

Mouse reporting is enabled only for wheel scrolling through pane history.
Every other mouse interaction is unbound in root and copy-mode tables.
Terminal-native text selection therefore needs the terminal's override
modifier, such as Shift in Alacritty.

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
    "gitBinary": "lazygit",
    "sessions": [
      {"id": "tflow-p-8f42ac91", "label": "code", "type": "terminal"},
      {"id": "tflow-p-a13d5e02", "label": "git", "type": "git"},
      {"id": "tflow-p-96ad4c10", "label": "agent", "type": "agent", "command": "codex"}
    ]
  }]
}
```

`agentBinary`, `gitBinary`, and session `command` fields may be omitted where
inapplicable. Missing `type` on an older record means `terminal` without
migration or rewrite. Within one project, a `git`-typed record's label is
normalized to `git`, and every `git`-typed record after the first (stored
order) is normalized to `terminal` -- both without migration or rewrite.
Present types must be `terminal`, `git`, or `agent`; agent sessions
require a command; a git session's launch command is its project's
`gitBinary`, not a stored field, so terminal and git sessions both forbid
one. State is rejected, with a path-qualified error, for empty or
duplicate normalized project names, empty or duplicate session IDs, empty
or duplicate labels within a project, duplicate agent sessions, or other
schema violations. Unknown JSON fields may be ignored.

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
Only `workdir`, `agent-binary`, and `git-binary` are accepted; unknown keys
or invalid YAML are rejected. Valid changes use the normal JSON mutation
path. Editor, validation, or persistence failures leave state unchanged and
are reported when the sidebar resumes.

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
sessions it creates, promotes, renames, moves, or deletes. Optimizations
require command counts or measurements, and implementation favors direct
testable code over new lifecycle, persistence, or recovery frameworks.

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
