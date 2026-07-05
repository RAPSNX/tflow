# tflow

`tflow` is a minimal terminal session manager built on top of `tmux`.

It starts by attaching to the `default` session on the dedicated `tflow` tmux socket.

## Behavior

- startup attaches to the `default` session
- each tflow session is backed by a tmux session
- the menu is a tree of projects and sessions
- project configs are persisted as YAML files alongside the app state
- projects can be expanded, collapsed, edited, and protected from deletion
- sessions can be selected, created, moved, or killed
- `Ctrl+F` toggles the menu pane in the current tmux window

## Keys

- `j` / `k`: move through the tree
- `h` / `l`: collapse or expand the current project
- `Enter`: switch to the selected session
- `n` then `p`: create a new project
- `n` then `t`: create a new terminal session
- `n` then `k`: create a new `k9s` session
- `n` then `c`: create a new agent session
- `c`: set the default directory for the current project
- `e`: edit the current project YAML
- `m`: move the selected session to another project by prefix
- `r`: rename the selected project or session
- `d`: confirm-delete the selected project or session
- `q`, `Esc`, `Ctrl+C`: close the menu

## Project Config

Project configs are stored as YAML in the app config directory under `projects/`.

```yaml
name: "small"
workdir: "/tmp/project-small"
protect: true
cluster:
  path: "/tmp/kubeconfig"
```

`cluster.connection-cmd` is also supported for `k9s` sessions.

Projects can also select the agent binary:

```yaml
agent-binary: "codex"
```

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
