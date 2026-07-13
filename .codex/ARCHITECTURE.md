# tflow

`tflow` is a terminal session manager built on top of `tmux`.

The architecture should follow the interaction model that already works on `main`:

- the active terminal stays a real `tmux` terminal
- the sidebar is opened as a real `tmux` pane
- the top badges are shown through `tmux`
- `tflow` tracks project and session metadata in one state file

The goal is a practical design that matches the current UI and session handling, without extra persistence or restore complexity.

## Runtime Model

`tflow` owns a dedicated `tmux` socket.

Each session managed by `tflow` is an ordinary `tmux` session.

`tflow` starts by creating one volatile session with a generated temporary name and attaching the user to it.

Volatile sessions belong only to the current `tflow` instance. They are used for scratch work and are removed when that instance exits normally or `Ctrl+Q` is confirmed.

Persistent sessions are just non-volatile `tmux` sessions that are grouped into projects by `tflow` metadata. They are not recreated from a complex runtime snapshot. If they still exist in `tmux`, they stay usable.

`tflow` may use `tmux` session options to mirror metadata such as:

- whether a session is temporary
- which project a session belongs to
- whether a pane is the sidebar pane

This is part of the intended design. In this architecture, `tmux` is both the runtime and part of the visible UI surface.

## Terminal UI

On startup, the user should see a normal live terminal session, not a captured or replayed terminal surface.

The primary interaction model is:

- the active terminal runs directly inside `tmux`
- `Ctrl+F` toggles a slim sidebar by splitting the current `tmux` window
- switching sessions closes the sidebar and returns focus to the selected session

The top line shows two badges with values:

- `project`
- `session`

In volatile mode, the project badge value is empty.

These badges are implemented through the `tmux` status line, as on `main`.

The sidebar is shown on the left and contains:

- a `TFLOW` header
- a session list section
- an inline command/status area (like a neovim commandline)

The neovim commandline acts as an input field.
The neovim commandline acts as a confirmation tooling.

The sidebar handles the current feature set:

- create project or session
- rename project or session
- delete project or session
- update project settings
- quit the current `tflow` instance

The UI should stay simple and responsive. The architecture does not require terminal capture, VT replay, or a full outer TUI around the live session.

## Projects and Sessions

Projects are lightweight groups over existing `tmux` sessions.

A project contains:

- a unique project name
- a `workdir` in which sessions start per default

Sessions are identified by their `tmux` session names.

Sessions have the following criterias:

- session names are only unique in project scope
- a session belongs to at most one project
- moving a session changes metadata, not the underlying runtime model
- adding the current volatile session to a project makes it persistent

Creating a project creates a default session named "code" automatically.

Deleting the last session of a project does ask via comandline bar.

When a new session is created:

- inside a project, it starts in that project `workdir` when one is set
- outside a project, it starts in the current working directory

## State

`tflow` does not use a user-edited `config.yaml` or per-project YAML files.

All persistent metadata lives in one JSON file:

- `$XDG_STATE_HOME/tflow/store.json`

The store is only meant to keep the current application state. It is not a full runtime database.

The store should contain enough information to rebuild the sidebar state and project metadata, including:

- project order
- session-to-project mapping
- per-project settings

The store should stay simple:

- no stable internal session IDs are required
- no mandatory file locking requirement is part of the architecture
- no mandatory atomic rename requirement is part of the architecture
- best-effort writes are enough for this version

The main requirement is that the file stays valid JSON and that `tflow` can load it on startup.

If the state file does not exist, `tflow` creates an empty one when needed.

If the state file is invalid, startup should fail with a clear error.

## Key Interactions

- `Ctrl+F`: toggle the sidebar in the current `tmux` window
- `Ctrl+Q`: confirm shutdown of the current `tflow` instance and remove its volatile sessions
- `n`: enter the new-item flow used on `main`
- `m`: move the selected session
- `r`: rename the selected item
- `d`: delete the selected item with confirmation
- `e`: edit project settings


`Ctrl+F`: toggle the sidebar in the current session window
`Ctrl+Q`: open a centered confirmation dialog to terminate the current tflow instance
`Ctrl+C`: close the sidebar when it is open
`Ctrl+C`: pass through to the terminal session when the sidebar is closed
`Esc`: cancel the active prompt, hint mode, or confirmation first; close the sidebar on the next press
`?`: toggle help on or off
`j` / `k`: move up or down in the focused list
- `Enter` on a session: switch to that session and close the sidebar
`n`: create a new terminal session
`N`: create a new project, which makes all volatile sessions part of it
`r`: rename the selected session
d: delete the selected session with confirmation

Exact prompt wording can change, but the interaction style should stay close to `main`.

## Dependencies and Design

- `tmux` is required
- Charmbracelet libraries are used for the menu/sidebar UI
- the design should stay close to the current visual behavior on `main`
- architecture should prefer simple session handling over speculative abstractions
