# tflow DESIGN

`tflow` is a tmux-backed terminal manager for project-scoped terminal and agent sessions.

The goal is to replace multiple terminal windows for one task with one `tflow` project containing multiple switchable sessions.

Typical usage:

```sh
alacritty -e tflow
```

## Target Model

- A project groups sessions for one logical task/context.
- A session always belongs to exactly one project.
- Project/session names shown in the UI are logical names.
- tmux session names are internal implementation details.
- Every new `tflow` start creates a volatile random animal project with a default `code` session.
- Random animal projects are always volatile.
- Volatile projects are not listed in the `Projects` section.
- Volatile projects cannot be switched into from another `tflow` instance.
- Volatile projects and all sessions inside them are cleaned up when the owning terminal exits.
- Persistent projects are defined only in `~/.config/tflow/config.yaml`.
- Persistent projects are listed in the `Projects` section.
- Persistent projects and their sessions survive terminal exit.
- Closing a terminal attached to a persistent project must not delete, stop, rename, or modify the project or its sessions.
- `P` persists the current volatile project.
- Persisting a project asks for `name`, `workdir`, and `agent-cmd`.
- After persisting, the current project is renamed to the chosen persistent name and written to `config.yaml`.
- `R` does not exist. Project renaming is not a separate action.
- `state.json` stores runtime and restore state, but is not the source of truth for persistent projects.

## config.yaml

`config.yaml` is the source of truth for persistent projects.

Path:

```text
~/.config/tflow/config.yaml
```

Target format:

```yaml
projects:
  - name: lala
    workdir: ~/some/path
    agent-cmd: tabnine
  - name: lulu
    workdir: ~/other/path
    agent-cmd: codex
```

Rules:

- if `config.yaml` does not exist, there are no persistent projects
- `tflow` may create `config.yaml` when the user persists a project with `P`
- persistent projects are stored directly in `config.yaml`
- there should be no separate per-project YAML file model
- use `agent-cmd` as the public config field

## state.json

`state.json` stores runtime and restore state.

It may store:

- current project
- current session
- logical session names
- internal tmux backing names
- session types
- session last known working directory
- volatile project ownership
- UI runtime state if needed

Rules:

- persistent projects are restored from `config.yaml`
- persistent project sessions may be restored from `state.json`
- volatile projects are never restored after logout/reboot
- stale state for removed persistent projects should be ignored or cleaned up
- `state.json` must never be required to recreate the persistent project list

Example restore state:

```json
{
  "persistentSessions": {
    "lala": [
      {
        "name": "code",
        "type": "terminal",
        "cwd": "/home/me/src/lala"
      },
      {
        "name": "agent",
        "type": "agent",
        "cwd": "/home/me/src/lala/service"
      }
    ]
  }
}
```

## Session Restore

For persistent projects, `tflow` tracks the last known working directory of each session.

Restore rules:

- only restore sessions for projects that still exist in `config.yaml`
- restore each session in its last known working directory
- if no session state exists for a persistent project, create a default `code` session in the project `workdir`
- volatile projects are never restored
- volatile project sessions are deleted with the owning terminal

CWD tracking:

- use tmux `#{pane_current_path}`
- do not add shell integration initially
- update cwd when the sidebar opens/closes, before switching sessions, and before shutdown where possible

Example tmux query:

```sh
tmux -L tflow display-message -p -t '<internal-tmux-session-name>' '#{pane_current_path}'
```

Restore command shape:

```sh
tmux -L tflow new-session -d -s '<internal-tmux-session-name>' -c '/last/known/path'
```

Known limitation:

- `#{pane_current_path}` is good enough initially
- it may not reflect remote SSH/container paths perfectly
- shell integration is intentionally avoided for now

## Startup Behavior

When `tflow` starts:

- load persistent projects from `config.yaml`
- load runtime/restore state from `state.json`
- clean up stale runtime state
- create a new volatile random animal project
- create a default `code` session in it
- attach to the `code` session
- ensure the sidebar can be toggled with `Ctrl+F`

First-start state example:

```text
project: otter
session: code
persistent: false
```

## Terminal Exit Behavior

When the owning terminal exits:

- if the current project is volatile, delete the project and all sessions inside it
- if the current project is persistent, do nothing to the project or sessions
- cleanup must be idempotent
- cleanup must never delete persistent projects or persistent sessions

## Sidebar Target

The sidebar is the main control surface.

Target layout:

```text
        [TFLOW]

       [Sessions]
-------------------
  [live] code
         ide
         agent
-------------------

-------------------
       [Projects]

  [live] tflow
         pro1
         pro2
         pro3
-------------------
```

Rules:

