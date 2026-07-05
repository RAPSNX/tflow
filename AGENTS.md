## Rules
- Keep edits small, reviewable, and testable.
- Create or use the codex-agent branch for your work.
- Verify always to work cleanly, finished work should always end with a cleanup and a merge into the `codex-agent` branch.
- Use a worktree, verify the work is done, and merge it everytime on that branch.
- The `TODO.md` contains all open & finished tasks for features, bugs, changes or issues.
    - When working on these, always mark them as done acrodingly.

## Workflow

1. Inspect the current branch and worktree state before editing.
2. Create or reuse a dedicated worktree for the change.
3. Make the code change in that worktree.
4. Run focused tests first.
5. Merge all of the work into your branch `codex-agent`
6. Cleanup any other branch or worktree

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

Use subagents selectively. Do not invoke them for trivial changes or when the main agent can complete the task efficiently.

### Available Subagents

- `explorer`: Use before larger or unfamiliar changes to identify relevant files, code paths, dependencies, and existing patterns.
- `worker`: Use to delegate clearly scoped work that can be done in parallel, such as an isolated feature, bug fix, refactor, or investigation.
- `reviewer`: Use after implementation to review the diff, check for regressions, and identify missed edge cases.

### Usage Guidelines

Prefer subagents when they reduce risk, improve coverage, or allow meaningful parallelization.

Avoid subagents when the task is small, obvious, purely mechanical, or faster to complete directly.
