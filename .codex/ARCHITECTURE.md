# tflow

`tflow` is a terminal session manager using `tmux` as its backend.

`tflow` starts with one volatile session named `code`.

Volatile sessions are not attached to a project. In volatile mode, the project badge is still shown, but its value is empty.

If the terminal running `tflow` exits or `Ctrl+Q` is confirmed, all volatile sessions owned by that instance are terminated. Persistent project sessions are left untouched.

Each `tflow` instance is runtime-isolated. It owns its own volatile sessions and must not interfere with the runtime sessions or behavior of any other `tflow` instance.

Persistent projects are shared globally through the store.

## Terminal UI

On startup, `tflow` looks like a normal terminal.

Normal Terminal means that all coloring, font and style settings, will be 100% identical to a normal terminal.

This is very IMPORTANT, `tmux` must not overwrite any color or skin of other programs.

There is no status bar or additional UI, except for two `key=value` badges in the top bar showing the current project and session name.

The badges are updated whenever the session or project changes, so they always reflect the active state.

The core feature is a fixed-width sidebar toggled with `Ctrl+F`.
The sidebar should be slim, using not more as 20%-25% space.
The content of the sidebar is centered.

The sidebar is shown on the left side and contains:

- a header section with a centered `TFLOW` badge
- a session list, which visually a component with boarder and shadow
- a project list, which visually a component with boarder and shadow
 an always-visible command line
- a help section shown only when help is enabled

All sidebar-related prompts, inputs, confirmations, and dialog messages use the command line.

`Ctrl+Q` is the only exception. Because it can be triggered outside the sidebar, its confirmation uses a centered dialog.

Command-line messages disappear automatically after a short time.

Invalid input keeps the current prompt open, shows an error, and allows the user to correct the input.

## Runtime Architecture

`tflow` uses `tmux` panes/windows as the terminal runtime.

The sidebar and badges are implemented by `tflow` as the controlling TUI around the active tmux client/session, not as tmux status lines.

The initial backend is a minimal `tmux` abstraction built on ordinary `tmux` commands.

`tmux` control mode and VT-style terminal rendering are not mandatory for the baseline implementation.

They are introduced only if the simple backend cannot support the required outer TUI behavior.

Persistent session identity is stored in `store.json`.

Each persistent session has an internal stable ID used to derive the internal tmux session name.

User-visible names are project/session names only. Internal tmux names are never shown.

Multiple `tflow` instances share the same persistent store. If two instances select the same persistent session, they attach to the same underlying tmux session.

Volatile sessions are owned by one `tflow` instance only and are never shared.

`tflow` captures `pane_current_path`:
- before switching sessions
- before switching projects
- before moving sessions
- before shutdown when possible

CWD persistence is best-effort. Crash or forced termination may lose the latest cwd update.

On normal exit or confirmed `Ctrl+Q`, volatile sessions are killed.

Moving a session into a project fails if the destination project already contains a session with the same name.

When deleting the active project, `tflow` switches to the next project in store order. If the deleted project was the last project, it wraps to the first project. If no persistent project remains, `tflow` switches to a fresh volatile `code` session.


## Projects and Sessions

Projects are persistent and managed only through the `tflow` UI.

A project has:

- a globally unique name
- a `workdir`
- at least one session

Session names are unique within one project.

Different projects may contain sessions with the same name. Switching, moving, and selecting sessions must use the project context internally, so equal session names across projects never conflict.

A session can be moved into a project. This makes the session persistent and lets it survive terminal exit or `Ctrl+Q`.

When the active context is volatile, creating a project attaches all current volatile sessions to it, writes the project to the store, and selects the new project.

When the active context is already inside a persistent project, creating a project creates a new project with a fresh `code` session in the new project `workdir`.

If no volatile sessions exist when a project is created, `tflow` creates a default `code` session in the project `workdir`.

Project order follows the store order. New projects are appended to the end.

Session order follows creation order.

When switching to a project, `tflow` selects the first session by creation order.

Renaming a project or session keeps its position and updates the store.

Renaming a project validates global project name uniqueness.

Renaming a session validates session name uniqueness within the current project.

Duplicate names are rejected with an error in the command line.

## Lifecycle and Store

`tflow` does not use a user-edited config file.

Persistent data is stored in `$XDG_STATE_HOME/tflow/store.json`.

The store is the source of truth for:

- projects
- sessions
- persistent session stable IDs
- project `workdir`
- last known working directory per session

The store is shared by all `tflow` instances.

If no store exists, `tflow` creates an empty one.

Invalid `store.json` fails startup with a clear error.

Duplicate project names in `store.json` fail startup with a clear error.

Duplicate session names within one project fail startup with a clear error.