- `[TFLOW]` is a badge-style title
- `[Sessions]` shows only sessions of the current project
- `[Projects]` shows only persistent projects
- volatile projects are not listed in `[Projects]`
- `[live]` marks the current session/project
- selected-row highlighting must work when badges are present
- section separators should match the existing bordered TUI style
- keep the sessions list at a minimum visible height of five rows, unless fewer sessions exist
- selection flows happen inside the sidebar
- no separate menu
- no centered overlay except the `P` persist-project overlay
- help is hidden until `?` is pressed

## Keybindings

```text
j / k    move selection
Enter    switch session/project
t        new terminal
a        new agent
r        rename session
m        move session
p        switch project
P        persist current project
?        toggle help
Esc      cancel/close
Ctrl+C   close sidebar
```

Rules:

- actions use direct keybindings
- there is no `n`-prefixed command mode
- there is no `R` key
- there is no explicit project termination key
- killing the owning terminal is the project cleanup mechanism for volatile projects

## Help Layout

Help is hidden by default.

Show only after `?` is pressed:

```text
  t new terminal
  a new agent
  r rename session
  m move session
  p switch project
  P persist project
```

Rules:

- one action per line
- keep wording short
- pressing `?` again hides help

## Persistent Project Overlay

`P` opens a minimal centered overlay to persist the current volatile project.

Fields:

```text
name
workdir
agent-cmd
```

Behavior:

- only works when the current project is volatile
- writes the project to `~/.config/tflow/config.yaml`
- renames the current volatile project to the chosen persistent project name
- keeps existing sessions, including `code`
- marks the project as persistent
- adds it to the `Projects` section
- uses `workdir` as default cwd for new sessions
- uses `agent-cmd` for new agent sessions
- if the current project is already persistent, show a no-op status message
- overlay should be minimal, centered, and visually consistent with the Catppuccin TUI style

## Actions

### New Terminal Session

Key:

```text
t
```

Behavior:

- create a new terminal session in the current project
- ask for a session name inside the sidebar
- use the project `workdir` if configured
- otherwise use the current/default directory
- switch to the new session after creation

### New Agent Session

Key:

```text
a
```

Behavior:

- create a new agent session in the current project
- ask for a session name inside the sidebar
- use the project `agent-cmd`
- switch to the new session after creation
- if no agent command is available, show a clear error

### Rename Session

Key:

```text
r
```

Behavior:

- rename the selected session
- prompt inside the sidebar
- validate uniqueness inside the current project
- update runtime state
- update internal tmux backing name if needed

### Switch Project

Key:

```text
p
```

Behavior:

- switch only to persistent projects
- use hints mode inside the sidebar
- show sessions for the selected project
- restore persistent sessions if needed
- volatile projects are never valid switch targets

### Move Session

Key:

```text
m
```

Behavior:

- move selected session to another persistent project
- use hints mode inside the sidebar
- allow moving the last session out of a project
- source project remains valid even when empty
- update runtime state
- do not switch projects automatically

### Persist Project

Key:

```text
P
```

Behavior:

- persist the current volatile project
- ask for `name`, `workdir`, and `agent-cmd`
- write the project to `config.yaml`
- rename the current project to the chosen name
- keep existing sessions
- add the project to the persistent project list

## Hints Mode

Hints mode is used for project selection flows inside the sidebar.

Used by:

- `p` switch project
- `m` move session

Rules:

- hints are rendered inside the existing sidebar
- hints target persistent projects only
- hints highlight selectable prefixes
- hints support multi-character disambiguation
- hints are deterministic while active
- `Esc` cancels hints mode
- selecting a unique hint executes the action

Example:

```text
tf  tflow
te  test
p   prod
```

## UI States

The sidebar should support:
- normal
- help
- inline input
- hints
- confirm if needed for destructive future actions

Rules:

- create session uses inline input
- rename session uses inline input
- move/switch project use hints mode
- `P` persist project may use a minimal centered overlay
- avoid full-screen or separate menus

## Open Questions

- If multiple terminals attach to the same persistent project, should their selected sessions be tracked independently or globally?
- Should moving a session into a persistent project immediately write restore state, or only on sidebar close/session switch?
- Should `P` require all fields, or allow empty `workdir` / `agent-cmd`?
- Should a persistent project with no restore state always start with `code`, or stay empty until the user creates a session?

## Non-Goals

- Do not implement a tmux replacement.
- Do not expose tmux session names in the UI.
- Do not add mouse-driven UI.
- Do not add shell integration for cwd tracking initially.
- Do not support switching into volatile projects.
- Do not keep volatile projects after terminal exit.
- Do not keep `n`-prefixed command flows.
- Do not keep project/session tree layout as the target model.
