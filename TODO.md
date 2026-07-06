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

## Current State Assessment

The current implementation does not match this target architecture yet.

What already exists:

- tmux-backed session lifecycle management
- Bubble Tea / Lip Gloss TUI framework and rendering styles
- persisted app config and project config files
- session creation for terminal and agent sessions
- session rename and move behavior
- menu toggling via `Ctrl+F` inside tmux
- tests covering startup, menu actions, state normalization, config parsing, and tmux integration

What exists but follows the old design:

- startup creates a temporary tmux session instead of a project with a `code` session
- the main UI is a tree of projects and sessions instead of a sidebar for the current project
- project creation, editing, directory config, protection, delete flows, and `:` command mode are built around the old model
- move-to-project uses typed prefix matching, not a reusable hints system
- project deletion reassigns sessions to `default` instead of terminating project-owned sessions
- `k9s` session support exists although it is not part of the current target design
- the runtime model still assumes a synthetic `default` project

What is missing for the target design:

- a project-centric runtime model where sessions only exist inside projects
- first-start bootstrap for random animal project creation plus a `code` session
- dedicated sidebar mode with the target keymap (`R`, `r`, `t`, `a`, `P`, `m`, `Ctrl+Q`, `Esc`)
- separate project overlay that excludes the current project
- reusable hints mode with deterministic multi-character hints
- project termination confirmation that kills all sessions in the project
- distinct visual treatment for current session versus selected session within the new sidebar model

## Open Questions

- Empty projects: keep the recommended behavior from above and allow them, or require at least one session per project?
- First-start attach behavior: after creating the initial `code` session, should `tflow` attach directly into that session, or still open a dedicated control session and switch into `code` from there?
- New project creation: the target keymap does not define a project creation action after first start. Should creating a project happen implicitly from move/switch flows, or is an explicit action still required?
- Agent command source: should `a` always use a global configured agent command, or should per-project overrides remain supported from the current config model?
- Legacy data migration: should existing `default`/temporary sessions and old persisted state be migrated automatically, or can incompatible state be reset once?
- Session naming scope: the target says names are unique within a project. tmux session names are currently global, so either tmux naming must be namespaced internally or the app must continue enforcing global uniqueness.

## Implementation Task List

- [ ] Task 1: Define the target runtime data model.
  Replace `default`/temporary-session assumptions with explicit current-project/current-session state, project-owned sessions, and a clear representation for sidebar mode, overlay mode, and hints mode.

- [ ] Task 2: Define the tmux naming and ownership strategy.
  Decide how project-scoped session names map onto globally unique tmux session names and document the persistence/migration approach before changing behavior.

- [ ] Task 3: Bootstrap first-start behavior.
  Replace `prepareStartup` startup flow so first start creates a random animal project plus a `code` session, persists it, and attaches consistently to the intended initial runtime state.

- [ ] Task 4: Simplify persisted state around the new architecture.
  Refactor `appState`, load/save logic, and project config handling so persisted data reflects the new project/session ownership model and drops obsolete tree-specific state where possible.

- [ ] Task 5: Introduce reusable hints-mode primitives.
  Extract deterministic hint generation and typed-selection handling into focused logic with tests, independent of project switching and session moving.

- [ ] Task 6: Rework the main TUI into sidebar mode.
  Replace the tree menu interaction model with a current-project sidebar that shows only that project's sessions and preserves clear selection/current-session styling.

- [ ] Task 7: Implement the target sidebar keymap.
  Add `R`, `r`, `t`, `a`, `P`, `m`, `Ctrl+Q`, and `Esc`; remove or retire old-tree actions that are no longer part of the target workflow.

- [ ] Task 8: Implement the project overlay.
  Add a dedicated overlay for project switching that excludes the current project and shows a deliberate empty state when there are no alternatives.

- [ ] Task 9: Implement move-session via hints mode.
  Replace prefix matching with hints-based project selection and preserve the agreed empty-project behavior when moving the last session out.

- [ ] Task 10: Implement project termination.
  Add explicit confirmation, show the project name, kill all project sessions, remove project runtime state, and remove persisted project state.

- [ ] Task 11: Align session creation flows with the target design.
  Keep terminal and agent creation, remove or isolate `k9s` from the main UI path, and ensure new sessions are always created inside the current project.

- [ ] Task 12: Remove or isolate obsolete features.
  Retire command mode, project YAML editing from the main flow, protection semantics, collapse/expand tree behavior, and other interactions that no longer belong in the target UX.

- [ ] Task 13: Update tests around the new behavior.
  Replace old tree/menu expectations with focused tests for bootstrap, sidebar toggling, overlay selection, hints generation, move semantics, and project termination.

- [ ] Task 14: Refresh docs after behavior convergence.
  Update `README.md` and any user-facing docs only after the implementation matches this target design.
