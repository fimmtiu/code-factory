# Code Factory

We're going to create a command-line tool in this repository called `code-factory`. It's a terminal-based coding agent manager which spins up a pool of workers to tackle all of the tickets in a git repository.

This is a program with a terminal UI (use bubbletea and lipgloss libraries) that runs a set of workers to process the tickets in `.code-factory/`, with various views that allow users to monitor their progress and inspect/approve their work.

## Architecture notes

A "work unit" is a ticket or a project.

Workers are identified by a 1-based number (e.g., with the default pool size of 4, we'd have workers 1 through 4.)

There's a log channel that all workers use to report their status back to the main goroutine. Log entries are composed of a timestamp, a worker number, a short message, and an optional logfile name. We only keep the most recent 200 log messages; if we exceed 200, the application deletes the oldest log messages to get back to 200. Logs are stored in the database for persistence between `code-factory` sessions, so we'll need a new `logs` table in SQLite and new accessor methods in the DB wrapper to abstract it.

The main goroutine keeps track of the following data for each worker: the worker's identifier number, a bidirectional channel for communication between the main process and the worker, a "paused" flag, and a status enum ("idle", "awaiting response", or "busy", with more to come later).

Pausing a worker prevents it from picking up any new work. When paused, it should finish off whatever it's currently working on and then sit idle.

Communication from the main goroutine to the workers includes:
  - Telling the workers to pause and unpause
  - Sending a response to a question asked by a worker's agent
  - Approving or denying a permissions request from a worker's agent

Communication from the workers to the main goroutine includes:
  - Sending a question asked by a worker's agent
  - Sending a permissions request from a worker's agent for approval

There's a housekeeping thread that wakes up once per minute to release claimed tickets:
  - If there are any tickets that have been claimed, and their `last_updated` timestamp is more than 10 minutes ago, release them.

Some functionality involves letting the user edit a field from the database. Here's how to do it:
  1. Write the existing content to a temporary file in `/tmp` (use `os.CreateTemp` to create the file)
  2. The `EDITOR` environment variable contains a command (possibly including arguments) to execute to start an editor. Invoke it on that file.
  3. When the editor command exits, update the database field that we're editing with the new contents of that temporary file.
  4. Delete the temporary file.
This should be abstracted into a single piece of code that can be shared by multiple parts of the app, rather than being duplicated for every feature that needs to edit text.

Sometimes we want to open a terminal window in a specific directory. We can do that with this command: `open -a iTerm <directory>`.

Whenever we run a subprocess (editor, `open`, `pbcopy`, etc.), we don't want the terminal UI to suspend because these will be either background commands or things that open in a new window. Use `os/exec` instead of `tea.ExecProcess` for starting subprocesses.

## Command-line arguments

`-p <N>`, `--pool <N>`: Number of agents in the worker pool (defaults to 4)
`-w <N>`, `--wait <N>`: Seconds to wait between polls for open tickets (defaults to 5)

## Lifecycle

On startup, we do the following:

- If we're not in a git repository or there's no top-level `.code-factory/` directory, die with an error message.
- Start a new worker goroutine for each worker in the pool
- Start the bubbletea-based terminal UI


The worker goroutines' loop looks like this:

- If not paused, each worker calls `db.Claim` to fetch a ticket that's ready to be worked on.
  - If there are no tickets available, or if we're paused, sleep for `--wait` seconds and retry.
- Set the worker's status to "busy" and the ticket's status to `in-progress`.
- Use the [`acp-go-sdk` library](https://github.com/coder/acp-go-sdk) to run a Claude subprocess.
  - The ACP subprocess' working directory MUST be the ticket's worktree directory. Do this by setting the `Dir` field on an `exec.Cmd`. DO NOT USE `os.Chdir`, as it will cause bugs!
  - Pipe the prompt for the current phase into the ACP.
  - All input to and output from Claude should be saved to a logfile. Claude logfiles should be named `.code-factory/<ticket-identifier>/<ticket-phase>.log`. If that file already exists, append a `.1`, `.2`, etc. monotonically increasing number as a suffix to make it unique.
    - This output MUST include Claude's "thinking" messages, so that we can use them to introspect on agent behaviour later.
  - If Claude requests clarification from the user or requests additional permissions, set the worker's status to "awaiting response", set the ticket's status to `needs-attention`, send the request to the main goroutine, and wait for a response.
  - When a response is received, send it back to Claude, set the ticket's status to `in-progress`, and set the worker's status to "busy".
- When Claude is completely done processing the prompt, change the ticket's status to `user-review` and run `db.Release` on it, then set the worker's status to "idle".
- Loop back to `db.Claim` and start over

After each noteworthy action taken by a worker, it sends a timestamped message through the log channel saying "Worker N started reviewing <ticket-identifier>" or a similar human-readable description of what happened. Noteworthy actions include:
  - Successfully claiming a ticket
  - Releasing a ticket
  - Completing a Claude process

What the worker does with a ticket depends on what phase it's in:

#### Implement phase

- This is the prompt we will pass to the Claude agent as input:
```
You are an experienced staff software developer with a keen eye for abstraction and good code design. Implement the following work in the git worktree <path-to-worktree> for ticket "<ticket-identifier>":

<ticket description>

For all of your changes, you MUST begin by writing new specs or editing existing relevant specs to cover your changes before you start coding. Specs that cover your planned changes MUST exist before you begin changing code.

When you're done, commit your changes to the worktree with a commit message that explains what you did and why you did it. You may create intermediate commits if you need to, as long as you give them complete commit messages.
```

For tickets with no parent project, the above prompt is sufficient. For tickets inside a project, we will:
  1. Read the parent project's `identifier` and `description` from the database
  2. Append the following text to the prompt:
```

### Additional context from <project identifier>

<project description>
```
  3. If the project is a subproject that has a parent, repeat steps 1 through 3 for its parent project recursively until we've appended data from the entire project tree to the prompt.

#### Refactor phase

Pass the following prompt to the Claude agent as input:
```
/cf-refactor on worktree <path-to-worktree> for ticket "<ticket-identifier>"
```

#### Review phase

Pass the following prompt to the Claude agent as input:
```
/cf-review on worktree <path-to-worktree> for ticket "<ticket-identifier>"
```

#### Respond phase

Pass the following prompt to the Claude agent as input:
```
/cf-respond on worktree <path-to-worktree> for ticket "<ticket-identifier>"
```

## Approval

A chief function of `code-factory` is showing the user the work that the agents are doing and giving them a chance to approve them or request changes. The actions we take depend on the phase the ticket is in:

#### Implement phase

Upon approval, we will:
- set the ticket's phase to "refactor"
- set the ticket's status to "idle"

#### Refactor phase

Upon approval, we will:
- set the ticket's phase to "review"
- set the ticket's status to "idle"

#### Review phase

Upon approval, we will:
- set the ticket's phase to "respond"
- set the ticket's status to "idle"

#### Respond phase

Upon approval, we will:

**Set the ticket to done**: Use `SetStatus` to set the ticket's phase to "done" and the status to "idle".

**Recursively mark work units as done**: If the ticket has no parent project, nothing needs to be done. Otherwise, recursively do the following, starting with the ticket's parent project:
1. If all of the project's work units (direct descendants only, non-recursive) are in the "done" phase, mark the project as "done".
2. Repeat for the project's parent, if it has one.

## Views

The terminal UI has four different views that the user can switch between. It starts in Project view.

Key bindings common to all views:
- Q or Ctrl-C: Bring up the quit dialog
- ? or H: Bring up the help dialog
- F1: Switch to the project view
- F2: Switch to the command view
- F3: Switch to the worker view
- F4: Switch to the log view
- Shift-Tab: Switch to the next view
- Ctrl-Tab: Switch to the previous view

### Project view

This view has three panes, two of which are focusable and interactable. Each pane has a border around it. The currently focused pane has a bold double-line border in blue; the unfocused panes have a single-line grey border.

1. A **status pane** in the upper left. It's fixed-size and not focusable or interactable. It shows the current status of the work being done: the number of tickets, the number of open tickets, and a percentage of tickets complete. (We may add more later.) It fetches this data from the database using `internal/db`. (You may add a new query + accessor method to make this more eficient.)

2. A **tree pane** in the upper right. It will expand horizontally as the window grows. It shows a hierarchical tree-style list of work units. Blocked work units are drawn in grey; work units in the "done" state are underlined; all other work units are drawn normally.

This is a selection list. By default, the first item in the list is selected.

Additional key bindings when this pane is focused:
- Up/down arrows: Move the currently selected list item up or down one line. Scroll the list up or down by one line if the new selected list item would be off the edge of the visible list.
- Page Up/Page Down: Move the currently selected list item up or down one pane-height. Scroll the list up or down by one pane-height if the new selected list item would be off the edge of the visible list.
- Tab: Switch focus to the work unit pane.
- Enter: Open the change request dialog for that ticket (no-op for projects)
- T: Opens a terminal window in the work unit's worktree.
- E: Edit the description for the work unit.

3. A **work unit pane** that fills the bottom half of the screen. It shows details about the ticket or project that's currently selected by the tree pane. It includes the type (ticket or project), the identifier, the phase, the status (for tickets only), the description, and the list of change requests (for tickets only).

Additional key bindings when this pane is focused:
- Up/down arrows: Scroll the work unit information up or down one line.
- Page Up/Page Down: Scroll the work unit information up or down by one pane-height.
- Tab: Switch focus to the tree pane.
- Enter: Open the change request dialog for that ticket (no-op for projects)
- T: Opens a terminal window in the work unit's worktree
- E: Edit the description for the work unit.

### Command view

One single pane that covers the entire screen. Shows a list of tickets ordered by status: `needs-attention` on top, then `user-review`. Within those categories, they're sorted by `last_updated`. `in-progress` and `idle` tickets aren't shown. Updates every `--wait` seconds by querying the database. This is a selection list. By default, the first item in the list is selected. If both `needs-attention` and `user-review` tickets are in the list, we insert a blank line between the `needs-attention` and `user-review` tickets. The blank line is purely cosmetic and not selectable; pressing down-arrow when you're on the final `needs-attention` ticket takes you to the first `user-review` ticket.

Each list item looks like:

"<ticket-identifier> <status> <minutes since last updated>m"

It's displayed in a tabular fashion, with the ticket-identifier taking up as much of the line width as possible.

Additional key bindings for this view:
- Up/down arrows: Move the currently selected list item up or down one line. Scroll the list up or down by one line if the new selected list item would be off the edge of the visible list.
- Page Up/Page Down: Move the currently selected list item up or down one pane-height. Scroll the list up or down by one pane-height if the new selected list item would be off the edge of the visible list.
- R: Open a new blank file with the user's $EDITOR. When the editor exits, feed the contents of the file into this worker's agent via ACP. (Find the worker for a given ticket with `tickets.claimed_by`.) Only applies to `needs-attention` tickets; it's a no-op for `user-review` tickets.
- T: Opens a terminal window in the ticket's worktree.
- E: Run `cursor <ticket's worktree directory>` as a background process with `exec.Start()`. We don't care about examining its output or waiting for it to finish.
- A: Approves the currently selected ticket according to the logic defined in the "Approval" section above.

### Worker view

One single pane that covers the entire screen. Shows a list of workers in the worker pool, ordered by worker number. Each list item looks like `<worker number>: <worker status>` The color of the text indicates the worker's status: grey for "idle", red for "awaiting response", dark green for "busy", yellow if it's paused. Beneath each worker is the last three lines of text output by the worker's agent, updated constantly as the main goroutine reads output from the worker's channel. There's a separator line beneath each worker's list entry. This list is non-selectable and the only interaction allowed is scrolling.

Additional key bindings for this view:
- Up/down arrows: Scroll the list up or down by one line.
- Page Up/Page Down: Scroll the list up or down by one pane-height.

### Log view

One single pane that covers the entire screen. Shows a list of the last 200 log entries with three columns: timestamp, worker number, and message. Updates itself by checking the database every 3 seconds when the view is active, or immediately when we switch to this view. It's sorted from oldest to newest, so new entries go at the bottom.

When the list updates, if the selection is on the previous final log entry, we select the new final log entry and push all the older entries up. If the selection is on a higher line, it stays there and the list doesn't scroll. (We may tinker with this behaviour later, so make it easy to modify.)

Additional key bindings for this view:
- Up/down arrows: Move the currently selected list item up or down one line. Scroll the list up or down by one line if the new selected list item would be off the edge of the visible list.
- Page Up/Page Down: Move the currently selected list item up or down one pane-height. Scroll the list up or down by one pane-height if the new selected list item would be off the edge of the visible list.
- E: If the currently selected log entry has a logfile associated with it, open the logfile directly in $EDITOR. (Do not use a temporary file.)
- C: If the currently selected log entry has a logfile associated with it, copy its path to the clipboard with the `pbcopy` command. (For now, we're assuming this program will be macOS-only.)

### Dialogs

Additional key bindings for all dialogs:
- Escape: Dismiss the dialog

**Quit dialog**: If there are any workers that are not in the `Idle` state when the user requests that the program exit, we should bring up a dialog that says "There are still active workers. Really quit?", with "Cancel" and "Quit" buttons. The cancel button dismisses the dialog; the quit button exits the program. If all workers are idle, just exit the program immediately without showing the dialog. It has a title: "Really quit?".

**Help dialog**: Shows a dialog that lists all the keybindings in the current view. Has an "Okay" button that dismisses the dialog. It has a title: "Help".

**Change request dialog**: Has two panes: a list of change requests on the left and the details of the change request on the right. It has a title: "Change requests for <ticket-identifier>".

The **list pane** shows one line for all of the change requests on a given ticket. This is a selection list. By default, the first item in the list is selected. Each line contains the timestamp (truncated format, `MM/DD HH:MM`) and author name for the change request, sorted by most recent first. Dismissed change requests are drawn in grey. Open change requests are drawn in black. Closed change requests are drawn in dark green.

The **contents pane** shows the filename, line number, status, the contents of that line in that file from the ticket's worktree plus three lines of context in each direction, and description of the change request. Always look up the file at the change request's `commit_hash` rather than the current HEAD to ensure that we're looking at the right code.

Additional key bindings for the change request dialog:
- Up/down arrow: Move the selected list item up/down by one line. Scroll the list by one line if the new selected list item would be off the edge of the visible list.
- Page Up/Page Down: Move the selected list item up/down by one pane-height. Scroll the list by one pane-height if the new selected list item would be off the edge of the visible list.
- X: Set the currently selected change request's status to "dismissed"
- O: Set the currently selected change request's status to "open"
- E: Edit the description of the currently selected change request
