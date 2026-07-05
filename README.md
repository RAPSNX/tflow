# tflow

`tflow` is a minimal terminal session manager built on top of `tmux`.

It starts by attaching to the `default` session on the dedicated `tflow` tmux socket.

## Behavior

- startup attaches to the `default` session
- each tflow session is backed by a tmux session
- the menu is a tree of projects and sessions
- projects can be expanded and collapsed
- sessions can be selected, created, moved, or killed
- `Ctrl+F` toggles the menu pane in the current tmux window

## Keys

- `j` / `k`: move through the tree
- `h` / `l`: collapse or expand the current project
- `Enter`: switch to the selected session
- `n`: create a new session in the current project
- `p`: create a new project
- `m`: move the selected session to another project by prefix
- `d`: delete the selected project and move its sessions to `default`
- `x`: kill the selected session
- `q`, `Esc`, `Ctrl+C`: close the menu

## Run

```sh
go run ./cmd/tflow
```

Open the switcher directly with:

```sh
go run ./cmd/tflow menu
```

When `tflow` starts or attaches a tmux session, it provisions a `Ctrl+F` binding that toggles the
menu pane.
