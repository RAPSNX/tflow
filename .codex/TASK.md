# Open tasks

Unfinished work derived from `.codex/ARCHITECTURE.md`, as `- [ ]` items.
Delete an item once implemented and verified. No finished items, notes, or
history.

- [ ] Fix `mergeStateProjectFields` (`internal/ui/helpers.go`) so that when
      a project was removed from `latest` by a concurrent save, the
      not-found fallback reinserts the whole `desired` project, including
      its sessions -- not just scalar fields via `ensureStateProject`,
      which hardcodes an empty `Sessions` slice. Today, saving only a
      scalar field (e.g. `agent-binary`) on a project another instance
      just deleted silently drops every unchanged session, because the
      later per-session merge loop skips sessions that already match
      `base`. Add a regression test in `internal/ui/helpers_test.go`
      mirroring `TestMergeAppStatesPreservesConcurrentAgentBinaryDuringWorkdirChange`.

- [ ] Guard `.github/workflows/release.yml` against `v2+` tags before the
      `release` job runs. `go.mod`'s module path
      (`github.com/rapsnx/tflow`) has no `/vN` suffix, so per Go's
      major-version-suffix rule a `v2.x.x`+ tag can publish a release
      whose own `verify-published-module` job is guaranteed to fail
      (`go install .../tflow@v2.x.x` can never resolve). Either reject
      unsupported major-version tags early with a clear failure, or
      migrate the module path when v2 is actually intended.

- [ ] Close the same-second activity race in `AttentionScan`
      (`internal/ui/attention.go`): `MarkSessionVisited` stamps
      `VisitedAt` from tmux's `window_activity` (1-second resolution), and
      the scan compares fresh `window_activity` against it with strict
      `>`. Output produced in the same wall-clock second as the visit (and
      never again later) reads as equal to `VisitedAt` forever, so it is
      never flagged -- only the event-driven `alert-activity` hook
      (`SessionActivity`) catches that case today, and only on tmux builds
      where it fires. Fix direction: add
      `SessionActivityFlags() (map[string]bool, error)` reading
      `#{window_activity_flag}` (mirrors `SessionActivityTimestamps`),
      reset per-window at visit time via `set-window-option
      monitor-activity off` then `on` (batched into `MarkSessionVisited`'s
      existing tmux call with the `runBatch` helper already used by
      `internal/tmux/popup.go`'s `openMenu`), and treat a session as fresh
      when `activityAt > VisitedAt` **or** its flag is set. The
      monitor-activity reset idiom is unverified in this codebase --
      confirm it actually clears the flag against a real tmux server
      (`scripts/tmux-verify.sh`) before relying on it.
