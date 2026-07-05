# Open

## Issues / Bugs
- Seems that there are still some code traces that interfer with "default" project.
    - There is no such thing like a default project anymore!!

## Features

- Deletion of a project deletes all sessions in it.
- Conceptual change:
    1. the start of `tflow`, creates a random animal project.
    2. In the sidebar is only a list with `sessions`. (So every project only sees his own sessions.)
    3. New overhaul of all keybindings:
        - `t` creates a terminal session
        - `a` creates a agent session
        - `k` creates a k9s session
        - `n` creates a nvim session
        - `r` is to rename the session
        - `R` is to rename the project
        - `ctrl+q` kills everything
        - `P` adds a overlay with a list of all other active Projects -> enter will switch into this.

# Done
## Issues / Bugs
This describes issues that are existing, which should be fixed in future.

- [x] there is a typo, section should be session, tflow has projects and sessions, sessions have a type: `terminal`, `k9s` and `agent`
- [x] no deletion prevention, should ask if deletion is ok
    - [x] project yaml option: protect: true


## Features
This are a list of featuures that need to be implemented:

- [x] Make a project configuratble `e` for edit, a project is a yaml with: `name`, `workdir`, `cluster`
    - [x] Projects are persistent
    - [x] cluster: could be `path` or `connection-cmd`
- [x] `n` for new needs now a arg, which is simply the presiding key so:
    - [x] `np` new project
    - [x] `nt` new terminal session
    - [x] `nk` new k9s session
    - [x] `nc` new agent session
- [x] `m` move dont has a menu, it only highlits (like `hint` mode in alacritty), the starting charcters of the posiblites.
- [x] general design improvement
- [x] Create a `README.md` with style, add the logo from actual repo root there, but move it somewhere.
    - Look into other readmes like `zellij` and my others in `rapsnx/dotfiles` and `rapsnx/neonix` as a example.
- [x] Design change: Temporary sessions
    - [x] the default session will be removed, per default it starts a new temporary session, with a <animal>-temp name.
    - [x] temp sessions are not part of any project, or in any list.
    - [x] I can add a temp session with `na` to the selected project
    - [x] A temp session which is not added to any project, dies with the terminal itself.
- [x] tflow should have a config file, which refers to a projects folder with the yamls of each project.
    - [x] the config file can be used to set colors / style
        - [x] per default it uses catpuccin
    - [x] with empty config file, thre are no projects
- [x] Add `flake.nix` to build `tflow`.
- [x] In a Project its possible to configure which agent binary should be spawned.
- [x] Add a home-manager module to configure it:
    - enable and pkg as usual
    - [x] settings:
        - [x] projects can be configured here

