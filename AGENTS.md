# Repository Guidelines

## Project Structure

This is a small Go module with its current entry point in `cmd/tflow/main.go`.

- Put reusable application code under `internal/`.
- Avoid `pkg/` unless the project intentionally exposes public reusable APIs.
- Place tests next to the code they cover using `*_test.go`.

## Development Commands

- `go run ./cmd/tflow`: run the application locally.
- `go test ./...`: run all tests.
- `go build ./...`: compile all packages.
- `gofmt -w <files>`: format changed Go files.

## Go Style

- Use standard Go formatting and idioms.
- Use short, lowercase package names.
- Use `PascalCase` for exported identifiers and `camelCase` for unexported identifiers.
- Keep packages and files small and focused.
- Split code by responsibility when it improves clarity or testability.
- Do not hide unrelated behavior in one package file.

## Testing

Use Go's built-in `testing` package by default.

- Name tests clearly, for example `TestParserHandlesEmptyInput`.
- Prefer table-driven tests for input/output behavior.
- Add tests for new logic and bug fixes.
- Run `go test ./...` before finishing work.

## Architecture and Tasks

- Read `AGENTS.md` before editing.
- Read `.codex/ARCHITECTURE.md` before changing behavior.
- Treat `.codex/ARCHITECTURE.md` as the source of truth for intended behavior, including the persistent state path.
- Use `.codex/TASK.md`, which must be derived from the architecture, as the implementation checklist.
- After completing implementation work, always check off every verified completed item in `.codex/TASK.md`; leave all other items unchecked.
- If the task list conflicts with the architecture, stop and ask.
- Implementation detail in the task list is allowed when it does not conflict with the architecture.
- Do not implement features outside the task list unless the user explicitly asks.
- If a requested implementation needs behavior not defined by the architecture, stop and ask.
- Do not introduce a user-edited configuration file.
- Always check `README.md` for user-facing changes; update it only when something needs documenting, and keep it minimal and aligned with the architecture.

## Git Workflow

- If the current worktree has pre-existing changes, stop immediately.
- Work in a dedicated git worktree on a task branch created from `main`; when updating an existing PR, continue on that PR's branch.
- When addressing pull request review comments, always verify each comment against the architecture and codebase, and resolve the review thread in GitHub once fixed.
- Keep changes focused and avoid unrelated rewrites.
- End every change in a meaningful commit.
- Push the branch and open or update a published pull request.
