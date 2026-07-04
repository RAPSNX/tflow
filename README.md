# tflow

`tflow` is a minimal terminal session manager.

It starts directly in a `default` session so the first screen looks like a normal shell. The session menu is hidden until you toggle it.

## Behavior

- startup attaches to the `default` session in the `default` project
- `Ctrl+F` toggles the left-side session menu
- the menu is a tree of projects and sessions
- projects can be expanded and collapsed
- sessions can be selected, created, moved, or killed

## Keys

- `j` / `k`: move through the tree
- `h` / `l`: collapse or expand the current project
- `Enter`: open a project or switch to the selected session
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
