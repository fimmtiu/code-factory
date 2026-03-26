# Code Factory

We're going to create a command-line tool in this repository called `code-factory`. It's a terminal-based coding agent manager which spins up a pool of workers to tackle all of the tickets in a git repository.

## `code-factory` command-line tool

# FIXME: IDEAS

Basically a combination ticket viewer/worker viewer. Separate windows tmux-style?

Projects view: Same as current tickets-ui, except it shows which tickets are being worked on and the status of the ticket on the navigator pane. Hit enter on a ticket to get to...

Ticket view:
  - Top status bar: ticket name, status
  - Main pane: Ticket prompt, comments?
    - Key to open the prompt in an editor with "I asked for this, but got this. How can I change the prompt to be better?" When saved, opens a new iTerm tab and pushes that as a prompt to Claude.
  - For each comment:
    - Key to open the prompt in an editor with "The review skill came up with this, but it's bad advice. How can I change the skill to be better?" When saved, opens a new iTerm tab and pushes that as a prompt to Claude.
  - Keys to tell an agent to fix or ignore that comment.
  - Human approval to move things to the next step? I don't think we can trust the steps to run independently yet.

Workflow:
  - An outside force pushes a bunch of projects and tickets into the .tickets directory
  - We run code-factory. It reads the work units and spins up N agents to work on them
    - Each agent picks up a ticket with the `idle` status that's not `done` or `blocked`, sets status to `in-progress`,
      - `plan` phase: writes a `work_plan.md` file to the ticket directory
      - `implement` phase: creates a worktree in the ticket directory, does work there
      - `refactor` phase: runs refactor skill on that worktree, commits changes
      - `review` phase: run the review skill on the worktree, adds comments to ticket
      - `respond` phase: addresses comments that are marked as fixable
      Then it sets status to `needs-attention`, moves on to the next.
    - The user watches a list of `needs-attention` tickets. Clicking into one:
      - shows the output of the current phase (plan file, git diff, comments, etc.)
      - allows the user to interact with it (open terminal or Cursor window in worktree)
        - Comments: Show a list of comments that can be scrolled through.
          - Mark comments as ignored. All non-ignored comments will be addressed in the `respond` phase.
      - allows the user to approve the changes and move the ticket to the next phase + `idle` status


Windows:
  - Project view: tree view of projects
  - Log view: Shows all events in the system. Selecting a log entry shows the Claude debug log for that session.
  - To-do view: Shows all tickets that need attention. User can press a key to move them to the next step.
  - Worker view: Shows what each worker is up to.

FIXME DO IN SEPARATE STEP:
New way to do statuses:
Phase: plan, implement, refactor, review, respond, done
Status: idle, needs attention, user review, in progress

On a new branch, we're going to break the existing `status` field for tickets into two separate, more detailed fields:

Phase: one of `blocked`, `implement`, `review`, `respond`, `refactor`, `done`
Status: one of `idle`, `needs-attention`, `user-review`, `in-progress`

We want to change the `set-status` command to take three arguments: an identifier, a phase, and an optional status. If the status is not provided, it defaults to `idle`. It will set both the phase and status fields on the ticket.

Let's remove the `status` field from projects altogether. For now, make `tickets-ui` no longer show project statistics.


FIXME DO IN SEPARATE STEP

Ensure that `tickets claim` only hands out tickets which have `idle` status and any phase besides `blocked` or `done`. Blocked tickets, done tickets, needs-attention tickets, or in-progress tickets should not be returned by `claim`.

### Command-line arguments

`-p <N>`, `--pool <N>`: Number of agents in the worker pool (defaults to 4)

### Lifecycle

The program

When the program runs without any arguments:

- Check to see if there's an existing `tmux` session named "code-factory-<current-repository-name>" using the tmux `has-session` command.
  - If so, exec `tmux attach-session -t <session-name>`. (By "exec" here, I specifically mean the Unix syscall that replaces the current process.)
  - Otherwise, exec `tmux new-session -s <session-name> -n Factory code-factory --new`

When the program is invoked with the `--new` option:

- Run `tmux new-window -d -n Tickets tickets-ui`
- Create a Unix-domain socket at the root of the `.tickets/` directory called `.factory.sock`.
- For each agent in the worker pool, run `tmux new-window -d -n WorkerN code-factory-worker N`, where `N` is the 1-based number of the agent. (At the default pool size of 4, that would be `Worker1` through `Worker4`).
- Start the bubbletea-based terminal UI

### User interface

The user interface, which will run in window 0 of the tmux session, has two bubbletea panes, each taking up half of the terminal's vertical screen real estate. The top one is the worker pane and the bottom one is the log pane. There's a border around the panes, and the border of the currently focused pane is highlighted.

#### Worker pane

The worker pane is the initially focused pane when the UI starts. It displays a table of workers and what they're working on like so:

```
<worker-number> <full-ticket-identifier> <current-status> <attention-emoji>
```

