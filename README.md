# tflow

`tflow` starts by attaching directly to the `default` session on the dedicated `tflow` tmux socket.

If the session does not exist yet, `tflow` creates it in the `default` project first. The initial experience is meant to look like a normal terminal, not a launcher screen.

## Menu

Press `Ctrl+F` inside a managed session to toggle the left menu pane.

The menu is a tree:

- projects are top-level items
- projects can be opened and closed
- sessions appear under their project

## Keys

- `j` / `k`: move through the tree
- `h`: collapse the current project, or move from a session back to its project
- `l`: expand the current project
- `Enter`: toggle a project, or switch to the selected session
- `n`: create a session in the current project
- `p`: create a project
- `m`: move the selected session to another project with prefix matching
- `x`: kill the selected session
- `Esc`, `q`, `Ctrl+C`: close the menu pane

## Run

```sh
go run ./cmd/tflow
```
