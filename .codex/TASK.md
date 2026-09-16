# Open tasks

Unfinished work derived from `.codex/ARCHITECTURE.md`, as `- [ ]` items.
Delete an item once implemented and verified. No finished items, notes, or
history.

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

- [ ] Initialize the attention watermark for sessions tflow did not itself
      create (`internal/ui/attention.go`, `AttentionScan`): a session with
      no `@tflow-visited-at` marker -- upgraded from before this feature,
      or created outside tflow -- reads `VisitedAt` as zero, so its entire
      pre-existing activity history compares as newer and gets flagged on
      the first scan even though nothing happened since tflow started
      watching it. Stamping every zero-watermark session unconditionally
      at scan time is not the fix: `AttentionScan` cannot tell that case
      apart from a session tflow *just* materialized and that legitimately
      has real fresh, unvisited output (`TestAttentionScanMarksUnvisitedSessionsWithFreshActivity`
      depends on exactly that immediate flagging). Fix direction: stamp a
      baseline watermark for every session already present at startup, in
      one pass before the recurring scan begins (not per-tick), so only
      genuinely pre-existing sessions get the baseline and anything
      materialized afterward keeps today's immediate-flag behavior;
      confirm the startup ordering against a real tmux server
      (`scripts/tmux-verify.sh`) before relying on it.
