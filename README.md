# code-factory

A self-hosted AI coding agent manager. `code-factory` runs a pool of [Claude Code](https://claude.ai/code) agents that automatically work through your project's tickets — writing code, refactoring, reviewing, and responding to change requests — while you supervise from a terminal UI.

Tickets live in a `.tickets/` directory inside your repository alongside your code. Each ticket gets its own git branch and worktree, so agent work is always isolated and reviewable before merging.

---

## How it works

1. **Define work** — Create projects and tickets by running the `/cf-project` skill on your design doc. Each ticket has an identifier, a description, and an optional list of dependencies.
2. **Run agents** — Start `code-factory` to spawn a pool of Claude Code workers. Workers claim idle tickets, run the appropriate agent prompt, and advance tickets through a four-phase pipeline.
3. **Supervise** — Watch progress in the terminal UI. Approve work, respond to agent questions, request changes, and merge completed tickets.

### Ticket phases

Each ticket moves through four phases before it is done:

| Phase | What the agent does |
|-------|-------------------|
| `implement` | Writes specs first, then implements the ticket |
| `refactor` | Refactors and cleans up the resulting code |
| `review` | Reviews the refactored changes and makes change requests |
| `respond` | Applies any open change requests |

You approve each phase transition. When all phases are done, the ticket's branch is merged into its parent project's worktree (or the repo's default branch) and the worktree is removed.

---

## Getting started

### Prerequisites

- Go 1.21+
- [Claude Code](https://claude.ai/code) installed and authenticated
- A git repository to work in

### Install

```sh
git clone https://github.com/fimmtiu/code-factory
cd code-factory
make install
```

This builds and installs three binaries to `~/bin/` and installs the Claude Code skills to `~/.claude/skills/`.

### Set up a repository

```sh
cd your-project
tickets init
```

This creates `.tickets/` with a default `settings.json`. Edit `settings.json` to configure your editor:

```json
{
  "editor": "cursor"
}
```

Supported editors: `cursor`, `vscode`. (For more settings, see [the `code-factory` README](cmd/code-factory/README.md).)

### Create some work

Write a specification, then run the `/cf-project` skill on it to decompose it into projects, subprojects, and tickets. (This is temporary; eventually we'll want to fold a more powerful project-decomposer into code-factory itself.)

### Start the agent manager

```sh
code-factory
```

Workers will immediately start claiming and working on idle tickets. See `cmd/code-factory/README.md` for all options.

---

## Terminal UI

The TUI has four views (switch with F1–F4 or Shift+Tab):

- **F1: Projects** — Hierarchical tree of all work units, with a status pane and detail view. Press Enter on a ticket to see its change requests and logfiles.
- **F2: Commands** — Actionable tickets waiting for your input (`needs-attention`) or review (`user-review`). Press A to approve, R to respond to an agent question, D to open a debug prompt.
- **F3: Workers** — Live view of each agent worker: status, current output, and activity.
- **F4: Log** — Timestamped history of all worker actions with access to raw agent logfiles.

Here are some screenshots, though the terminal UI is in flux and these will be out of date quickly. (Note that the tickets and comments are all randomly generated placeholders from the `cf-testdata` program and are not expected to make sense.)

| Projects view | Agent logs | Change requests |
|    :---:      |     :---:  |    :---:        |
| ![Screenshot of projects view](img/projects_view.png) | ![Screenshot of agent logs dialog](img/agent_logs.png)  | ![Screenshot of change requests dialog](img/change_requests.png) |

| Commands view | Workers view | Log view |
|    :---:      |     :---:  |    :---:        |
| ![Screenshot of commands view](img/commands_view.png) | ![Screenshot of workers view](img/workers_view.png)  | ![Screenshot of log view](img/log_view.png) |

---

## Binaries

| Binary | Purpose |
|--------|---------|
| `code-factory` | Terminal UI agent manager — the main program |
| `tickets` | CLI for managing projects, tickets, and change requests |
| `cf-testdata` | Generates test data for UI development and testing |

See each binary's README in `cmd/` for full documentation.

---

## Claude Code skills

The `skills/` directory contains Claude Code skills that are installed to `~/.claude/skills/` by `make install`. These are used by agent workers during the refactor, review, and respond phases:

| Skill | Trigger | Purpose |
|-------|---------|---------|
| `cf-refactor` | `/cf-refactor` | Scan and refactor recent changes for code smells |
| `cf-review` | `/cf-review` | Thorough multi-perspective code review |
| `cf-respond` | `/cf-respond` | Apply change requests to a ticket's worktree |
| `cf-project` | `/cf-project` | Decompose a large project into tickets |
| `cf-clarify` | `/cf-clarify` | Identify underspecified parts of a design document |

---

## Development

```sh
make build    # Build all three binaries
make test     # Run the test suite
make lint     # Run go vet and gofmt
make clean    # Remove built binaries
make install  # Build, install to ~/bin/, and install skills
```

For UI testing without running real agents:

```sh
cf-testdata -reset        # Generate test data
code-factory --mock       # Run with fake workers
```

---

## Repository layout

```
cmd/
  code-factory/   Terminal UI agent manager
  tickets/        CLI management tool
  cf-testdata/    Test data generator
internal/
  config/         Settings loading and validation
  db/             SQLite database layer
  gitutil/        Git worktree operations
  models/         Domain types (WorkUnit, ChangeRequest, etc.)
  storage/        Path utilities and .tickets/ initialisation
  ui/             Bubbletea terminal UI
  util/           Shared utilities (editor, clipboard, terminal)
  worker/         Agent worker pool and ACP integration
  workflow/       Ticket approval and phase-transition logic
skills/           Claude Code skills installed alongside the binaries
rules/            Cursor rules installed to ~/.cursor/rules/
```
