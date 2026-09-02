# tflow open implementation checklist

This checklist contains only unfinished work derived from
`.codex/ARCHITECTURE.md`. Remove an item after its implementation and
verification are complete.

## P0: Final-session deletion reliability

* [ ] Reproduce the documented sidebar `d` final-session deletion flow with the full fallback handoff.
* [ ] Keep the fallback configured and switch the originating client to it before killing the deleted persistent sessions or removing their metadata.
* [ ] Preserve the active client and report the original error when fallback creation, switching, deletion, or metadata cleanup fails.
* [ ] Add lifecycle tests for successful fallback handoff and each failure boundary.

## P1: Contextual session navigation and top bar

* [ ] Bind tmux prefix `h` and `l` to internal previous/next session workers without expanding public CLI help.
* [ ] Navigate with wraparound in the sidebar's current context: ordered persistent sessions in the active project, or volatile sessions of the current instance.
* [ ] Keep navigation client-scoped, prevent cross-project and foreign-instance targets, lazily materialize missing persistent targets, and skip sidebar-only dead-session cleanup.
* [ ] Maintain target-only derived tmux metadata for previous, active, and next top-bar entries without JSON writes or a background refresher.
* [ ] Render the top bar with contextual previous/active/next entries, showing only the active entry for a one-session context.
* [ ] Test ordering, wraparound, one-session behavior, lazy targets, client ownership, and command/write limits.

## P1: Typed persistent sessions

* [ ] Extend persistent project and session state with optional `agentBinary`, session `type`, and agent `command`; interpret records without a type as terminal sessions without forcing a migration.
* [ ] Validate allowed types, agent-command requirements, one agent session per project, and exact-label uniqueness while preserving unknown-field compatibility.
* [ ] Create ordinary new projects with lazy `code` terminal and `git` sessions in that order; preserve existing projects and volatile-session promotions without presets.
* [ ] Keep `n` as normal terminal creation and materialize terminal, `lazygit`, and captured agent-executable sessions in the project workdir.
* [ ] Fail clearly and without mutation when `lazygit` or an agent executable is unavailable.
* [ ] Extend the temporary YAML project settings document with executable-only `agent-binary`; save adds or updates the lazy agent session, and clearing the setting retains its captured agent command.
* [ ] Test old-state compatibility, schema validation, new-project records, promotion preservation, settings updates and clearing, agent-label collision suffixes, materialization, and executable failures.

## P1: Typed visual identity

* [ ] Render blue `>_ CODE`, teal `⎇ GIT`, and yellow `✦ AGENT` chips in every sidebar row and top-bar session entry.
* [ ] Keep type chips visible in selected rows; render independent teal live and red attention indicators without overwriting type identity.
* [ ] Test chip text, color/style selection, selected rows, active rows, attention rows, and top-bar consistency.

## P1: Session attention

* [ ] Enable tmux activity monitoring for every managed session and install internal alert and client-session-change hooks.
* [ ] Set attention only for output in a session not being visited, clear it when any client visits that session, and show it in the sidebar and top bar.
* [ ] Keep attention session-scoped and runtime-only: never write it to JSON and allow it to disappear after a tmux restart.
* [ ] Test hook commands, inactive activity, visit clearing, indicator rendering, and persistence isolation.

## P1: Published-module verification

* [ ] Install `github.com/rapsnx/tflow/cmd/tflow@latest` through the module proxy into a temporary location and verify `tflow version` matches the current published release.
