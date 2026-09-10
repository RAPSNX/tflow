# tflow open implementation checklist

Only unfinished work derived from `.codex/ARCHITECTURE.md` belongs here.
Remove each item after implementation and verification.

## P1: Session attention

* [ ] Verify the `alert-activity` hook actually invokes `session-activity` in a
  real interactive tmux session. Manual testing on this development machine
  (tmux 3.7c) found the hook's registered `run-shell` command does not appear
  to execute, even a trivial one, while the identical mechanism reliably fires
  for `client-session-changed` on the same server -- confirmed via tmux's own
  `-vv` server trace, which showed the window-activity flag transitioning
  correctly with no observable effect from the hook. This may be specific to
  that tmux build; retest on the target release environment, and if it
  reproduces, find a working alternative trigger for "unvisited session
  produced output" before considering this done. The clearing half
  (`client-session-changed` → `session-visited`) was verified live and works.
