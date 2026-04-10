# tickets

A command-line tool for managing the work units (projects and tickets) stored in a repository's `.tickets/` directory.

## Usage

```
tickets <subcommand> [args]
```

Must be run from inside a git repository that has been initialised with `tickets init`.

## Subcommands

### Initialisation

```
tickets init
```

Creates the `.tickets/` directory and a default `settings.json` in the current git repository. Safe to run multiple times (idempotent).

### Querying state

```
tickets status
```

Prints all projects and tickets as pretty-printed JSON, including phase, status, dependencies, and change requests.

```
tickets list-crs <identifier>
```

Prints the open change requests for the given ticket as a JSON array. (Dismissed or closed CRs aren't shown.)

### Creating work units

```
echo '{"description": "...", "dependencies": ["other/ticket"]}' | tickets create-project <identifier>
echo '{"description": "...", "dependencies": ["other/ticket"]}' | tickets create-ticket <identifier>
```

Creates a project or ticket with the given slash-separated identifier (e.g. `my-project/my-ticket`). The JSON body is read from stdin and must include a `description` field. `dependencies` is optional. Creating a ticket immediately creates a git worktree for it.

### Updating state

```
tickets set-status <identifier> <phase> [<status>]
```

Updates a ticket's phase (e.g. `implement`, `refactor`, `review`, `respond`, `done`) and optionally its status (defaults to `idle`). Setting phase to `done` automatically merges the ticket's worktree into its parent project's worktree (or the repo's default branch FIXME FIXME) and removes the worktree.

### Worker protocol

These subcommands are used by `code-factory` workers and are rarely called directly.

```
tickets claim <pid>
```

Atomically claims the next available `idle` ticket for the given process ID and prints it as JSON. Returns an error if no ticket is available.

```
tickets release <identifier>
```

Releases the claim on a ticket, returning it to the `idle` status.

### Change requests

```
tickets create-cr <identifier> <code-location> <author> <description>
```

Adds an open change request to a ticket. `code-location` must be in `file:line` format (e.g. `internal/db/db.go:42`). The commit hash is recorded automatically from the ticket's worktree HEAD.

```
tickets close-cr <id> [<explanation>]
tickets dismiss-cr <id> [<explanation>]
```

Closes or dismisses the change request with the given numeric ID. The optional explanation, if provided, is appended to the change request's description before closing.

## Settings

On startup, `tickets` reads `settings.json` from the `.tickets/` directory. See `internal/config` for available fields. If the file does not exist, defaults are used.
