# code-factory

A terminal UI application that manages a pool of Claude Code agents to automatically work through the tickets in a repository's `.code-factory/` directory. Workers claim tickets, run the appropriate agent prompt for each phase, and advance tickets through the implement → refactor → review → respond → done pipeline.

## Prerequisites

- Run `cf-tickets init` in the repository first to create `.code-factory/`.
- Set `editor` in `.code-factory/settings.json` to `"cursor"` or `"vscode"` (default: `"cursor"`).

## Usage

```
code-factory [-p <pool>] [-w <wait>] [--mock]
```

Must be run from inside a git repository containing a `.code-factory/` directory.

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--pool` | `4` | Number of parallel agent workers |
| `-w`, `--wait` | `5` | Seconds between polls for available tickets |
| `--mock` | off | Use fake workers instead of real Claude subprocesses (useful for UI testing) |

## Views

The TUI has four views, switched with F1–F4 or Shift-Tab / Ctrl-Tab:

| Key | View | Description |
|-----|------|-------------|
| F1 | Projects | Hierarchical tree of all projects and tickets with a status pane and detail pane |
| F2 | Commands | Actionable tickets (`needs-attention` and `user-review`), with controls to respond, approve, and debug |
| F3 | Workers | Real-time view of each worker's status and recent agent output |
| F4 | Log | Timestamped log of all worker actions, with access to raw logfiles |

## Key bindings (Commands view)

| Key | Action |
|-----|--------|
| R | Respond to an agent's question (pre-fills a template with recent output) |
| A | Approve a `user-review` ticket (advances it to the next phase) |
| D | Open a debug prompt template in the editor and launch `claude` on it |
| E | Open the ticket's worktree in the configured non-blocking editor |
| T | Open a terminal window in the ticket's worktree |
| Enter | Open the ticket dialog (change requests and logfiles) |

## Worker lifecycle

Each worker continuously:
1. Claims the next available `idle` ticket from the database.
2. Sets the ticket to `in-progress` and runs the phase-appropriate agent prompt via Claude Code ACP.
3. On completion, sets the ticket to `user-review` and releases it.
4. Waits for the user to approve via `A` before advancing to the next phase.

If an agent asks a question or requests a permission, the ticket becomes `needs-attention` and the worker pauses until the user responds with `R`.

### Logfiles

Each agent run produces a logfile at `.code-factory/<identifier>/<phase>.log`. Multiple runs produce `.log.1`, `.log.2`, etc. Logfiles include the session ID (for `--resume`), the full prompt, and all agent output.

## Configuration

Settings are read from `.code-factory/settings.json` at startup.

```jsonc
// .code-factory/settings.json — all fields are optional; defaults shown below
{
  "stale_threshold_minutes": 30,
  "editor": "cursor",
  "open_terminal_command": "open -a iTerm .",
  "model_implement": "sonnet",
  "model_refactor": "opus",
  "model_review": "opus",
  "model_respond": "sonnet",
  "effort": "high"
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `stale_threshold_minutes` | `30` | Minutes before an in-progress ticket is considered abandoned |
| `editor` | `"cursor"` | Editor to open worktrees in (`"cursor"` or `"vscode"`) |
| `open_terminal_command` | `"open -a iTerm ."` | Shell command to open a terminal in the worktree directory |
| `model_implement` | `"sonnet"` | Claude model for the implementation phase |
| `model_refactor` | `"opus"` | Claude model for the refactoring phase |
| `model_review` | `"opus"` | Claude model for the review phase |
| `model_respond` | `"sonnet"` | Claude model for the response phase |
| `effort` | `"high"` | Effort level for the agent (`"low"`, `"normal"`, `"high"`, or `"max"`) |
