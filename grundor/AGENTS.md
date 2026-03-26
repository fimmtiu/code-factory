# AGENTS.md — Developer and Agent Context

## Project Structure

- `cmd/tickets/` — The main CLI binary for managing tickets and projects
- `cmd/code-factory/` — The agent-manager binary (pool + TUI); builds with `go build ./cmd/code-factory`
- `internal/db/` — SQLite DB access layer; all writes go through `withTx`
- `internal/models/` — Shared data structs (WorkUnit, LogEntry, etc.)
- `internal/storage/` — Path utilities and `.tickets/` directory initialization
- `internal/util/` — Shared utilities: editor invocation, terminal opening, clipboard

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

## internal/util package

- `EditText(content string) (string, error)` — opens `$EDITOR` on a temp file; errors if `$EDITOR` unset
- `OpenTerminal(dir string) error` — fires `open -a iTerm <dir>` and returns immediately (`.Start()`)
- `CopyToClipboard(text string) error` — pipes text to `pbcopy` and waits for completion (`.Run()`)
