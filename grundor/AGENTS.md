# AGENTS.md — Developer and Agent Context

## Project Structure

- `cmd/tickets/` — The main CLI binary for managing tickets and projects
- `cmd/code-factory/` — The agent-manager binary (pool + TUI); builds with `go build ./cmd/code-factory`
- `internal/db/` — SQLite DB access layer; all writes go through `withTx`
- `internal/models/` — Shared data structs (WorkUnit, LogEntry, etc.)
- `internal/storage/` — Path utilities and `.tickets/` directory initialization
- `internal/ui/` — Bubbletea TUI: root `Model` in `app.go`, stub view models, dialogs
- `internal/util/` — Shared utilities: editor invocation, terminal opening, clipboard
- `internal/worker/` — Worker pool data model: `Worker`, `Pool`, `WorkerStatus`, message types, `LogMessage`

## Key Conventions

### DB schema changes
- Add new `CREATE TABLE IF NOT EXISTS` or `CREATE INDEX IF NOT EXISTS` statements to the `schemaStatements` slice in `internal/db/db.go`
- All DB writes use `d.withTx(func(tx *sql.Tx) error { ... })`
- Tests use `openTestDB(t)` from `db_test.go` which injects a fake git client
- `db.TicketStats()` returns `db.TicketStats{Total, Done}` counts
- `db.UpdateDescription(identifier, newDescription)` updates tickets or projects table (tries tickets first, falls back to projects)

### Subprocess execution
- Always use `os/exec`, never `tea.ExecProcess`
- `$EDITOR` may contain arguments (e.g. `"vim -u NONE"`); use `strings.Fields(editor)` to split before building `exec.Cmd`
- Fire-and-forget processes use `.Start()`; processes you need to wait on use `.Run()`

### CLI binaries
- `cmd/tickets` uses a hand-rolled subcommand parser (check `os.Args[1]`)
- `cmd/code-factory` uses the standard `flag` package with `-p`/`--pool` and `-w`/`--wait` flags
- Startup must verify: (1) inside a git repo via `storage.FindRepoRoot`, (2) `.tickets/` directory exists

### Git: cmd/code-factory source files
- `.gitignore` has a bare `code-factory` entry that also matches the `cmd/code-factory/` directory path, causing `git add` to refuse it
- Use `git add -f cmd/code-factory/main.go` (or any file inside that directory)

### internal/ui package (PRD-05, PRD-06)
- Root model: `ui.NewModel(pool, database, waitSecs)` — use `tea.NewProgram(model, tea.WithAltScreen())`
- `KeyBinding{Key, Description}` + `globalKeyBindings` in `keybinding.go`; each view returns its own via `KeyBindings() []KeyBinding`
- Dialog dismiss is message-based: dialog sends `dismissDialogCmd()` → root model sets `m.dialog = nil`
- `renderCenteredOverlay` in `dialogs.go` merges the dialog string over the background at the terminal centre
- View cycle (Shift-Tab = next, Ctrl-Tab = prev): implemented via `nextView`/`prevView` helpers in `views.go`
- Sub-model `Init()` cmds must be collected and batched in the root `Init()` — bubbletea only auto-calls the root model's `Init()`
- `tea.WindowSizeMsg` must be forwarded to all views (not just the active one) so each can compute layout
- `ProjectView` uses a `chromeHeight = 2` constant to subtract the tab-bar and help-hint rows from the available body height
- Three-pane layout: `lipgloss.JoinHorizontal` for top row (status + tree), `lipgloss.JoinVertical` for full layout; border style switches between `DoubleBorder` (focused, blue) and `NormalBorder` (unfocused, grey) based on focus state
- `CommandView` (PRD-07): `NewCommandView(database, pool, waitSecs)` — full-screen selectable list of actionable tickets; `Approve(db, identifier)` placeholder lives in `command_view.go` for phase 11 to replace
- `WorkerView` (PRD-08): `NewWorkerView(pool)` — read-only monitoring view; 500ms `tea.Tick` with `workerTickMsg`; linesPerWorker=5 (status+3 output+separator); scroll only (no selection)
- Non-key messages only go to the active view; tick-based refresh chains (fetch → schedule next tick → on tick, fetch again) only run while a view is active
- `ChangeRequestDialog` (PRD-10): opened via `openChangeRequestDialogMsg{wu}` from ProjectView/CommandView → root model creates `NewChangeRequestDialog(db, wu, width, height)`; when a dialog is open, non-key messages are routed to the dialog (not the active view) so its own internal update messages (e.g. `crStatusChangedMsg`) arrive correctly; code context uses `git show <hash>:<file>` in the worktree dir

