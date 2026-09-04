# Repository Guidelines

## Development

This is a small Go module with entry point `cmd/tflow/main.go`.

* Put reusable application code in focused, short lowercase packages under
  `internal/`; use `pkg/` only for intentional public APIs.
* Follow standard Go idioms and naming, keep responsibilities separate, and
  format changed Go files with `gofmt`.
* Place `*_test.go` beside covered code, use `testing`, prefer table-driven
  cases, and cover new logic and bug fixes.

Commands:

* `go run ./cmd/tflow`: run locally
* `go test ./...`: run tests
* `go build ./...`: compile all packages
* `gofmt -w <files>`: format changed Go files

## Sources of truth

* Read this file before editing and `.codex/ARCHITECTURE.md` before changing
  behavior.
* `.codex/ARCHITECTURE.md` defines the intended end state, including the
  persistent state path.
* `.codex/TASK.md` contains only unfinished, architecture-derived work. Remove
  verified items when completed; leave remaining items unchecked.
* Stop and ask if the task list conflicts with the architecture or requested
  behavior is undefined. Do not implement work outside the task list unless
  explicitly requested.
* Do not introduce a user-edited configuration file.
* `README.md` documents implemented user-facing behavior. Check it for every
  user-facing change and update it minimally when needed; it may differ from
  unimplemented end-state work.

## Git workflow

* Stop immediately if the worktree has pre-existing changes.
* Use a dedicated worktree and task branch from `main`; continue on the PR
  branch when updating an existing PR.
* Verify review comments against the architecture and code, then resolve fixed
  GitHub threads.
* Keep changes focused, end them in a meaningful commit, push the branch, and
  open or update a published pull request.
* Run `go test ./...` before finishing.
