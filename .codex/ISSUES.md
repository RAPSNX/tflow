# Issue tracker
This file is human written, and contains findings while using `tflow`.
This findings should always be validated against `ARCHITECTURE.md` and `TASK.md`.
And afterwards it should valid and correct tasks derived from it.
If anything here will collide or interfer with the current `ARCHITECTURE.md`, stop and ask questions.

## Issues

- Projects are vanished after reboot.
- Project creation should take the actual PWD as workdir directly.
- Project edit should use `~` as home folder, and should have autocompletion.
- Project edit should open the project settings yaml as file in the `EDITOR`.
- Having multiple projects open, and delete a session in a project, can result in jumping to another project.
    - It should be ensured, that no action will move the Project ever, except the `switch project`.