Duplicate persistent session stable IDs in `store.json` fail startup with a clear error.

Store writes must use file locking and atomic rename.

`tflow` stores project `workdir` and session `cwd` as expanded absolute paths.

Persistent projects are loaded from the store on startup.

Persistent sessions are restored lazily when switching to a project from the sidebar.

Lazy restore only restores sessions for the selected project.

Restored persistent sessions reopen in their last known working directory.

If a restored session `cwd` no longer exists, `tflow` falls back to the project `workdir`.

If a persistent project has no saved sessions, `tflow` creates a default `code` session in the project `workdir`.

Volatile sessions are never restored.

Project creation validates that `workdir` exists before writing to the store.

If a project `workdir` does not exist, selecting or creating the project fails and asks the user to change the `workdir`.

`tmux` session names are internal only and must never be shown in the UI.

Internal `tmux` session names must be globally unique.

```json
{
  "projects": [
    {
      "name": "project-a",
      "workdir": "/home/user/Projects/project-a",
      "sessions": [
        {
          "id": "3d7a67d6-3f57-4a7e-8ae0-f9ea4d8164d7",
          "name": "code",
          "cwd": "/home/user/Projects/project-a"
        }
      ]
    }
  ]
}
```

## Deletion

Destructive confirmations use a `y/N` prompt.

Deleting a session requires confirmation in the command line.

Deleting the last session of a project prompts for deleting the whole project.

If the project deletion is cancelled, the session deletion is cancelled.

Deleting a project kills all sessions of that project and removes it from the store.

Deleting the active project switches to the next persistent project by store order.

If no next persistent project exists, `tflow` switches to a fresh volatile `code` session.

Deleting a non-active project does not change the current session or project.

Moving the last session out of a project prompts for deleting the now-empty project.

If confirmed, the move completes and the empty project is deleted.

If cancelled, the move is cancelled.

## New Sessions

Creating a new session prompts for the session name in the command line.

Duplicate session names in the current project are rejected with an error in the command line.

When a project is selected, new sessions start in the project `workdir`.

When no project is selected, new volatile sessions start in the current session working directory.

## Moving Sessions

`m` moves only the selected session.

`m` can move a volatile session into an existing project.

Moving a session removes it from its source project or volatile session list.

If all volatile sessions are moved into projects, `tflow` creates a fresh volatile `code` session.

## Editing Projects

`e` edits the selected project `workdir`.

The new `workdir` must exist before it is written to the store.

The new `workdir` is stored as an expanded absolute path.

## Keybindings

- `Ctrl+F`: toggle the sidebar in the current session window
- `Ctrl+Q`: open a centered confirmation dialog to terminate the current `tflow` instance
- `Ctrl+C`: close the sidebar when it is open
- `Ctrl+C`: pass through to the terminal session when the sidebar is closed
- `Esc`: cancel the active prompt, hint mode, or confirmation first; close the sidebar on the next press
- `?`: toggle help on or off
- `Tab`: switch focus between the session list and the project list
- `j` / `k`: move up or down in the focused list
- `Enter`: switch to the selected session and close the sidebar
- `Enter`: switch to the selected project and its first session by creation order
- `n`: create a new terminal session
- `N`: create a new project
- `e`: edit the selected project `workdir`
- `r`: rename the selected session or project
- `m`: move the selected session to another project using hint mode
- `d`: delete the selected session with confirmation
- `D`: delete the selected project and all of its sessions with destructive confirmation

`Ctrl+Q` confirmation shows the number of volatile sessions that will be killed.

`Ctrl+Q` kills only volatile sessions. Persistent sessions stay alive.

`Ctrl+Q` is ignored while a sidebar prompt, hint mode, or confirmation is active.

## Hint Mode

Hint mode is entered by actions that require selecting a project.

When hint mode is active, `tflow` highlights the required starting characters of each project name. The highlighted characters are colorized.

If multiple projects share the same prefix, hint mode continues until the typed characters identify exactly one project.

Hint mode follows the project order from the store.

Example projects:

- Apple
- Pier

In this case, `A` and `P` are highlighted. Pressing one of them selects the matching project.

## Workdir Autocompletion

Workdir prompts use path autocompletion.

While entering a `workdir` in the command line, pressing `Tab` completes the current path segment.

If multiple matches exist, `tflow` shows a limited number of available matches in the command line and waits for more input.

If more matches exist than can be shown, the command line indicates that the result was truncated.

## Dependencies and Design

- `tmux` is required. If it is missing, `tflow` fails startup with a clear error.
- Charmbracelet modules are used for all TUI elements.
- The design uses the Catppuccin color palette.
- The UI should be minimal, consistent, and visually clean.
