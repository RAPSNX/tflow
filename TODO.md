# tflow

## Overview

`tflow` is a minimal, keyboard-first terminal session manager with an `nvim`-style workflow.

It organizes terminal sessions into projects. A session can only exist inside a project, and each project acts as the lifecycle boundary for its sessions.

On first start, `tflow` creates:

- a project with a random animal name
- a default session named `code`

The main interaction model is the sidebar mode, which can be toggled with `Ctrl+F`.

## Core Concepts

### Project

A project groups related terminal sessions.

Rules:

- project names are unique
- a project owns its sessions
- when a project is terminated, all sessions inside it are killed
- the current project is visibly marked in the UI
- the current project is not shown in the project-switch overlay

### Session

A session is a terminal inside a project.

Rules:

- a session can only exist inside a project
- session names are unique only within their project
- different projects may contain sessions with the same name
- the current session and selected session should be visually distinguishable

## Sidebar Mode

`Ctrl+F` toggles sidebar mode.

Sidebar mode shows the sessions of the current project and provides project/session actions.

### Keybindings

| Key | Action |
|---|---|
| `R` | Rename the current project |
| `r` | Rename the selected session |
| `t` | Create a new terminal session |
| `a` | Create a new agent session |
| `P` | Open the project overlay |
| `m` | Move the selected session to another project |
| `Ctrl+Q` | Terminate the current project |
| `Esc` | Close sidebar mode |

### Project Termination

`Ctrl+Q` terminates the current project.

This must require confirmation.

The confirmation prompt must clearly show the project name.

Termination means:

- kill all sessions in the project
- remove the project from runtime state
- remove persisted project state if persistence is implemented

## Project Overlay

`P` opens the project overlay.

The overlay shows all projects except the current project.

Selecting a project switches into that project.

If no other projects exist, the overlay should show an empty-state message instead of opening a broken selection view.

Project selection uses hints mode.

Example:

Given projects:

- `tiger`
- `mouse`

The overlay highlights selectable hints:

- `t` for `tiger`
- `m` for `mouse`

Typing `Pt` selects and enters the `tiger` project.

## Move Session

`m` moves the selected session to another project.

The target project is selected using hints mode.

Open design decision:

- define whether moving the last session out of a project is allowed
- define whether empty projects are valid

Recommended behavior:

- empty projects are allowed
- moving the last session out of a project keeps the source project alive
- terminating a project is only done explicitly via `Ctrl+Q`

## Hints Mode

Hints mode is inspired by Alacritty hints.

When an action enters hints mode, the TUI highlights characters for selectable resources.

The typed hint selects the matching resource.

Hints mode must support multi-character hints to avoid collisions.

Example:

Given projects:

- `tiger`
- `table`
- `mouse`

Possible hints:

- `ti` for `tiger`
- `ta` for `table`
- `m` for `mouse`

Hints should be deterministic and stable while the overlay is open.

## Acceptance Criteria

- On first start, create a project with a random animal name and one session named `code`.
- `Ctrl+F` toggles sidebar mode.
- Sidebar mode shows sessions of the current project.
- The current project is visibly marked.
- The current session and selected session are visually distinguishable.
- `R` renames the current project.
- `r` renames the selected session.
- `t` creates a new terminal session using the user shell.
- `a` creates a new agent session using the configured agent command.
- `Ctrl+Q` asks for confirmation before terminating the current project.
- Project termination kills all project sessions and removes the project state.
- `P` opens a project overlay excluding the current project.
- `m` moves the selected session to another project using hints mode.
- Hints mode supports multi-character hints when prefixes collide.
