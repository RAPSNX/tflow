<p align="center">
  <img src="./assets/tflow-logo.png" alt="tflow logo" width="360">
</p>

<p align="center">
  A focused terminal session manager built on top of tmux.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-experimental-f5c2e7?style=for-the-badge" alt="status">
  <img src="https://img.shields.io/badge/go-1.25+-89b4fa?style=for-the-badge&logo=go&logoColor=white" alt="go">
  <img src="https://img.shields.io/badge/tmux-powered-a6e3a1?style=for-the-badge" alt="tmux">
</p>

---

`tflow` keeps the terminals for each project together.

A typical repository needs more than one terminal: an IDE, a shell for tests, an agent, and sometimes a shell for checking a Kubernetes cluster. Once two or three projects are open, those terminals quickly become difficult to follow.

tflow gives each project its own context. Its sessions stay together, and a small sidebar lets you move between them without losing your place.

It is intentionally focused. tflow uses tmux to manage ordinary terminal sessions and tries to preserve normal terminal behavior. It is not a full terminal multiplexer or an IDE; it is a lightweight way to organize and revisit project sessions.

## Get started

tmux is required. Choose one of the following options.

### Nix

```sh
nix run github:rapsnx/tflow
```

### Home Manager

Add the flake input and module, then enable tflow:

```nix
# flake.nix
inputs.tflow.url = "github:rapsnx/tflow";

# Home Manager modules
inputs.tflow.homeManagerModules.tflow

# home.nix
programs.tflow.enable = true;
```

### Go

```sh
go install github.com/rapsnx/tflow/cmd/tflow@latest
tflow
```

## Use tflow

Start `tflow` in a terminal. It opens a volatile session that behaves like a normal terminal; closing that tflow instance closes its volatile sessions too. Projects you create contain persistent sessions that can be revisited later.

Press `Ctrl+Space` followed by `o` to open the sidebar. From there you can create, rename, move, delete, and switch projects and sessions. Press `Ctrl+Q` to quit the current tflow instance.

## Shortcuts

| Key | Action |
|---|---|
| `Ctrl+Space` `h` | Navigate to previous session in current context |
| `Ctrl+Space` `l` | Navigate to next session in current context |
| `Ctrl+Space` `o` | Toggle sidebar |
| `Ctrl+Space` `Esc` / `Ctrl+C` | Cancel command mode |
| `Ctrl+Q` | Quit the current tflow instance |
| `Ctrl+C` | Close sidebar, or pass through when the sidebar is closed |
| `Esc` | Cancel the current prompt or close the sidebar |
| `?` | Toggle help |
| `j` / `k` | Move selection |
| `Enter` | Switch to the selected session |
| `n` | Create a session |
| `N` | Create a project |
| `p` | Switch project |
| `e` | Edit project settings |
| `r` | Rename session |
| `R` | Rename project |
| `m` | Move session to another project |
| `d` | Delete session |
| `D` | Delete project |

`tflow` is experimental and intentionally minimal.
