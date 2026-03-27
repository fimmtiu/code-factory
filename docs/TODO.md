TO DO

- Update references to "comments" in /cf-respond

- Add a /cf-change skill to manually add change requests to the ticket that the current worktree belongs to.

- Some sort of `tickets clean` command that removes directories for all tickets/projects that are closed

- Ephemeral pop-ups that show current view name when you switch, and tell you when you copy stuff &c.

- Settings file:
  - Wait-for editor command
  - Independent editor command
  - Terminal-opening command

- Run Claude on the other skills to get suggestions

- Change repo name to `code-factory`


We're going add a few new parameters in the settings file. These include:
- `blocking_editor_command`: replaces the current use of $EDITOR in the code base. Default value: the value of $EDITOR, or `cursor --wait` if not specified.
- `nonblocking_editor_command`: replaces the current use of `"cursor"` in openCursor(). Defaults to `cursor` if not specified. In the process, rename `openCursor()` to `openEditorNonblocking()`.
- `open_terminal_command`: replaces the current use of `open -a iTerm`. Defaults to `open -a iTerm .` if not specified.
