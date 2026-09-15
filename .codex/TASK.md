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

- [ ] Add a configurable, locked `git` session type, mirroring the existing
      `agent` pattern: `Project.GitBinary` in
      `internal/store/state_schema.go` (+ `storedProject.GitBinary`,
      `omitempty`), normalized alongside `AgentBinary` in
      `internal/store/state_normalize.go` and
      `internal/store/project_config.go`. `materializeCommand`
      (`internal/ui/session_type.go:16-19`) resolves the git binary from
      the owning project, falling back to `lazygit` when unset, instead of
      the current hardcoded literal. `ValidateAppState`
      (`internal/store/state_normalize.go`) gains a `git` counterpart to
      the existing `seenAgent` dedup check (one `git` session per project)
      and rejects a `git`-typed session whose label isn't exactly `git`;
      `internal/store/move.go` inherits both rejections for free once that
      validation exists (it already relies on `ValidateAppState` for the
      agent case). `internal/ui/actions.go`'s `beginRename` refuses to
      rename a `git`-type session, with a clear `m.status` message.
      `internal/ui/project_settings.go` gains a `provisionGitSession`
      alongside `provisionAgentSession` (`project_settings.go:359-382`) --
      same shape, but always labeled `git` with no numbered fallback,
      since exactly one is ever allowed -- invoked wherever a project's
      `git-binary` setting is saved. The `e` YAML editor's accepted-key
      allowlist gains `git-binary` next to `workdir`/`agent-binary`. Add
      table-driven tests for the new dedup/label-lock validation, the
      `MoveSession` rejection, `provisionGitSession`, and
      `materializeCommand` resolving a custom `gitBinary`; hand-verify via
      `scripts/tmux-verify.sh` that a project with a custom `git-binary`
      launches it on selecting the git session and that renaming that
      session is rejected.

- [ ] Make command mode (the `Ctrl+F` wait state, `prefixTable` in
      `internal/tmux/control.go`) reach every dialog-driving sidebar
      action directly, the same way `h`/`l`/`g` already reach
      navigate-prev/navigate-next/jump-git without opening the popup at
      all. Add a `MenuMode*` constant (`internal/tmux/types.go`) and a
      `cmd/tflow/main.go` subcommand for each of: create session, create
      project, switch project, rename, delete, move, and edit project
      settings -- mirroring `open-quit`'s existing
      `TFLOW_MENU_MODE`/`openMenu` wiring. In
      `internal/ui/lifecycle.go`'s `openMenu` mode switch, route each new
      mode to the same `begin*` method the popup's own keypress already
      calls (`m.beginRename()`, `m.beginSessionMove()`,
      `m.startSessionCreate()`, `m.beginProjectCreate()`,
      `m.beginProjectSwitch()`, `m.beginDelete()`, `m.editProject()` --
      see `internal/ui/keys.go:41-60`), so both entry points share one
      code path. Bind each new subcommand's shell command at `prefixTable`
      in `internal/tmux/control.go`, reusing `sessionOnlyPart` for
      `TFLOW_CURRENT_SESSION` the way `jumpGitShell` does. Leave `j`/`k`
      selection and `Enter`-to-switch popup-only -- there is nothing to
      move through or select before the sidebar's list is visible. Add
      tests covering each new `MenuMode*` value in the `openMenu` switch;
      hand-verify via `scripts/tmux-verify.sh` that, for at least
      create-session, rename, and delete, pressing the bound key from
      command mode opens the sidebar already inside that flow (not the
      plain session list), and that finishing or cancelling it behaves
      the same as reaching it through the popup's own keypress.
