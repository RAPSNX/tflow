# tflow

`tflow` is a minimal terminal session manager built on top of `zellij`.

It starts by attaching to a `default` zellij session. The session switcher is a separate minimal menu opened with `tflow menu`.

## Behavior

- startup attaches to the `default` session in the `default` project
- each tflow session is backed by a zellij session
- the active zellij tab is renamed to the current project
- the menu is a tree of projects and sessions
- projects can be expanded and collapsed
- sessions can be selected, created, moved, or killed
- zellij is started with a compact layout and reduced chrome

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

Open the switcher from inside or outside zellij with:

```sh
go run ./cmd/tflow menu
```

When `tflow` starts or attaches a zellij session, it also provisions a zellij keybinding so `Ctrl+F`
opens the switcher in a floating pane.
