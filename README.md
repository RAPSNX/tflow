<p align="center">
  <img src="docs/tflow-logo.png" alt="tflow logo" width="180">
</p>

# tflow

`tflow` is a focused tmux-backed session manager for project-scoped terminal and agent sessions.

It keeps sessions grouped by project, starts every run with a volatile random-animal project and `code` session, and opens a sidebar with `Ctrl+F` for project and session actions.

## Behavior

- startup creates a new volatile random-animal project and a default `code` session
- volatile projects are hidden from the Projects list and are cleaned up when the owning terminal exits
- persistent projects are defined in `~/.config/tflow/config.yaml` under `projects`
- persistent projects and their sessions survive terminal exit unchanged
- each logical session maps to an internal tmux session name; the UI shows only logical names
- the sidebar shows Sessions for the current project and Projects for persistent projects
- persistent session restore state, including last known cwd and selected session, is stored in `state.json`
- `Ctrl+F` toggles the sidebar pane in the current tmux window

## Keys

- `j` / `k`: move through sessions and persistent projects
- `Enter`: switch to the selected session or persistent project
- `t`: create and switch to a new terminal session in the current project
- `a`: create and switch to a new agent session in the current project
- `r`: rename the selected session
- `p`: switch to another persistent project using hints
- `P`: persist the current volatile project
- `m`: move the selected session to another persistent project using hints
- `?`: toggle help
- `Esc`, `Ctrl+C`: cancel inline input or close the sidebar

## Config

`config.yaml` lives beside `state.json` in the tflow config directory. It is the source of truth for persistent projects.

```yaml
projects:
  - name: "small"
    workdir: "/tmp/project-small"
    agent-cmd: "codex"
theme: "catppuccin"
colors:
  blue: "#89b4fa"
```

`tflow` may create or update `config.yaml` when a volatile project is persisted with `P`. `state.json` is runtime and restore state only.

Built-in themes:

- `catppuccin` (default)
- `forest`

## Run

```sh
go run ./cmd/tflow
```

Open the sidebar directly with:

```sh
go run ./cmd/tflow menu
```

## Nix

Build the package with:

```sh
nix build .#tflow
```

The flake also exports a Home Manager module.
