# Issues / Bus
This describes issues that are existing, which should be fixed in future.

- no deletion prevention, should ask if deletion is ok
    - project yaml option: protect: true


# Features
This are a list of featuures that need to be implemented:

- Make a project configuratble `e` for edit, a project is a yaml with: `name`, `workdir`, `cluster`
    - Projects are persistent
    - cluster: could be `path` or `connection-cmd`
- `n` for new needs now a arg, which is simply the presiding key so:
    - `np` new project
    - `nt` new terminal session
    - `nk` new k9s session
    - `nc` new codex session
- `m` move dont has a menu, it only highlits (like `hint` mode in alacritty), the starting charcters of the posiblites.
- general design improvement
