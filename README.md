# Tickets

A lightweight work-tracking system for git repositories, designed for coordinating work between AI coding agents and humans. Projects and tickets are stored as plain JSON files inside your repository and managed through a background daemon that multiple clients can talk to concurrently without race conditions.

## How it works

- A **daemon** (`ticketsd`) runs in the background, owns the `.tickets/` directory, and serialises all reads and writes through a single worker queue.
- A **CLI** (`tickets`) lets agents and humans create projects/tickets, claim work, and mark it done.
- A **terminal UI** (`tickets-ui`) gives a live three-pane view of all projects and tickets.
- Every in-progress ticket gets its own **git worktree** inside the `.tickets/` directory. When a ticket is marked done its branch merges into the parent project's branch; when an entire project finishes it merges into `main`.

## Installation

Requires Go 1.21+.

```sh
git clone https://github.com/fimmtiu/tickets.git
cd tickets
make install          # installs to ~/bin
# or: go install ./cmd/tickets ./cmd/ticketsd ./cmd/tickets-ui
```

## Quick start

```sh
# 1. Initialise a repository (run once)
tickets init

# 2. Create a project
echo '{"description": "Add user authentication: login, logout, password reset."}' \
  | tickets create-project auth-feature

# 3. Create tickets inside the project
echo '{"description": "Implement POST /auth/login — accept email+password, return JWT."}' \
  | tickets create-ticket auth-feature/login-endpoint

echo '{"description": "Implement POST /auth/logout — invalidate session.", "dependencies": ["auth-feature/login-endpoint"]}' \
  | tickets create-ticket auth-feature/logout-endpoint

# 4. Claim the next available ticket
tickets get-work
# → {"identifier":"auth-feature/login-endpoint","status":"in-progress",...}

# 5. Do the work, then signal it is ready for review
tickets review-ready auth-feature/login-endpoint

# 6. A reviewer claims and approves it
tickets get-review
tickets done auth-feature/login-endpoint

# 7. The dependency is now satisfied — claim the next ticket
tickets get-work
# → {"identifier":"auth-feature/logout-endpoint","status":"in-progress",...}
```

## CLI reference

### Commands that do not start the daemon

| Command | Description |
|---------|-------------|
| `tickets init` | Create `.tickets/` and default config in the current repo |
| `tickets running` | Print daemon PID or "No daemon running" |
| `tickets exit` | Ask the daemon to shut down cleanly |

### Commands that auto-start the daemon

| Command | Description |
|---------|-------------|
| `tickets status` | Print all projects and tickets as JSON |
| `tickets create-project <id>` | Create a project (reads JSON from stdin) |
| `tickets create-ticket <id>` | Create a ticket (reads JSON from stdin) |
| `tickets get-work` | Claim the next open, unblocked ticket |
| `tickets review-ready <id>` | Mark a ticket ready for review |
| `tickets get-review` | Claim the next review-ready ticket |
| `tickets done <id>` | Mark a ticket done and merge its branch |
| `tickets add-comment <id> <location> <author> <text>` | Add a comment to a ticket at a code location |
| `tickets close-thread <thread-id>` | Close a comment thread |

### Identifiers

Identifiers are kebab-case slugs. A ticket inside a project is written as `project-slug/ticket-slug`. Nesting can go arbitrarily deep: `project/subproject/ticket`.

### Stdin format for `create-project` and `create-ticket`

```json
{"description": "What needs to be done"}
```

For tickets, an optional `dependencies` array lists identifiers that must be `done` before this ticket becomes available via `get-work`:

```json
{
  "description": "What needs to be done",
  "dependencies": ["other-project/other-ticket"]
}
```

## Ticket lifecycle

```
blocked → open → in-progress → review-ready → in-review → done
```

A ticket starts as `open` (or `blocked` if it has unsatisfied dependencies). The daemon automatically unblocks tickets when their dependencies are marked done. Tickets that are `in-progress` or `in-review` for longer than `stale_threshold_minutes` (default: 30) are automatically reset by the housekeeping process so abandoned work doesn't stay locked forever.

## Project lifecycle

```
open → in-progress → done
```

A project transitions to `in-progress` automatically when any of its tickets is claimed. It transitions to `done` automatically when every ticket in it is `done`, and its branch is merged into its parent project's branch (or `main` if it's a top-level project).

