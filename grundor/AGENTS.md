# AGENTS.md — Developer and Agent Context

## Project Structure

- `cmd/tickets/` — The main CLI binary for managing tickets and projects
- `cmd/code-factory/` — The agent-manager binary (pool + TUI); builds with `go build ./cmd/code-factory`
- `internal/db/` — SQLite DB access layer; all writes go through `withTx`
- `internal/models/` — Shared data structs (WorkUnit, LogEntry, etc.)
- `internal/storage/` — Path utilities and `.tickets/` directory initialization
- `internal/util/` — Shared utilities: editor invocation, terminal opening, clipboard
- `internal/worker/` — Worker pool data model: `Worker`, `Pool`, `WorkerStatus`, message types, `LogMessage`

## Key Conventions

### DB schema changes
- Add new `CREATE TABLE IF NOT EXISTS` or `CREATE INDEX IF NOT EXISTS` statements to the `schemaStatements` slice in `internal/db/db.go`
- All DB writes use `d.withTx(func(tx *sql.Tx) error { ... })`
- Tests use `openTestDB(t)` from `db_test.go` which injects a fake git client

### Subprocess execution
- Always use `os/exec`, never `tea.ExecProcess`
- `$EDITOR` may contain arguments (e.g. `"vim -u NONE"`); use `strings.Fields(editor)` to split before building `exec.Cmd`
- Fire-and-forget processes use `.Start()`; processes you need to wait on use `.Run()`

### CLI binaries
- `cmd/tickets` uses a hand-rolled subcommand parser (check `os.Args[1]`)
- `cmd/code-factory` uses the standard `flag` package with `-p`/`--pool` and `-w`/`--wait` flags
- Startup must verify: (1) inside a git repo via `storage.FindRepoRoot`, (2) `.tickets/` directory exists

### Makefile
- `make build` builds all three binaries: `tickets`, `tickets-testdata`, `code-factory`
- `make lint` runs `go vet` and `gofmt -w .`
- `make test` runs `go test ./...`

## internal/worker package

- `WorkerStatus` is a typed `string` enum; use `StatusIdle`, `StatusAwaitingResponse`, `StatusBusy`
- `Worker` uses two separate typed channels: `ToWorker chan MainToWorkerMessage` and `FromWorker chan WorkerToMainMessage`
- Message kinds are typed string constants (`MainToWorkerKind`, `WorkerToMainKind`) — not plain strings
- `Pool.GetWorker(n)` uses 1-based numbering; returns nil for out-of-range values
- Log channel buffer is 100 to avoid blocking workers during log bursts
- `Pool` holds a `context.Context`/`cancel`/`sync.WaitGroup` created at `NewPool` time; `Start()` and `StartHousekeeping()` add to the WaitGroup; `Stop()` cancels the context and waits
- Worker main loop calls `drainMessages()` at the top of each iteration so pause/unpause messages sent before `Start()` (or between iterations) take effect immediately
- `Pool.PauseWorker(n)` / `UnpauseWorker(n)` send `MsgPause`/`MsgUnpause` to the worker's `ToWorker` channel; out-of-range numbers are silently ignored
- `db.FindStaleTickets(thresholdMinutes int)` returns tickets where `status = 'in-progress'` and `last_updated < now - threshold`; `Claim()` alone does NOT set status to in-progress — the worker calls `db.SetStatus(..., "in-progress")` explicitly after claiming
- Tests for stale ticket detection must call `db.SetStatus` to set status=in-progress before querying; use a negative threshold (e.g. -1) to make freshly-updated tickets appear stale in tests

## internal/util package

- `EditText(content string) (string, error)` — opens `$EDITOR` on a temp file; errors if `$EDITOR` unset
- `OpenTerminal(dir string) error` — fires `open -a iTerm <dir>` and returns immediately (`.Start()`)
- `CopyToClipboard(text string) error` — pipes text to `pbcopy` and waits for completion (`.Run()`)
