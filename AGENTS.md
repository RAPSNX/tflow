# Agent instructions

You are working on the `tflow`, a terminal project and session manager.
Primary this is a orchestrator for terminal sessions, that can be created in so called Projects.
It is possible to switch between sessions, or projects.

## Hard rules

- Keep edits small, reviewable, and testable.
- Create or use the codex-agent branch for your work.
- Use a worktree, verify the work is done, and merge it everytime on that branch.

## Workflow

1. Inspect the current branch and worktree state before editing.
2. Create or reuse a dedicated worktree for the change.
3. Make the code change in that worktree.
4. Run focused tests first.
5. Merge it into your codex-agent branch

## Commands

- Format: `gofmt -w <changed .go files>`
- Test all: `go test ./...`


## Coding guidance

- Prefer small, direct changes.
- Keep TUI state transitions explicit and easy to review.
- Handle API errors visibly and predictably.
- Do not introduce new dependencies unless necessary.
- Add or update tests for behavior changes.

## Subagents

Use subagents only when useful:

- `explorer`: before larger changes, to map relevant code paths.
- `reviewer`: after implementation, to review the diff.

Do not use subagents for trivial changes.
