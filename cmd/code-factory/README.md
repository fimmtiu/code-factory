# code-factory

A terminal UI application that manages a pool of Claude Code agents to automatically work through the tickets in a repository's `.code-factory/` directory. Workers claim tickets, run the appropriate agent prompt for each phase, and advance tickets through the implement → refactor → review → done pipeline. After each phase, if reviewers have filed change requests, a `/cf-respond` run is interleaved to address them before the ticket advances.

## Prerequisites

- Run `cf-tickets init` in the repository first to create `.code-factory/`.
- Set `editor` in `.code-factory/settings.json` to `"cursor"` or `"vscode"` (default: `"cursor"`).
- Set `terminal` in `.code-factory/settings.json` to `"iterm2"`, `"terminal"`, or `"cmux"` (default: `"iterm2"`).

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

The TUI has five views, switched with F1–F5 or Shift-Tab / Ctrl-Tab:

| Key | View | Description |
|-----|------|-------------|
| F1 | Projects | Hierarchical tree of all projects and tickets with a status pane and detail pane |
| F2 | Commands | Actionable tickets (`needs-attention` and `user-review`), with controls to respond, approve, and debug |
| F3 | Workers | Real-time view of each worker's status and recent agent output |
| F4 | Diffs | Interactive view of a ticket's commit history with a diff viewer |
| F5 | Log | Timestamped log of all worker actions, with access to raw logfiles |

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
1. Claims the next available `idle` or `responding` ticket from the database.
2. Sets the ticket to `working` (or leaves it `responding`) and runs the appropriate agent prompt via Claude Code ACP — the phase skill for `idle` tickets, `/cf-respond` for `responding` tickets.
3. On completion, sets the ticket to `user-review` and releases it.
4. Waits for the user to approve via `A`. If the ticket has open change requests, approval sends it back to `responding` to address them; otherwise it advances to the next phase.

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
  "terminal": "iterm2",
  "model_implement": "sonnet",
  "model_refactor": "opus",
  "model_review": "opus",
  "model_respond": "opus",
  "effort": "high",
  "terminal_theme": "tan"
}
```

| Setting | Default | Description |
|---------|---------|-------------|
| `stale_threshold_minutes` | `30` | Minutes before an in-progress ticket is considered abandoned |
| `editor` | `"cursor"` | Editor to open worktrees in (`"cursor"` or `"vscode"`) |
| `terminal` | `"iterm2"` | Terminal emulator for opening worktrees and notifications (`"iterm2"`, `"terminal"`, or `"cmux"`) |
| `model_implement` | `"sonnet"` | Claude model for the implementation phase |
| `model_refactor` | `"opus"` | Claude model for the refactoring phase |
| `model_review` | `"opus"` | Claude model for the review phase |
| `model_respond` | `"opus"` | Claude model used when responding to change requests |
| `effort` | `"high"` | Effort level for the agent (`"low"`, `"normal"`, `"high"`, or `"max"`) |
| `terminal_theme` | `"tan"` | Terminal colour theme (`"tan"`, `"dark"`, or `"light"`) |
