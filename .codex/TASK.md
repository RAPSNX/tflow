# tflow open implementation checklist

Only unfinished work derived from `.codex/ARCHITECTURE.md` belongs here.
Remove each item after implementation and verification.

## P1: Command mode, contextual navigation, and top bar

* [ ] Bind fixed, one-shot `Ctrl+Space` command mode (`h` previous, `l` next, `o` overview), including cancellation, internal workers, and removal of the global `Ctrl+F` binding without expanding public help.
* [ ] Navigate with wraparound in stored project order or current-instance volatile tmux order; remain client-scoped, never cross contexts, lazily materialize persistent targets, and skip sidebar-only dead-session cleanup.
* [ ] Maintain target-only derived metadata and render previous/active/next top-bar entries, reducing a one-session context to its active entry; refresh the originating client's active or selected target after context-changing rename, move, creation, settings, deletion, and post-switch cleanup paths without rewriting unrelated sessions.
* [ ] Test bindings, cancellation, overview, ordering, wraparound, lazy targets, one-session behavior, ownership isolation, mutation and cleanup refreshes, and tmux command/write limits.

## P1: Typed persistent sessions

* [ ] Add optional project `agentBinary`, session `type`, and agent `command`; treat legacy untyped records as terminal and validate types, commands, one agent per project, exact label uniqueness, and agent-move conflicts.
* [ ] Give ordinary new projects lazy `code` terminal and `git` sessions in order; keep promotions and existing projects unchanged, keep `n` terminal-only, and materialize each type in the project workdir.
* [ ] Add executable-only `agent-binary` to temporary project settings, including agent creation/update, collision suffixes, clearing semantics, and non-mutating executable failures.
* [ ] Test legacy and unknown-field compatibility, schema validation, presets, promotion preservation, settings updates and clearing, label suffixes, move conflicts, materialization, and missing executables.

## P1: Typed visual identity

* [ ] Render blue `>_ CODE`, teal `⎇ GIT`, and yellow `✦ AGENT` chips in sidebar and top bar without selection, live, or attention states replacing them.
* [ ] Test chip content and styling across selected, active, live, attention, sidebar, and top-bar states.

## P1: Session attention

* [ ] Install tmux activity and client-visit hooks that set attention only for unvisited output and clear it on any visit; display the runtime-only marker in sidebar and top bar without JSON writes.
* [ ] Test hook commands, inactive activity, visit clearing, rendering, and persistence isolation.

## P1: Published-module verification

* [ ] Install `github.com/rapsnx/tflow/cmd/tflow@latest` through the module proxy in a temporary location and verify `tflow version` matches the published release.
