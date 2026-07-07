# tflow

`tflow` is a keyboard-first, tmux-backed session manager for project-scoped terminal and agent sessions.

It keeps the UI focused on one sidebar, hides tmux internals, and treats a project as the lifecycle boundary for its sessions.

## Design Decisions

- Sessions always belong to exactly one project.
- Terminating a project terminates all sessions inside it.
- Terminate the terminal in which `tflow` runs, will terminate also the project and all sessions in it.
- `tflow` bootstraps a random project and a `code` session on first start.
- The UI should expose logical names only; tmux session names stay internal.
- The sidebar is the main control surface. Project and session actions should happen there.
- The only way to persist a project, is to create it in the `tflow` configuration yaml `~/.config/tflow/config.yaml`
- Empty projects are valid.
- Help stays hidden until `?` is pressed.

## config.yaml
This file describes persistent projects, this file is only written by a human user.
If this file is not existing, there are no persistent projects


## Target Sidebar

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

Notes:

- `[]` indicates a badge-style label.
- Section separators should match the bordered session list style already used in the TUI.
- There should be no separate menu or overlay beyond the sidebar. Selection flows should use hints mode inside the existing UI.

## Open Tasks

- [ ] Fix session highlighting when a badge is present.
- [ ] Clean up the project and its sessions when the terminal running `tflow` exits unexpectedly.
- [ ] Keep the sessions list at a minimum visible height of five rows, unless fewer sessions exist.
- [ ] Add the project list to the sidebar so the layout matches the target design.
- [ ] Remove separate project overlays or extra menus and keep project switching inside the sidebar and hints flow.
- [ ] Show help only after `?` is pressed, one action per line, using this layout:

```text
  t new terminal
  a new agent
  r rename session
  R rename project
  m move session
  p switch project
  P create static project
```

## Done

- [x] Defined the project/session runtime model.
- [x] Namespaced tmux session ownership behind logical UI names.
- [x] Bootstrapped first-start behavior with a default `code` session.
- [x] Simplified persisted state around the project/session model.
- [x] Added reusable hints-mode primitives.
- [x] Reworked the TUI around sidebar-driven session management.
- [x] Implemented rename, create, move, and terminate session/project flows.
- [x] Updated tests and supporting docs for the current behavior.
