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

Start `tflow` in a terminal. It opens with a default empty project and a volatile session. This behaves like a normal terminal: closing that tflow instance closes the terminals inside it too. Projects you create contain persistent sessions that can be revisited later.

Press `Ctrl+F` to open the sidebar. From there you can create, rename, move, delete, and switch projects and sessions. Press `Ctrl+Q` to quit the current tflow instance.

## Shortcuts

| Key | Action |
|---|---|
| `Ctrl+F` | Toggle sidebar |
| `Ctrl+Q` | Quit the current tflow instance |
| `Ctrl+C` | Close sidebar, or pass through when the sidebar is closed |
| `Esc` | Cancel the current prompt or close the sidebar |
| `?` | Toggle help |
| `j` / `k` | Move selection |
| `Enter` | Switch to the selected session |
| `n` | Create a session |
| `N` | Create a project |
| `p` | Switch project |
| `e` | Edit project workdir |
| `r` | Rename session |
| `R` | Rename project |
| `m` | Move session to another project |
| `d` | Delete session |
| `D` | Delete project |

## Known limitations

Some multi-line shell prompts (for example, starship configs that use a `fill`/padding module to reach the exact last terminal column before breaking to a new line) can show an extra blank line between the prompt's info line and the input line. This is a tmux line-wrap behavior for content that exactly fills the last column and is not specific to tflow — it reproduces in any tmux session. If you hit this, leave at least one column of margin instead of padding to the exact edge (for example, reserve one extra column in the fill/padding calculation).

## Future ideas

- diff view
- ripgrep search
- repository manager

`tflow` is experimental and intentionally minimal.
