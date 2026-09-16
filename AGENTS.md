# Repository Guidelines

This is a small Go module with entry point `cmd/tflow/main.go`.

## Sources of truth

* `.codex/ARCHITECTURE.md` defines tflow's intended end state, including the
  persistent state path. Read it before changing behavior.
* `.codex/TASK.md` contains only unfinished, architecture-derived work, as
  `- [ ]` items. Remove an item once it is implemented and verified; leave
  remaining items unchecked.
* `README.md` documents implemented user-facing behavior. Check it for every
  user-facing change and update it minimally when needed; it may differ from
  unimplemented end-state work.
* Always ask when requirements, scope, or intended behavior are unclear,
  including a conflict between the task list and the architecture or
  undefined behavior. Do not guess, and do not implement work outside the
  task list unless explicitly requested.
* Do not introduce a user-edited configuration file.

## ARCHITECTURE.md is the goal state

`.codex/ARCHITECTURE.md` describes tflow as if it were already fully built,
in the present tense. It is never a place for history, changelogs, bug logs,
debugging traces, verification evidence, TODOs, or comparisons with an
earlier design ("instead of", "rather than", "no longer", "previously", "was
verified", "matters because"). A standing constraint may be stated, with at
most one clause of rationale ("compares against the visit watermark so a
stale background-window flag never re-sets the marker"); the story of how
that constraint was discovered belongs in a commit message or PR
description, not here. Open work belongs in `.codex/TASK.md`. When the
design changes, rewrite the affected text to the new end state directly;
never append an amendment.

## Development

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

## Verification

* Run `go test ./...` before finishing.
* Before finishing any change that touches tmux control mode, key bindings,
  the popup, or the status bar, verify it by hand in a real-like environment:
  build the binary, run it under a dedicated tmux socket other than the
  default, attach it under `script` or a real terminal, and drive it with
  `tmux -L <socket> send-keys` / `capture-pane`. Never run a manual
  verification build against the default `tflow` socket; it collides with
  any real tflow instance already running on the machine.
* Set up and tear down that isolated socket only through
  `scripts/tmux-verify.sh` (`eval "$(scripts/tmux-verify.sh setup)"` sets
  `ROOT`/`TMUX_TMPDIR`/`XDG_STATE_HOME`/`SOCK`; `scripts/tmux-verify.sh
  cleanup "$ROOT"` tears it down) -- never hand-roll
  `mktemp`/`export`/`rm -rf` for this. A past incident (see git history)
  ran a bare `rm -rf "$TMUX_TMPDIR"` with the variable unset in a shell
  that hadn't re-sourced it; it fell back to an ambient value and wiped
  the user's XDG runtime directory (Wayland, D-Bus, and PipeWire sockets),
  forcing a session restart. The script's `cleanup` refuses to run unless
  its argument is a real, existing, canonicalized path under its own
  `${TMPDIR:-/tmp}/tmux-verify.*` naming, so the same class of mistake
  can't resolve to a dangerous path even if it recurs.

## Git workflow

* Stop immediately if the worktree has pre-existing changes.
* Use a dedicated worktree and task branch from `main`; continue on the PR
  branch when updating an existing PR.
* Keep changes focused, end them in a meaningful commit, push the branch,
  and open or update a published pull request.
* Verify review comments against the architecture and code, then resolve
  fixed GitHub threads.

## PR reviews

An upstream bot reviews every published PR. After pushing, poll for it
yourself instead of spawning an agent: wait 30 seconds, then check for
unresolved review threads and for the PR description's own reactions
(👀 = review in progress or requested, 👍 = review done); repeat until 👍 is
present and no unresolved threads remain. Verify each finding against the
architecture and code, fix it, push, and poll again.

## No sub-agents

Do not spawn sub-agents for work in this repository. A spawn re-reads the
whole context from scratch, which costs more than doing the step inline —
including polling for PR reviews above.
