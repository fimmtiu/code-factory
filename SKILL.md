# tickets — AI Agent Usage Guide

This document teaches AI agents how to use the `tickets` CLI to manage work items in a git repository.

## Overview

`tickets` is a task-tracking system that lives inside a git repository. It stores projects and tickets as files in a `.tickets/` directory at the repo root. A background daemon process handles all reads and writes; the CLI communicates with it over a Unix socket. Most CLI commands auto-start the daemon if it is not already running.

Use `tickets` when you need to:
- Break down a large task into named, trackable work items
- Claim a unit of work so other agents know you are working on it
- Signal when a unit of work is ready for review or is complete

## Setup

### Initialize a repository

Run once in any git repository before using other commands:

```
tickets init
```

Expected output:
```
Initialized .tickets/ in /path/to/your/repo
```

This creates the `.tickets/` directory. Commit it to version control so all agents share the same state.

## Identifiers

All projects and tickets are referenced by **kebab-case slug identifiers** (lowercase letters, digits, and hyphens). Ticket identifiers are nested under their project: `project-slug/ticket-slug`.

Examples of valid identifiers:
- `my-feature` (a project)
- `fix-auth-bug` (a project)
- `my-feature/add-login-form` (a ticket inside the `my-feature` project)
- `fix-auth-bug/validate-token` (a ticket inside the `fix-auth-bug` project)

## Commands

### Create a project

```
echo '{"description": "Detailed description of the project"}' | tickets create-project <project-slug>
```

Example:

```
echo '{"description": "Add user authentication to the web app, including login, logout, and password reset flows."}' | tickets create-project auth-feature
```

Expected output:
```json
{
  "id": "auth-feature",
  "description": "Add user authentication to the web app, including login, logout, and password reset flows.",
  "status": "open"
}
```

The JSON on stdin must contain a `description` field. No other fields are used.

### Create a ticket

```
echo '{"description": "...", "dependencies": ["dep-ticket-id"]}' | tickets create-ticket <project-slug>/<ticket-slug>
```

The `dependencies` field is optional. If provided, it is a list of ticket identifiers (in `project/ticket` form) that must be completed before this ticket can be worked on.

Example without dependencies:

```
echo '{"description": "Implement the login endpoint: POST /auth/login. Accept email and password, return a JWT on success."}' | tickets create-ticket auth-feature/login-endpoint
```

Example with dependencies:

```
echo '{"description": "Implement the logout endpoint: POST /auth/logout. Invalidate the user session.", "dependencies": ["auth-feature/login-endpoint"]}' | tickets create-ticket auth-feature/logout-endpoint
```

Expected output for either:
```json
{
  "id": "auth-feature/login-endpoint",
  "description": "Implement the login endpoint: POST /auth/login. Accept email and password, return a JWT on success.",
  "status": "open",
  "dependencies": []
}
```

### Get work

Claim the next available open ticket. The daemon selects a ticket whose dependencies are all complete and marks it as in-progress.

```
tickets get-work
```

Expected output:
```json
{
  "id": "auth-feature/login-endpoint",
  "description": "Implement the login endpoint: POST /auth/login. Accept email and password, return a JWT on success.",
  "status": "in_progress"
}
```

If there is no available work, the output will indicate no ticket was returned.

### Mark a ticket ready for review

When you finish implementing a ticket, signal that it is ready for review:

```
tickets review-ready <project-slug>/<ticket-slug>
```

Example:

```
tickets review-ready auth-feature/login-endpoint
```

Expected output:
```json
{
  "id": "auth-feature/login-endpoint",
  "status": "review_ready"
}
```

### Claim a review

Claim the next ticket that is ready for review:

```
tickets get-review
```

Expected output:
```json
{
  "id": "auth-feature/login-endpoint",
  "description": "Implement the login endpoint: POST /auth/login. Accept email and password, return a JWT on success.",
  "status": "in_review"
}
```

### Mark a ticket done

After reviewing and approving a ticket, mark it complete:

```
tickets done <project-slug>/<ticket-slug>
```

Example:

```
tickets done auth-feature/login-endpoint
```

Expected output:
```json
{
  "id": "auth-feature/login-endpoint",
  "status": "done"
}
```

### Check status

List all projects and tickets with their current statuses:

```
tickets status
```

Expected output (JSON array of work units):
```json
[
  {
    "id": "auth-feature",
    "description": "Add user authentication to the web app...",
    "status": "in_progress",
    "children": [
      {
        "id": "auth-feature/login-endpoint",
        "status": "done"
      },
      {
        "id": "auth-feature/logout-endpoint",
        "status": "in_progress"
      }
    ]
  }
]
```

### Check if the daemon is running

```
tickets running
```

Expected output when running:
```
Daemon is running (pid 12345)
```

Expected output when not running:
```
No daemon running
```

### Stop the daemon

```
tickets exit
```

Expected output:
```
Daemon exiting
```

The daemon auto-starts on the next command that needs it, so manual shutdown is rarely needed.

## Full Example Workflow

This example shows an end-to-end flow: creating a project with two tickets, doing the work, and completing the review cycle.

```sh
# 1. Initialize (only needed once per repo)
tickets init

# 2. Create a project
echo '{"description": "Add user authentication: login, logout, password reset."}' \
  | tickets create-project auth-feature

# 3. Create tickets (logout depends on login being done first)
echo '{"description": "Implement POST /auth/login — accept email+password, return JWT."}' \
  | tickets create-ticket auth-feature/login-endpoint

echo '{"description": "Implement POST /auth/logout — invalidate session.", "dependencies": ["auth-feature/login-endpoint"]}' \
  | tickets create-ticket auth-feature/logout-endpoint

# 4. Claim work — daemon returns the first available ticket (login-endpoint)
tickets get-work
# {"id": "auth-feature/login-endpoint", "status": "in_progress", ...}

# 5. Do the implementation work...

# 6. Signal implementation is complete and ready for review
tickets review-ready auth-feature/login-endpoint
# {"id": "auth-feature/login-endpoint", "status": "review_ready"}

# 7. A reviewer claims the review
tickets get-review
# {"id": "auth-feature/login-endpoint", "status": "in_review", ...}

# 8. Review passes — mark done
tickets done auth-feature/login-endpoint
# {"id": "auth-feature/login-endpoint", "status": "done"}

# 9. Now logout-endpoint is unblocked — claim it
tickets get-work
# {"id": "auth-feature/logout-endpoint", "status": "in_progress", ...}

# 10. Check overall progress at any time
tickets status
```

## Notes for AI Agents

- Always run `tickets init` before any other command in a fresh repository.
- Use `tickets status` to understand the current state before claiming work.
- Identifiers are permanent — choose descriptive kebab-case slugs that describe the work.
- The `dependencies` field in `create-ticket` accepts ticket identifiers in `project/ticket` form; tickets with unmet dependencies will not be returned by `get-work`.
- The daemon starts automatically when needed; you do not need to manage it explicitly.
- All commands print JSON to stdout on success, making output easy to parse programmatically.
