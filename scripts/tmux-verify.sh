#!/usr/bin/env bash
# scripts/tmux-verify.sh -- safe setup/cleanup for isolated tmux
# verification sessions (AGENTS.md's "Verification" section). Never
# hand-roll mktemp/export/rm -rf for this -- see the incident in
# URGENT.md, where a bare `rm -rf "$TMUX_TMPDIR"` ran with the variable
# unset in a shell that hadn't re-sourced it, fell back to an ambient
# value, and wiped the user's XDG_RUNTIME_DIR (Wayland, D-Bus, PipeWire
# sockets), forcing a session restart to recover.
set -euo pipefail

usage() {
  echo "usage: $0 setup | cleanup <root-dir>" >&2
  exit 1
}

# safe_root is the one check standing between a typo/unset-variable bug
# and another wiped XDG_RUNTIME_DIR: refuse anything that isn't a real,
# existing directory this script itself created.
safe_root() {
  local root="${1:-}" tmp="${TMPDIR:-/tmp}"
  tmp="${tmp%/}"
  if [[ -z "$root" ]]; then
    echo "refusing: root directory is empty" >&2
    exit 1
  fi
  case "$root" in
    "$tmp"/tmux-verify.*) ;;
    *)
      echo "refusing: '$root' is not one of this script's own $tmp/tmux-verify.* directories" >&2
      exit 1
      ;;
  esac
  if [[ ! -d "$root" ]]; then
    echo "refusing: '$root' does not exist" >&2
    exit 1
  fi
  # Re-check the *canonicalized* path too -- the case match above is a
  # literal string match, so "$tmp/tmux-verify.X/../../etc" would pass it
  # if that first segment happens to be a real directory. realpath resolves
  # any ".."/symlinks before this second check, closing that gap.
  local resolved
  resolved="$(realpath -- "$root")"
  case "$resolved" in
    "$tmp"/tmux-verify.*) ;;
    *)
      echo "refusing: '$root' resolves to '$resolved', outside $tmp/tmux-verify.*" >&2
      exit 1
      ;;
  esac
}

cmd_setup() {
  local root
  root="$(mktemp -d "${TMPDIR:-/tmp}/tmux-verify.XXXXXX")"
  mkdir -p "$root/tmux" "$root/state"
  # `export NAME=%q` (not a plain NAME=value assignment): eval'ing this
  # output must put these into the *exported* environment, or any tmux/
  # tflow subprocess launched afterward without an explicit per-command
  # prefix silently inherits whatever TMUX_TMPDIR/XDG_STATE_HOME the
  # ambient shell already had -- the same class of implicit-env-state bug
  # that caused the incident this script exists to prevent. %q shell-quotes
  # the value so eval handles it safely even if TMPDIR ever contains
  # unusual characters.
  printf 'export ROOT=%q\n' "$root"
  printf 'export TMUX_TMPDIR=%q\n' "$root/tmux"
  printf 'export XDG_STATE_HOME=%q\n' "$root/state"
  printf 'export SOCK=%q\n' "tflow"
}

cmd_cleanup() {
  local root="${1:-}"
  safe_root "$root"
  TMUX_TMPDIR="$root/tmux" tmux -L tflow kill-server >/dev/null 2>&1 || true
  rm -rf -- "$root"
}

case "${1:-}" in
  setup) cmd_setup ;;
  cleanup) shift; cmd_cleanup "${1:-}" ;;
  *) usage ;;
esac
