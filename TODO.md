# Issues / Bus
This describes issues that are existing, which should be fixed in future.

- [x] no deletion prevention, should ask if deletion is ok
    - [x] project yaml option: protect: true


# Features
This are a list of featuures that need to be implemented:

- [x] Make a project configuratble `e` for edit, a project is a yaml with: `name`, `workdir`, `cluster`
    - [x] Projects are persistent
    - [x] cluster: could be `path` or `connection-cmd`
- [x] `n` for new needs now a arg, which is simply the presiding key so:
    - [x] `np` new project
    - [x] `nt` new terminal session
    - [x] `nk` new k9s session
    - [x] `nc` new codex session
- [x] `m` move dont has a menu, it only highlits (like `hint` mode in alacritty), the starting charcters of the posiblites.
- [x] general design improvement
