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

`tflow` is a small terminal session manager with projects.

When working on one repository, you may have an IDE, a terminal for tests, an agent, and another terminal for checking a Kubernetes cluster. With two or three projects in parallel, it is easy to end up with terminals everywhere. tflow keeps those sessions together and lets you jump between project contexts.

tflow orchestrates this and not much more. It uses tmux as a backend for ordinary terminal sessions while preserving the terminal built-in behavior as much as possible. It is not a full terminal multiplexer or an IDE; it is a lightweight session handler for moving between sessions in the same context.

## Start

tmux is required.

With Nix:

```sh
nix run github:rapsnx/tflow
```

To install it with Home Manager, add the flake input and module, then enable it:

```nix
# flake.nix
inputs.tflow.url = "github:rapsnx/tflow";

# Home Manager modules
inputs.tflow.homeManagerModules.tflow

# home.nix
programs.tflow.enable = true;
```

Without Nix:

```sh
go install github.com/rapsnx/tflow/cmd/tflow@latest
tflow
```

## How it works

Start `tflow` in a terminal. The initial empty project is volatile, so closing that tflow instance closes the terminals inside it just like a normal terminal would. Projects you create contain persistent sessions that can be revisited later.

Press `Ctrl+F` to open the sidebar. From there you can create, rename, move, delete, and switch projects and sessions. `Enter` switches to the selected session, `n` creates a session, `N` creates a project, `p` switches projects, and `m` moves a session. Press `Ctrl+Q` to quit the current tflow instance.

## Future ideas

- diff view
- ripgrep search
- repository manager

`tflow` is experimental and intentionally minimal.
