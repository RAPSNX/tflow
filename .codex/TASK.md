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
      `internal/store/project_config.go`. `AgentBinary` is not confined to
      those two files, though -- it is manually copied or compared at
      every persistence projection, and `GitBinary` needs the same
      counterpart at each: `internal/store/state_codec.go`'s encode and
      decode (both construct their project literal field-by-field),
      `internal/ui/model.go`'s `currentState`/project-config
      reconstruction, and every `AgentBinary` reference inside
      `internal/ui/helpers.go` (`mergeStateProjectFields`'s scalar-diff
      check and its two-project-literal construction, plus the other
      `storedProject{...}` literals in the reinsert/ensure helpers). Grep
      the codebase for `AgentBinary` and add a `GitBinary` counterpart
      everywhere it appears; a spot missed here means a custom
      `git-binary` can be saved, compile, and then silently get dropped or
      overwritten on the next write or restart. `materializeCommand`
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
      `internal/ui/project_settings.go`'s `handleProjectEditorFinished`
      gains a `git-binary` counterpart to the existing
      `isBareExecutableToken(agentBinary)` check (around
      `project_settings.go:287`), rejecting an invalid value (e.g.
      `git --foo`) before it is ever persisted, the same way an invalid
      `agent-binary` is today -- without it, an invalid value compiles
      into stored state and only fails later at materialization. That
      file also gains a `provisionGitSession` alongside
      `provisionAgentSession` (`project_settings.go:359-382`) -- same
      shape, but always labeled `git` with no numbered fallback, since
      exactly one is ever allowed -- invoked wherever a project's
      `git-binary` setting is saved. The `e` YAML editor's accepted-key
      allowlist gains `git-binary` next to `workdir`/`agent-binary`. Add
      table-driven tests for the new dedup/label-lock validation, the
      `MoveSession` rejection, `provisionGitSession`, the `git-binary`
      rejection, a round-trip through the codec, and a concurrent-merge
      test that a saved custom `gitBinary` survives
      `mergeAppStates`/`mergeStateProjectFields` the way agent-binary
      already does; hand-verify via `scripts/tmux-verify.sh` that a
      project with a custom `git-binary` launches it on selecting the git
      session, that the setting survives a restart, and that renaming
      that session is rejected.

- [ ] Make command mode (the `Ctrl+F` wait state, `prefixTable` in
      `internal/tmux/control.go`) reach every dialog-driving sidebar
      action directly, the same way `h`/`l`/`g` already reach
      navigate-prev/navigate-next/jump-git without opening the popup at
      all. Add a `MenuMode*` constant (`internal/tmux/types.go`) and a
      `cmd/tflow/main.go` subcommand for each of: create session, create
      project, switch project, rename session, delete session, move,
      rename project, delete project, and edit project settings --
      mirroring `open-quit`'s existing `TFLOW_MENU_MODE`/`openMenu`
      wiring.

      `openMenu` (`internal/ui/lifecycle.go`) cannot invoke a `begin*`
      method directly for these new modes the way it sets `menu.commandMode`
      for `MenuModeCommand`: `begin*` methods read `m.sessions`,
      `m.selectedSession`, and `m.selectedProject`, none of which are
      populated until `sessionsLoadedMsg` arrives and `syncSelection()`
      runs inside `updateMessage` (`internal/ui/messages.go`) -- which
      only happens once `runProgram`'s Bubble Tea loop starts, strictly
      after `openMenu` would have already called `begin*` on an empty
      model. Instead, stash the requested mode as a pending-action field on
      `menu` before calling `runProgram`, then in `updateMessage`'s
      `sessionsLoadedMsg` case, immediately after `syncSelection()`, check
      that field and invoke the corresponding `begin*` method there,
      applying both the resulting model mutations and its returned
      `tea.Cmd` (`editProject`'s `ExecProcess` command included) through
      the normal `Update` return rather than discarding it.

      Route each new mode's pending action to the same `begin*` method the
      popup's own keypress already calls (`m.beginRename()`,
      `m.beginSessionMove()`, `m.startSessionCreate()`,
      `m.beginProjectCreate()`, `m.beginProjectSwitch()`,
      `m.beginDelete()`, `m.beginProjectRename()`,
      `m.beginProjectDelete()`, `m.editProject()` -- see
      `internal/ui/keys.go:41-60`), so both entry points share one code
      path; project rename and delete are separate subcommands from their
      session counterparts, matching the sidebar's own distinct `R`/`D`
      bindings.

      Bind each new subcommand's shell command at `prefixTable` in
      `internal/tmux/control.go` using `parts` (both `TFLOW_CURRENT_SESSION`
      and `TFLOW_CURRENT_CLIENT`), the way `jumpGitShell` actually does --
      not `sessionOnlyPart`, which omits the client and, per the existing
      comment on `navigatePrevShell`/`navigateNextShell`/`jumpGitShell`
      just above it, would make `Manager.SwitchClient`/`openMenu` fall back
      to an unscoped client lookup that can target the wrong client when
      more than one is attached -- exactly the client-scoping bug these
      destructive dialog actions can't afford.

      Leave `j`/`k` selection and `Enter`-to-switch popup-only -- there is
      nothing to move through or select before the sidebar's list is
      visible. Add tests covering each new `MenuMode*` value in the
      `openMenu` switch, including that its pending action fires only
      after `sessionsLoadedMsg`/`syncSelection` and that its returned
      command is not dropped; hand-verify via `scripts/tmux-verify.sh`
      that, for at least create-session, rename session, delete session,
      rename project, and delete project, pressing the bound key from
      command mode opens the sidebar already inside that flow (not the
      plain session list) with the correct session/project selected, that
      finishing or cancelling it behaves the same as reaching it through
      the popup's own keypress, and that it targets the pressing client
      when a second client is attached.
