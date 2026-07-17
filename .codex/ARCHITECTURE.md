# tflow

`tflow` is a terminal session manager built on top of `tmux`.

The goal is a simple, tmux-native design:

- the active terminal stays a real `tmux` terminal
- the sidebar is opened as a real `tmux` popup
- the top UI shows the current project and session
- persistent metadata lives in one state file

The architecture should stay practical and avoid terminal capture, VT replay, or a full outer TUI around the live session.

## Runtime Model

`tflow` owns a dedicated `tmux` socket.

Each session managed by `tflow` is an ordinary `tmux` session.

On startup, `tflow` creates one volatile session and attaches the user to it.

Volatile sessions belong only to the current `tflow` instance. They are used for scratch work and are removed when that instance exits normally or `Ctrl+Q` is confirmed.

Persistent sessions are ordinary `tmux` sessions grouped into projects by `tflow` metadata. They are not restored from a complex runtime snapshot.

## Terminal UI

On startup, the user should see a normal live terminal session.

The primary interaction model is:

- the active terminal runs directly inside `tmux`
- `Ctrl+F` toggles a slim sidebar as a `tmux` popup overlay anchored to the left edge of the active client
- switching sessions closes the sidebar and returns focus to the selected session
- toggling or closing the sidebar does not resize or move the active terminal
- closing the sidebar does not surface tmux error text to the user

The top UI shows two badges:

- `project`
- `session`

In volatile mode, the project badge value is empty.

The sidebar is shown on the left and contains:

- a `TFLOW` header
- a session list
- an inline command and status area

The sidebar handles the core management actions:

- create project or session
- rename project or session
- delete project or session
- switch to another project
- update project settings
- quit the current `tflow` instance

## Projects and Sessions

Projects are lightweight groups over `tmux` sessions.

A project contains:

- a unique project name
- a default `workdir`

Creating a project creates a default session named `code`.

A session belongs to at most one project.

When a new session is created:

- inside a project, it starts in that project `workdir` when one is set
- outside a project, it starts in the current working directory

Switching to another project is always supported.

Switching to a project uses the command line and shows all existing projects in a readable newline-separated list.

Typing enough characters to uniquely match a project and pressing `Enter` switches to that project.

For example, after starting project switch with `p`, typing `gar` switches to `gardener` and typing `gat` switches to `gate` when those are unique matches.

Switching to a project selects that project's first session and closes the sidebar.

Switching from a volatile session to a project requires confirmation.

Switching from one project to another is direct.

Deleting the last session of a project requires confirmation.

## State

`tflow` does not use a user-edited `config.yaml` or per-project YAML files.

All persistent metadata lives in one JSON file:

- `$XDG_STATE_HOME/tflow/store.json`

The store keeps only the metadata needed for `tflow` to rebuild project and sidebar state, including:

- project order
- project membership for sessions
- per-project settings

If the state file does not exist, `tflow` creates an empty one when needed.

If the state file is invalid, startup should fail with a clear error.

## Key Interactions

- `Ctrl+F`: toggle the sidebar
- `Ctrl+Q`: confirm shutdown of the current `tflow` instance and remove its volatile sessions
- `Ctrl+C`: close the sidebar when it is open
- `Esc`: cancel the active prompt or confirmation before closing the sidebar
- `j` / `k`: move through the current list
- `Enter` on a session: switch to that session and close the sidebar
- `n`: create a new session
- `N`: create a new project
- `p`: switch to another project
- `r`: rename the selected item
- `d`: delete the selected item with confirmation
- `e`: edit project settings

## Dependencies and Design

- `tmux` is required
- Charmbracelet libraries are used for the sidebar UI
- the design should prefer simple session handling over speculative abstractions