* `worker-number` is a 1-based number indicating which agent in the pool this is. (Same as `N` in the `tmux` command above.) When calculating column width, assume that the `worker-number` will never be more than two digits.

* `full-ticket-identifier` is the full `project-name/ticket-name` identifier of the ticket that agent is working on. This will take up most of the window's length. If the column isn't wide enough to show the full identifier, truncate the beginning with an ellipsis like so: `...foo/bar/baz`.

* `current-status` will be one of `Planning`, `Coding`, `Reviewing`, `Responding`. `Refactoring`, `Waiting`, or `Idle`.

* `attention-emoji` is "⁉️" if the worker's agent is requesting input from the user, "✅" if a ticket is ready for review by the user, or blank otherwise.


*****************************************

FIXME THIS SUCKS ASS

We don't want workers to get stuck waiting for users to approve tickets! We want them picking up the next piece of work. When a worker gets a ticket to a `merge-ready` state, we should push it to an approval queue and let the worker keep working.

This should not be a list of workers — it should be a list of tickets.

*****************************************


This table should resize dynamically when the window changes size, increasing or decreasing the size of the `full-ticket-identifier` column as necessary. (The other columns are fixed-width.)

The table is sorted by `(attention-emoji, worker-number)`. "⁉️" entries come first, then "✅" entries, then entries with no attention emoji.

Key bindings when this pane is focused:
  - Tab: Switch focus to log pane
  - Q or Ctrl-C: Bring up quit dialog
  - Escape: Clear the current selection
  - Up/down arrows: Select the worker above/below the current line. If no worker is currently selected, select the first line in the table.
  - Space: If a worker is selected, open a Cursor editor for the worktree of the currently selected worker's ticket
  - A: If the currently selected worker's ticket is in the `merge-ready` state, tells the worker to approve and merge the ticket (FIXME)
  - T: If a worker is selected, open a new iTerm tab in the worktree of the currently selected worker's ticket
  - ? or H: Show the help dialog

#### Log pane

Every time a worker changes state, we append a line like this to an in-memory log of state changes:
```
[12/06 23:45:16] Worker N reviewed "full-ticket-identifier"
```
It consists of a shortened timestamp, the worker number, a human-readable past-tense description of what the worker did, and the full identifier of the ticket.

The log pane displays this buffer, with the latest entries at the bottom, and updates whenever new log entries are added. It doesn't have a currently selected or currently highlighted line; it just shows a read-only log.

Key bindings when this pane is focused:
  - Tab: Switch focus to worker pane
  - Q or Ctrl-C: Bring up quit dialog
  - C: Clear the in-memory log.
  - Up/down arrows: Scroll up/down one line
  - Page Up/Page Down: Scroll up/down one page

#### Quit dialog

If there are any workers that are not in the `Idle` state when the user requests that the program exit, we should bring up a dialog that says "There are still active workers. Really quit?", with "Cancel" and "Quit" buttons. The cancel button dismisses the dialog; the quit button exits the program.

If all workers are idle, just exit the program immediately without showing the dialog.

#### Help dialog

Shows a dialog that lists all the keybindings in the app. Has an "Okay" button that dismisses the dialog.

## `code-factory-worker` command-line tool

This command-line tool handles the management of a single Claude agent using the `acp-go-sdk` library (https://github.com/coder/acp-go-sdk). It will continually claim a ticket, repeatedly run an agent for each phase of the ticket's lifecycle, then merge its changes and release the ticket.

FIXME: If we're using ACP in a separate process, how do we get the "agent has a question" notification to the parent `code-factory`?



### Ticket worker lifecycle

For each ticket, the worker will go through the following phases:

- Plan
- Implement
- Refactor
- Review
- Respond to review comments
- Give user an opportunity to test it to ensure it's what they wanted
- Merge into parent

Stages: blocked, open, planning, planned, implementing, refactor-ready, refactoring, review-ready, reviewing, responding, merge-ready, done





Hmm.

The goal is to have a system that works entirely autonomously except when it needs input from the user. The ideal UI would be a tmux session with:
  - A status window that shows the number of workers, what each is working on, the number of open tickets, etc. An interactable bit at the bottom shows a list of workers that need attention, and what they need attention for.
    - Ready for testing/approval: Press enter to open a new Cursor window in that worktree. Press 'A' to approve, 'T' to open a new iTerm tab in that worktree.
    - Agent has a question: Press enter to be switched to that agent's tmux session.

There's a key (D?) to detach the tmux session and exit. If a detached tmux session is running, it connects to that instead of starting a new one. (Per-repository, though.)

Ticket worker lifecycle:

- Plan
- Implement
- Refactor
- Review
- Respond to review comments
- Give user an opportunity to test it to ensure it's what they wanted
- Merge into parent

Stages: blocked, open, planning, planned, implementing, review-ready, reviewing, responding, refactor-ready, refactoring, merge-ready, done
