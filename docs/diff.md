# Diff viewer

This is a new feature for code-factory: a terminal-based viewer for git diffs that offers some of the conveniences of GitHub's compare page without having to leave the code-factory UI and switch to a browser.

## State

The following state will be relevant for the Diffs view:

- The current ticket
- The worktree of the current ticket
- The fork point of the worktree's current branch, as determined by `git merge-base --fork-point <default branch>`
- The starting commit of the diff
- The ending commit of the diff

## Interface

We'll add a fifth view to the code-factory TUI called "Diffs", accessible via F5. It consists of two screens: the commit selector screen and the diff viewer screen. These are full-screen mini-views -- when one is visible, the other is hidden. The app starts with the commit selector screen visible.

### Commit selector screen

When the app starts, the commit selector screen is just a blank screen with the message "No ticket selected" in `emptyStateStyle`. Once a ticket has been selected (see 'Behaviour' below), the commit selector screen changes to a two-pane layout with a status bar at the top:

The **status bar** contains the following elements:

Left-justified:
  - "Ticket: <ticket-identifier> (<ticket-phase>)"
Right-justified:
  - "<N> commit<s> selected"

It's separated from the panes by a horizontal line beneath it.

The **left pane** is about 1/3 of the terminal in width. It shows a list of commits on the branch in the following format: `<first four characters of commit hash> <first line of commit message>` ordered in normal git "newest commits first" order. The commit message will be truncated when it meets the right edge of the pane. This is a selectable,  scrollable list. If the fork point is not equal to HEAD, there's a non-selectable medium-grey line above the fork-point commit that separates it from more recent commits on the branch. (Scrolling-wise, it behaves identically to the blank line between `needs-attention` and `user-review` tickets in Commands view -- we skip over it while scrolling as if it were not there.)

Unlike the other selectable, scrollable lists in this program, this one allows multi-selection of ranges. Normal movement of the selection sets the starting and ending commits to the currently selected commit. When the user presses an arrow key or page up/down while Shift is held down, the behaviour depends on which direction the selection is going. If the user is scrolling down, it will set the starting commit to the new currently selected commit and leave the ending commit unchanged. If the user is scrolling up, it will set the ending commit to the new currently selected commit and leave the starting commit unchanged. The currently selected commit is rendered with a `colourPrimary` background; the other selected commits between the starting and ending commits inclusive will have a `colourAccent` background. (You should create new styles to represent this concept, though.)

We don't need the entire commit history of the branch -- the most recent 100 commits on the branch will be plenty. If there are any uncommitted changes on the branch, they appear at the top of the list as a pseudo-commit labelled `???? Uncommitted changes`, where the question marks replace the usual first four characters of the commit hash. Omit merge commits from the list entirely -- we don't care about them.

The **right pane** takes up the remaining 2/3 of the terminal. It shows the output of `git show --stat` for the currently selected commit. It's neither scrollable nor selectable.

Pressing Tab or Enter switches to the diff viewer screen.

### Diff viewer screen

The diff viewer screen is a status bar at the top, followed by a single viewer pane filling the screen.

The **status bar** is two lines long, and contains the following elements:

First line:
  Left-justified:
    - "Ticket: <ticket-identifier> (<ticket-phase>)"
  Right-justified:
    - "File <current-file-index> of <total-files>"

Second line:
  Left-justified:
    "<current-file-name>"

The "current filename" is the name of the file whose diff is currently at the top line of the viewer pane. We display it left-truncated by an ellipsis if it's too long to fit into the status bar. (Example: `…ernal/db/project_context_test.go`)

It's separated from the panes by a horizontal line beneath it.

The **viewer pane** is a scrollable, non-selectable pane that shows the git diff between (commit before the starting commit) and (ending commit). (See "Diff format", below.)

Pressing Tab, Escape, or Enter switches back to the commit selector screen.

## Behaviour

We want to remove the existing `terminalGitDiff` functionality. Instead, whenever the user uses the 'g' keybinding in the Command or Log views to view a diff, it does the following:

- sets the Diffs view's current ticket to the currently selected ticket that 'g' was pressed on
- sets the Diffs view's starting and ending commits to the HEAD commit of the worktree's current branch
- sets the Diffs view's active screen to the commit selector screen.
- switches to the Diffs view

## Diff format

We want to clean up the raw diff output to make it more readable on a terminal.

* In a git diff, the normal file header looks like this:
```
diff --git a/internal/ui/command_view.go b/internal/ui/command_view.go
index 364ab79..4dd5c2c 100644
--- a/internal/ui/command_view.go
+++ b/internal/ui/command_view.go
```
We want to replace that with just the filename and a colon, in bold:
```
internal/ui/command_view.go:
```

Diff sections are normally indicated by a `@@` line like this:
```
@@ -244,8 +244,10 @@ func (v CommandView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
```
We want to remove the line number ranges and show just this:
```
@@ func (v CommandView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
```
If the original `@@` line doesn't contain context information, just show `@@` and leave the rest of the line blank. The `@@` line should have a `159` (light blue) background.

We want to remove the `-` and `+` characters which indicate which lines are added or removed. Instead, let's render the removed lines with a `219` background and the added lines with a `156` background. (Create new styles to represent these concepts.)

Background colours for `@@`, `+`, and `-` lines should extend across the entire width of the pane, not just the part of the line with text in it.

We should show line numbers on the left-hand side of the diff. Use the `@@` lines to determine how many columns we need for each file's line numbers. (For example, the above file with `@@ -244,8 +244,10 @@` as a header would have three columns for the line numbers, then a space, then the code.) Only show line numbers for added lines; leave the line number space blank for removed lines.

Each file in the diff should begin with a blank line to keep them visually distinct. (This includes the first file.)

Binary files should just show the filename and the message `(binary stuff)`, like so:
```
bin/fooble:
  (binary stuff)
```
The filename should be bolded, like usual; the message should be drawn in `emptyStateStyle`.

Deletes should appear like so:
```
internal/ui/command_view.go:
  Deleted
```
The filename should be bolded, like usual; the "Deleted" message should be drawn in bolded `52` (dark red).

Renames should appear like so:
```
internal/ui/command_view.go:
  Renamed to internal/ui/i_like_pie.go
```
The filename should be bolded, like usual; the "Renamed to" message should be drawn in bolded `18` (dark blue) and the new filename should be in plain text.
