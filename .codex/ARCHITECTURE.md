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

Sessions created outside a project are volatile, belong to the current `tflow` instance, and follow the same cleanup rules.

`tflow` contains a reviewed, compiled list of exactly 25 animal names. The list is fetched once from a public animal API during development and is never requested at runtime.

The startup volatile session and the initial session of every newly created project receive a randomly selected animal name from that list. A single-animal session name is exactly the selected animal name.

When all single-animal names are in use for volatile sessions, `tflow` uses an unused two-animal name from the same list. It adds a numeric suffix only after all such combinations are in use.

Persistent sessions are ordinary `tmux` sessions grouped into projects by `tflow` metadata. They are not restored from a complex runtime snapshot.

## Terminal UI

On startup, the user should see a normal live terminal session.

The primary interaction model is:

- the active terminal runs directly inside `tmux`
- `Ctrl+F` toggles a slim sidebar as a `tmux` popup overlay anchored to the left edge of the active client
- switching sessions closes the sidebar and returns focus to the selected session
- toggling or closing the sidebar does not resize or move the active terminal
- closing the sidebar does not surface tmux error text to the user
- `Ctrl+Q` opens the quit confirmation from the live terminal even when the sidebar is closed

The top UI shows two badges:

- `project`
- `session`

In volatile mode, the project badge value is empty.

The sidebar is shown on the left and contains:

- a `TFLOW` header centered
- a session list
- an inline command and status area
- there is no metadata or help displayed in the sidebar by default

The sidebar handles the core management actions:

- create project or session
- rename project or session
- delete project or session
- switch to another project
- update project settings
- quit the current `tflow` instance
- typing `?` will open a help list, with all available shortcuts, one per row

## Design

The sidebar uses Charmbracelet Bubble Tea for interaction and Lip Gloss for terminal-native layout and styling. Visual design is limited to terminal-safe text, color, padding, and borders.

Badges share a compact, filled style with bold text and horizontal padding:

- `TFLOW` is the centered sidebar brand badge, with a blue background and dark foreground.
- `live` is a green badge shown immediately before the label of the actually active session.

The `live` badge appears exactly once in the session list. It represents the active terminal session, not merely the keyboard-selected row, and remains legible when that row is selected.

Every input, rename, settings, and confirmation dialog uses the same centered structured-card layout:

- a dark rounded card with a subtle border
- an accent badge and clear title in the header, followed by a divider
- muted context text and, where needed, a bordered focused input field
- a footer of keyboard keycaps: the primary action on `Enter` and cancellation on `Esc`

Create, rename, settings, and normal confirmations use the blue and teal interface accents. Deletion confirmations use a red header badge and red `Enter` action keycap; `Esc` remains neutral.

## Projects and Sessions

Projects are lightweight groups over `tmux` sessions.

A project contains:

- a unique project name
- a default `workdir`

Creating a project stores the current working directory as its default `workdir` and creates its initial session there with a random animal name.

A session belongs to at most one project.

When a new session is created:

- inside a project, it starts in that project `workdir` when one is set
- outside a project, it starts in the current working directory as a volatile session owned by the current `tflow` instance

Switching to another project is always supported.

Switching to a project uses the command line and shows all existing projects in a readable newline-separated list.

Typing enough characters to uniquely match a project and pressing `Enter` switches to that project.

For example, after starting project switch with `p`, typing `gar` switches to `gardener` and typing `gat` switches to `gate` when those are unique matches.

Switching to a project selects that project's first session and closes the sidebar.

Switching from a volatile session to a project requires confirmation.

Switching from one project to another is direct.

Deleting the last session of a project requires confirmation. Confirming deletes both the session and the now-empty project.

Project settings contain only the project `workdir`.

## State

`tflow` does not use a user-edited `config.yaml` or per-project YAML files.

All persistent metadata lives in one JSON file:

- `$XDG_STATE_HOME/tflow/store.json`

The store keeps only the metadata needed for `tflow` to rebuild project and sidebar state:

- project order
- project membership for sessions
- display labels for project-scoped sessions
- the `workdir` for each project

The store uses one canonical JSON schema. Unknown fields and obsolete fields such as session types, project protection, cluster settings, and agent settings make the store invalid and startup fails with a clear error.

The state file is owned and updated by `tflow`. Packaging integrations such as Home Manager must not render or manage `store.json`.

If the state file does not exist, `tflow` creates an empty one when needed.

If the state file is invalid, startup should fail with a clear error.

## Key Interactions

- `Ctrl+F`: toggle the sidebar
- `Ctrl+Q`: confirm shutdown of the current `tflow` instance and remove its volatile sessions
- `Ctrl+C`: close the sidebar when it is open
- `Esc`: cancel the active prompt or confirmation before closing the sidebar
- `?`: open the shortcut help list; `Esc` returns to the session list
- `j` / `k`: move through the current list
- `Enter` on a session: switch to that session and close the sidebar
- `n`: create a new session
- `N`: create a new project
- `p`: switch to another project
- `r`: rename the selected session
- `R`: rename the current project
- `d`: delete the selected session with confirmation
- `D`: delete the current project with confirmation
- `e`: edit project settings

## Dependencies and Design

- `tmux` is required
- Charmbracelet libraries are used for the sidebar UI
- the design should prefer simple session handling over speculative abstractions