## Comment threads

Tickets can carry comment threads, which are useful for attaching review notes or questions to specific lines of code.

```sh
# Add a comment (starts a new thread if none is open at that location)
tickets add-comment auth-feature/login-endpoint internal/auth/handler.go:42 \
  "Alice" "This will panic on an empty slice — add a bounds check."

# Add a follow-up comment to the same location (appends to the open thread)
tickets add-comment auth-feature/login-endpoint internal/auth/handler.go:42 \
  "Bob" "Good catch, fixed in the latest commit."

# Close the thread once resolved
tickets close-thread a3f8c1d2e4b56789
```

Each thread records the commit hash of the ticket's worktree HEAD at the time the thread was created. There can be at most one **open** thread per code location on a given ticket; additional comments at the same location are appended to that thread. Closed threads are retained for history.

Comment threads are stored in the ticket's `ticket.json` as a `comment_threads` array and are visible in the detail pane of `tickets-ui`.

## Terminal UI

```sh
tickets-ui
```

Opens a fullscreen three-pane interface:

| Pane | Position | Description |
|------|----------|-------------|
| Status | Top-left | Repository name and summary counts: total/open/in-progress/done for projects and tickets |
| Navigator | Top-right | Collapsible tree of all projects and tickets |
| Detail | Bottom | Status, dependencies, description, and comment threads of the selected item |

Each pane has a border; the focused pane is highlighted in bright blue.

**Navigator keys**: `↑`/`↓` move cursor one row, `PgUp`/`PgDn` move by a full page, `Enter` collapses/expands a project, `Tab`/`Space` switches focus to the detail pane.
**Detail keys**: `↑`/`↓` scroll one line, `PgUp`/`PgDn` scroll by a full page, `Tab`/`Space` switches focus back to the navigator.
**Global**: `Ctrl-R` refreshes immediately, `Q` or `Ctrl-C` exits.

The UI polls the daemon every 60 seconds automatically.

## Configuration

`.tickets/.settings.json` (created by `tickets init`):

```json
{
  "stale_threshold_minutes": 30,
  "exit_after_minutes": 60
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `stale_threshold_minutes` | 30 | Minutes before an `in-progress` or `in-review` ticket is reset |
| `exit_after_minutes` | 60 | Minutes of inactivity before the daemon exits automatically |

## Data layout

```
<repo-root>/
└── .tickets/
    ├── .settings.json           # configuration
    ├── .daemon.sock             # Unix socket (runtime only)
    └── my-feature/
        ├── project.json         # project metadata
        └── fix-bug/
            ├── ticket.json      # ticket metadata (status, deps, comments, …)
            └── worktree/        # git worktree, present while ticket is in-progress
```

Each ticket lives in its own subdirectory. The worktree for an in-progress ticket is created inside that same directory, keeping all ticket artefacts together.

All `.tickets/` content is intended to be committed to git so the full team (or agent pool) shares the same state.

## Architecture

```
tickets (CLI) ──┐
tickets-ui     ──┤── Unix socket ──► ticketsd (daemon)
                 │                      │
                 │                      ├── single worker goroutine
                 │                      ├── in-memory state (projects + tickets)
                 │                      ├── file I/O on .tickets/
                 │                      ├── git worktree management
                 │                      └── 60-second housekeeping timer
```

The daemon serialises all commands through a single goroutine — no concurrent writes, no locking needed on the `.tickets/` directory. Each client connection sends one JSON command and reads one JSON response before closing.

## For AI agents

See `SKILL.md` for a self-contained guide to using `tickets` from within an AI coding session, including worked examples and notes on dependency management.

## Development

```sh
make build    # build all binaries
make test     # run all tests
make lint     # vet + gofmt
make install  # install to ~/bin
make clean    # remove built binaries
```

### Generating test data

`tickets-testdata` populates `.tickets/` with a realistic set of randomly generated projects, tickets, and comment threads — useful for manually exercising `tickets-ui` or testing the daemon.

```sh
# Generate data (safe to run multiple times; adds to existing data)
tickets-testdata

# Replace all existing data with a fresh set
tickets-testdata -reset

# Reproducible output (same seed → same identifiers and descriptions)
tickets-testdata -reset -seed 42

# Target a different repository
tickets-testdata -target /path/to/other/repo
```
