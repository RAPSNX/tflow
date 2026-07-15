# Repository Guidelines

## Project Structure

This is a small Go module with the main entry point in `cmd/main.go`.

- Put reusable application code under `internal/`.
- Avoid `pkg/` unless the project intentionally exposes public reusable APIs.
- Place tests next to the code they cover using `*_test.go`.

## Development Commands

- `go run ./cmd`: run the application locally.
- `go test ./...`: run all tests.
- `go build ./...`: compile all packages.
- `gofmt -w <files>`: format changed Go files.

## Go Style

- Use standard Go formatting and idioms.
- Use short, lowercase package names.
- Use `PascalCase` for exported identifiers.
- Use `camelCase` for unexported identifiers.
- Keep packages small and focused.
- Split behavior into meaningful packages when it improves clarity or testability.

## Testing

Use Go’s built-in `testing` package by default.

- Name tests clearly, e.g. `TestParserHandlesEmptyInput`.
- Prefer table-driven tests for input/output behavior.
- Add tests for new logic and bug fixes.
- Run `go test ./...` before finishing work.

- all changes should always end in in meaningfull commits
- if the actual branch has changes stop your work imediatley
- alwyas work in a git worktree
- commit everything into a branch

## File Size and Refactoring

- Keep Go files small and focused.
- Avoid large multi-purpose files.
- Prefer files with lower line count.
- Split by responsibility, not randomly.
- Do not hide unrelated behavior in one package file.

## Agent Instructions

- Read `AGENTS.md` before editing.
- Read `.codex/ARCHITECTURE.md` before changing behavior.
- Use `.codex/TASK.md` as the current implementation checklist.
- This repo uses one primary agent and one sub-agent.
- `.codex/TASK.md` section tags define which agent owns each section.
- `.codex/ARCHITECTURE.md` describes the target state and is the source of truth for intended behavior.
- `.codex/TASK.md` must be derived from `.codex/ARCHITECTURE.md`.
- If `.codex/TASK.md` conflicts with `.codex/ARCHITECTURE.md`, stop and ask.
- If `.codex/TASK.md` adds implementation detail without conflict, complete it.
- Do not implement features not listed in `.codex/TASK.md` unless explicitly asked.
- Do not introduce a user-edited config file; persistent data belongs in `$XDG_STATE_HOME/tflow/store.json`.
- If implementation requires behavior not defined in `.codex/ARCHITECTURE.md`, stop and ask.
- Keep changes focused and avoid unrelated rewrites.
- Always create new branch for your work from main
- Never work on main, always commit your work !!
- Always !! Commit the refactor, push the branch, and open a published PR. !!!
