# cf-testdata

Generates a realistic set of fake projects, tickets, worktrees, and change requests in a repository's `.tickets/` directory. Intended for manual UI testing and local development of the `code-factory` and `tickets` tools without needing to run real agents.

## Usage

```
cf-testdata [--seed <N>] [--target <dir>] [--reset]
```

Must be run from inside (or targeting) a git repository. The `.tickets/` directory is created automatically if it does not exist.

### Options

| Flag | Default | Description |
|------|---------|-------------|
| `--seed` | current time | Random seed for reproducible data generation |
| `--target` | `.` | Path inside the target git repository |
| `--reset` | off | Remove all existing `.tickets/` content before generating |

## What it generates

- **5–7 projects** in a 2-level hierarchy (3–4 top-level, up to 2 subprojects each), each with a randomly generated description.
- **24–28 tickets** distributed across the projects, with ~30% chance of a dependency chain between sibling tickets. Each ticket gets a git worktree immediately upon creation.
- **Change requests** on ~40% of tickets (1–3 per ticket), referencing real tracked source files at real line numbers from the repository's git history. Real commit hashes are used so the ticket dialog can display actual code context.
- **Mock logfiles** (`.tickets/<identifier>/implement.log`) for tickets that have change requests, including a session ID header in the standard `=== SESSION ===` / `=== PROMPT ===` / `=== OUTPUT ===` format.

## Source file discovery

Change request locations are drawn from the repository's actual tracked source files (`.go`, `.rb`, `.py`, `.rs`, `.c`, `.cpp`, `.h`, `.ts`, `.js`) and commit hashes from the repository's recent git history. This means the change request dialog will show real code context when you press Enter on a ticket.

## Reproducibility

Pass `--seed <N>` with the same value to regenerate identical data (same identifiers, descriptions, and structure). The seed is printed on each run.

## Example

```sh
# Generate fresh test data, wiping any existing data
cf-testdata --reset

# Regenerate with the same seed for reproducible testing
cf-testdata --reset --seed 1234567890

# Target a different repository
cf-testdata --target /path/to/other-repo --reset
```