### Makefile
- `make build` builds all three binaries: `tickets`, `tickets-testdata`, `code-factory`
- `make lint` runs `go vet` and `gofmt -w .`
- `make test` runs `go test ./...`

## internal/worker package

- `WorkerStatus` is a typed `string` enum; use `StatusIdle`, `StatusAwaitingResponse`, `StatusBusy`
- `Worker.LastOutput` is protected by `Worker.mu` (sync.RWMutex); use `GetLastOutput()` to read and `SetLastOutput()` to write from any goroutine
- `Worker` uses two separate typed channels: `ToWorker chan MainToWorkerMessage` and `FromWorker chan WorkerToMainMessage`
- Message kinds are typed string constants (`MainToWorkerKind`, `WorkerToMainKind`) — not plain strings
- `Pool.GetWorker(n)` uses 1-based numbering; returns nil for out-of-range values
- Log channel buffer is 100 to avoid blocking workers during log bursts
- `Pool` holds a `context.Context`/`cancel`/`sync.WaitGroup` created at `NewPool` time; `Start()` and `StartHousekeeping()` add to the WaitGroup; `Stop()` cancels the context and waits
- Worker main loop calls `drainMessages()` at the top of each iteration so pause/unpause messages sent before `Start()` (or between iterations) take effect immediately
- `Pool.PauseWorker(n)` / `UnpauseWorker(n)` send `MsgPause`/`MsgUnpause` to the worker's `ToWorker` channel; out-of-range numbers are silently ignored
- `db.FindStaleTickets(thresholdMinutes int)` returns tickets where `status = 'in-progress'` and `last_updated < now - threshold`; `Claim()` alone does NOT set status to in-progress — the worker calls `db.SetStatus(..., "in-progress")` explicitly after claiming
- Tests for stale ticket detection must call `db.SetStatus` to set status=in-progress before querying; use a negative threshold (e.g. -1) to make freshly-updated tickets appear stale in tests

### ACP integration (PRD-04)

- Claude is launched via `npx -y @zed-industries/claude-code-acp@latest`; set `cmd.Dir` to the worktree path — NEVER call `os.Chdir`
- `acpWorkerClient` in `acp.go` implements `acp.Client`; it captures the worker, logfile, and db references needed during callbacks
- `RequestPermission` callback sets worker to `AwaitingResponse`, marks ticket `needs-attention`, sends `MsgPermissionRequest` on `FromWorker`, and blocks on `ToWorker` for `MsgPermission`
- `BuildPrompt` in `prompt.go` generates phase-specific prompts; implement phase appends parent context via `db.GetProjectContext`
- `NextLogfilePath` in `logfile.go` returns `.tickets/<id>/<phase>.log` with `.1`, `.2`, … suffixes for subsequent runs
- `db.GetProjectContext(identifier)` returns `[]db.ProjectContext` walking up the parent chain (immediate parent first)
- The `dbInterface` in `acp.go` is a minimal interface over `*db.DB` to allow test substitution without importing the full db package in client code

## internal/workflow package (PRD-11)

- `workflow.Approve(db, identifier) error` — advances a ticket through its phases: implement → refactor → review → respond → done
- After marking a ticket done, it recursively walks up the parent project chain and marks each project done via `db.SetProjectPhase` when `db.AllChildrenDone` returns true
- `db.GetTicketPhase(identifier)` — returns current phase of a ticket
- `db.SetProjectPhase(identifier, phase)` — updates a project's phase column
- `db.AllChildrenDone(projectIdentifier)` — returns true only when ALL direct children (tickets + subprojects) have phase "done" AND at least one child exists

## internal/util package

- `EditText(content string) (string, error)` — opens `$EDITOR` on a temp file; errors if `$EDITOR` unset
- `OpenTerminal(dir string) error` — fires `open -a iTerm <dir>` and returns immediately (`.Start()`)
- `CopyToClipboard(text string) error` — pipes text to `pbcopy` and waits for completion (`.Run()`)
