# AGENTS.md

## Scope

These instructions apply to the full repository.

## Work Style

- Read `DESIGN.md` before changing behavior.
- Use `TASK.md` as the implementation checklist.
- Keep changes focused and reviewable.
- Do not rewrite unrelated code.
- Preserve the existing project style unless a task explicitly requires a refactor.
- Prefer small, clear functions over broad abstractions.
- Avoid changing public behavior that is not mentioned in `DESIGN.md` or `TASK.md`.

## Source of Truth

- `DESIGN.md` defines the target architecture and behavior.
- `TASK.md` defines the implementation order and checklist.
- If `DESIGN.md` and `TASK.md` conflict, stop and document the contradiction.
- If implementation requires a design decision not covered by `DESIGN.md`, stop and document the question.

## Go Code

- Write idiomatic Go.
- Keep errors explicit and actionable.
- Avoid package-level mutable state unless already part of the existing design.
- Keep tmux-specific behavior behind internal abstractions.
- Do not expose internal tmux session names in UI-facing code.
- Add tests for new behavior where practical.

## TUI Behavior

- The sidebar is the main control surface.
- Avoid adding new full-screen menus.
- Avoid centered overlays except for the `P` persist-project overlay.
- Keep interaction keyboard-first.
- Keep help hidden unless `?` is active.
- Keep UI labels short and consistent.

## Persistence

- `config.yaml` is the source of truth for persistent projects.
- `state.json` is runtime and restore state only.
- Volatile projects must not be restored after terminal exit, logout, reboot, or tmux server loss.
- Persistent projects and sessions must survive terminal exit unchanged.

## Before Finishing

- Update `TASK.md` checkboxes for completed work.
- Update `README.md` if user-facing behavior changed.
- Run formatting.
- Run relevant tests.
- Summarize changed files, tests run, and remaining open items.
