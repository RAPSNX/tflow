# Issue tracker

This file is human written and contains findings while using `tflow`.
Findings must be validated against `ARCHITECTURE.md` and `TASK.md`, then
turned into correct derived tasks. If a finding conflicts with the current
architecture, stop and ask questions.

## Reviewed findings

### Projects disappear after reboot

Confirmed. Persistent project and session metadata must remain after tmux is
restarted, and missing sessions are recreated lazily only when selected.

### Project creation working directory

Confirmed. New projects use the originating active tmux pane current directory
as their initial workdir.

### Home paths and completion

`~` is expanded when a project workdir is saved. Path completion is out of
scope because the external editor provides it.

### Project settings editor

Confirmed. `e` temporarily replaces the sidebar with a YAML document in
`$EDITOR`, falling back to `nvim`; tflow strictly validates the document and
persists the supported settings to the central JSON store. The temporary
document is removed after editing and is not a persistent configuration file.

### Unexpected project changes after deletion

Confirmed. Deletion must never select another persistent project. Explicit
project selection, project creation, and successful session moves may switch
the client; a deletion either keeps the client in its current project or uses
a volatile fallback when it removes the final active project session.
