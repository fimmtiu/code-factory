// Package db provides access to the tickets SQLite database.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

const (
	workUnitTypeTicket  = 1
	workUnitTypeProject = 2
)

// MergeConflictError is returned when a git merge fails during ticket or
// project completion. It carries the path to the worktree where the conflict
// exists so the UI can offer to open a terminal there.
type MergeConflictError struct {
	WorktreePath string
	Branch       string
	Err          error
}

func (e *MergeConflictError) Error() string {
	return fmt.Sprintf("merge conflict in %s: %v", e.WorktreePath, e.Err)
}

func (e *MergeConflictError) Unwrap() error {
	return e.Err
}

// MergeUnresolvedError is returned by MergeChain when a rebase conflict
// could not be resolved by the onConflict callback (the worktree is still
// dirty after the callback returned, or the retry rebase failed). The
// caller should typically transition the ticket to user-review and notify
// the user.
type MergeUnresolvedError struct {
	Identifier   string
	WorktreePath string
}

func (e *MergeUnresolvedError) Error() string {
	return fmt.Sprintf("merge conflict on %s remains unresolved (worktree %s)", e.Identifier, e.WorktreePath)
}

// ForbiddenMarkersError is returned when a ticket's diff against its parent
// branch contains incomplete-work markers (TODO/FIXME/XXX/panic-stubs) in
// non-test files. Markers are listed as "path:line: text" so the UI can
// surface them as a jump-to-source list. The user must remove or rephrase
// the markers in the worktree, then retry the done transition.
type ForbiddenMarkersError struct {
	Identifier   string
	WorktreePath string
	Markers      []string
}

func (e *ForbiddenMarkersError) Error() string {
	plural := "marker"
	if len(e.Markers) != 1 {
		plural = "markers"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s cannot be merged: %d incomplete-work %s in %s\n",
		e.Identifier, len(e.Markers), plural, e.WorktreePath)
	for _, m := range e.Markers {
		b.WriteString("  ")
		b.WriteString(m)
		b.WriteByte('\n')
	}
	b.WriteString("Remove or rephrase these before marking the ticket done.")
	return b.String()
}

// DB provides read/write access to the tickets SQLite database.
type DB struct {
	db         *sql.DB
	ticketsDir string
	repoRoot   string
	git        gitutil.GitClient

	mergeMu    sync.Mutex             // protects mergeLocks
	mergeLocks map[string]*sync.Mutex // per-target lock serializing merges

	onWorkAvailable func() // called when a ticket transitions to a claimable state
}

// Open opens (or creates) the tickets database at ticketsDir/data.sqlite and
// ensures the schema is initialised. repoRoot is used for git operations.
func Open(ticketsDir, repoRoot string) (*DB, error) {
	dbPath := filepath.Join(ticketsDir, "data.sqlite")
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("db.Open: %w", err)
	}

	d := &DB{
		db:         sqlDB,
		ticketsDir: ticketsDir,
		repoRoot:   repoRoot,
		git:        gitutil.NewRealGitClient(),
	}

	if err := d.createSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("db.Open: create schema: %w", err)
	}

	return d, nil
}

// SetGitClient replaces the git client used for worktree operations.
// Intended for testing.
func (d *DB) SetGitClient(gc gitutil.GitClient) {
	d.git = gc
}

// SetOnWorkAvailable registers a callback that is invoked whenever a ticket
// transitions to a claimable state (e.g. when blocked tickets are unblocked).
// This allows a worker pool to wake idle workers immediately instead of
// waiting for the next poll interval.
func (d *DB) SetOnWorkAvailable(fn func()) {
	d.onWorkAvailable = fn
}

func (d *DB) notifyWorkAvailable() {
	if d.onWorkAvailable != nil {
		d.onWorkAvailable()
	}
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// withTx executes fn inside a DEFERRED transaction, committing on success and
// rolling back on any error returned by fn.
func (d *DB) withTx(fn func(*sql.Tx) error) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

var schemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS "projects" (
		"id" integer PRIMARY KEY,
		"identifier" text NOT NULL UNIQUE,
		"description" text NOT NULL,
		"last_updated" integer NOT NULL,
		"phase" text NOT NULL DEFAULT 'open',
		"project_id" integer DEFAULT NULL,
		"parent_branch" text NOT NULL DEFAULT '',
		FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS "tickets" (
		"id" integer PRIMARY KEY,
		"identifier" text NOT NULL UNIQUE,
		"description" text NOT NULL,
		"phase" text NOT NULL DEFAULT 'implement',
		"status" text NOT NULL DEFAULT 'idle',
		"claimed_by" integer DEFAULT NULL,
		"last_updated" integer NOT NULL,
		"project_id" integer DEFAULT NULL,
		"parent_branch" text NOT NULL DEFAULT '',
		FOREIGN KEY ("project_id") REFERENCES "projects"("id") ON DELETE CASCADE
	)`,
	`CREATE TABLE IF NOT EXISTS "dependencies" (
		"id" integer PRIMARY KEY,
		"work_unit_type" integer NOT NULL,
		"work_unit_id" integer NOT NULL,
		"dependency_type" integer NOT NULL,
		"dependency_id" integer NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS "change_requests" (
		"id" integer PRIMARY KEY,
		"ticket_id" integer NOT NULL,
		"filename" text NOT NULL,
		"line_number" integer NOT NULL,
		"commit_hash" text NOT NULL DEFAULT '',
		"status" text NOT NULL DEFAULT 'open',
		"date" integer NOT NULL,
		"author" text NOT NULL,
		"description" text NOT NULL,
		FOREIGN KEY ("ticket_id") REFERENCES "tickets"("id") ON DELETE CASCADE
	)`,
	`CREATE TRIGGER IF NOT EXISTS "update_ticket_last_updated"
	AFTER UPDATE OF "phase", "status" ON "tickets"
	FOR EACH ROW
	WHEN NEW.phase != OLD.phase OR NEW.status != OLD.status
	BEGIN
		UPDATE "tickets" SET "last_updated" = unixepoch() WHERE id = NEW.id;
	END`,
	`CREATE INDEX IF NOT EXISTS "idx_projects_project_id" ON "projects"("project_id")`,
	`CREATE INDEX IF NOT EXISTS "idx_tickets_project_id" ON "tickets"("project_id")`,
	`CREATE INDEX IF NOT EXISTS "idx_tickets_status" ON "tickets"("status")`,
	`CREATE INDEX IF NOT EXISTS "idx_deps_work_unit" ON "dependencies"("work_unit_type", "work_unit_id")`,
	`CREATE INDEX IF NOT EXISTS "idx_change_requests_ticket_id" ON "change_requests"("ticket_id")`,
	`CREATE TABLE IF NOT EXISTS "logs" (
		"id" integer PRIMARY KEY,
		"timestamp" integer NOT NULL,
		"worker_number" integer NOT NULL,
		"message" text NOT NULL,
		"logfile" text
	)`,
	`CREATE INDEX IF NOT EXISTS "idx_logs_timestamp" ON "logs"("timestamp")`,
}

// migrations are ALTER TABLE statements run after the schema is created to
// handle columns added after initial deployment. Each entry is idempotent:
// we ignore "duplicate column name" errors so they are safe to re-run.
var migrations = []string{
	`ALTER TABLE "projects" ADD COLUMN "parent_branch" text NOT NULL DEFAULT ''`,
	`ALTER TABLE "tickets" ADD COLUMN "parent_branch" text NOT NULL DEFAULT ''`,
	`ALTER TABLE "projects" ADD COLUMN "write_scope" text NOT NULL DEFAULT ''`,
	`ALTER TABLE "tickets" ADD COLUMN "write_scope" text NOT NULL DEFAULT ''`,
}

func (d *DB) createSchema() error {
	return d.withTx(func(tx *sql.Tx) error {
		for _, stmt := range schemaStatements {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		for _, stmt := range migrations {
			if _, err := tx.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
				return err
			}
		}
		return nil
	})
}

// validateParentBranch checks that parentBranch is a safe git branch name.
// It rejects special git reference syntax that could cause unexpected behavior
// when passed to git rebase or git merge.
func validateParentBranch(parentBranch string) error {
	if parentBranch == "" {
		return nil
	}
	if strings.HasPrefix(parentBranch, "-") {
		return fmt.Errorf("invalid parent_branch %q: must not start with '-'", parentBranch)
	}
	for _, bad := range []string{"..", "@{", "~", "^", ":", "?", "*", "[", "\\", " "} {
		if strings.Contains(parentBranch, bad) {
			return fmt.Errorf("invalid parent_branch %q: must not contain %q", parentBranch, bad)
		}
	}
	return nil
}

// worktreePath returns the git worktree path for a ticket identifier.
func (d *DB) worktreePath(identifier string) string {
	return storage.TicketWorktreePathIn(d.ticketsDir, identifier)
}

// mergeTargetDir returns the directory to merge into when a work unit is
// completed. parentBranch is a git branch name (e.g. "main", "other-proj",
// "parent_child"). If it corresponds to a project worktree, we merge into
// that worktree's directory; otherwise we fall back to the repository root
// (which is where branches like "main" live).
//
// The branch→identifier conversion (replacing "_" with "/") reverses the
// identifier→branch conversion that CreateWorktree performs.
func (d *DB) mergeTargetDir(parentBranch string, projectID sql.NullInt64) string {
	if parentBranch != "" {
		identifier := strings.ReplaceAll(parentBranch, "_", "/")
		var exists int
		if err := d.db.QueryRow(`SELECT 1 FROM projects WHERE identifier = ?`, identifier).Scan(&exists); err == nil {
			return d.worktreePath(identifier)
		}
		return d.repoRoot
	}
	if projectID.Valid {
		var parentIdentifier string
		if err := d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&parentIdentifier); err == nil {
			return d.worktreePath(parentIdentifier)
		}
	}
	return d.repoRoot
}

// parseCodeLocation splits "file.go:42" into ("file.go", 42, nil).
func parseCodeLocation(loc string) (string, int, error) {
	idx := strings.LastIndex(loc, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf("invalid code location %q: missing ':'", loc)
	}
	line, err := strconv.Atoi(loc[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("invalid code location %q: %w", loc, err)
	}
	return loc[:idx], line, nil
}

// lookupProjectID returns the row id of the project with the given identifier.
func lookupProjectID(tx *sql.Tx, identifier string) (int64, error) {
	var id int64
	err := tx.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, identifier).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("project %q not found", identifier)
	}
	return id, err
}

// insertDependencies resolves each dependency identifier and inserts a row in
// the dependencies table for the given work unit.
func insertDependencies(tx *sql.Tx, wuType int, wuID int64, deps []string) error {
	for _, dep := range deps {
		depType, depID, err := resolveDependencyID(tx, dep)
		if err != nil {
			return fmt.Errorf("dependency %q: %w", dep, err)
		}
		if _, err := tx.Exec(
			`INSERT INTO dependencies (work_unit_type, work_unit_id, dependency_type, dependency_id) VALUES (?, ?, ?, ?)`,
			wuType, wuID, depType, depID,
		); err != nil {
			return fmt.Errorf("insert dependency %q: %w", dep, err)
		}
	}
	return nil
}

// resolveDependencyID finds the given identifier in either the tickets or
// projects table and returns its type code and row id.
func resolveDependencyID(tx *sql.Tx, identifier string) (int, int64, error) {
	var id int64
	if err := tx.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&id); err == nil {
		return workUnitTypeTicket, id, nil
	} else if err != sql.ErrNoRows {
		return 0, 0, err
	}
	if err := tx.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, identifier).Scan(&id); err == nil {
		return workUnitTypeProject, id, nil
	} else if err != sql.ErrNoRows {
		return 0, 0, err
	}
	return 0, 0, fmt.Errorf("work unit %q not found", identifier)
}

// allDependenciesDone reports whether every identifier in deps refers to a
// work unit whose phase is "done". A project's phase is set to "done" by
// MarkTicketDoneCascading once all of its children are complete, so this also
// covers the "project with all subprojects/tickets done" case.
func allDependenciesDone(tx *sql.Tx, deps []string) (bool, error) {
	for _, dep := range deps {
		var phase string
		err := tx.QueryRow(`SELECT phase FROM tickets WHERE identifier = ?`, dep).Scan(&phase)
		if err == sql.ErrNoRows {
			err = tx.QueryRow(`SELECT phase FROM projects WHERE identifier = ?`, dep).Scan(&phase)
		}
		if err != nil {
			return false, fmt.Errorf("look up dependency %q: %w", dep, err)
		}
		if phase != string(models.PhaseDone) {
			return false, nil
		}
	}
	return true, nil
}

// Status returns all work units (projects and tickets) with their dependencies
// and change requests populated.
func (d *DB) Status() ([]*models.WorkUnit, error) {
	projectByID, err := d.loadProjects()
	if err != nil {
		return nil, err
	}

	ticketByID, err := d.loadTickets(projectByID)
	if err != nil {
		return nil, err
	}

	if err := d.loadDependencies(projectByID, ticketByID); err != nil {
		return nil, err
	}

	if err := d.loadChangeRequests(ticketByID); err != nil {
		return nil, err
	}

	result := make([]*models.WorkUnit, 0, len(projectByID)+len(ticketByID))
	for _, wu := range projectByID {
		result = append(result, wu)
	}
	for _, wu := range ticketByID {
		result = append(result, wu)
	}
	return result, nil
}

func (d *DB) loadProjects() (map[int64]*models.WorkUnit, error) {
	projectByID := make(map[int64]*models.WorkUnit)
	rows, err := d.db.Query(`SELECT id, identifier, description, last_updated, phase, parent_branch FROM projects`)
	if err != nil {
		return nil, fmt.Errorf("load projects: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var identifier, description, phase, parentBranch string
		var lastUpdated int64
		if err := rows.Scan(&id, &identifier, &description, &lastUpdated, &phase, &parentBranch); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projectByID[id] = &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			IsProject:    true,
			LastUpdated:  time.Unix(lastUpdated, 0),
			Dependencies: []string{},
			ParentBranch: parentBranch,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan projects: %w", err)
	}

	for _, wu := range projectByID {
		if parent, ok := models.ParentIdentifierOf(wu.Identifier); ok {
			wu.Parent = parent
		}
	}
	return projectByID, nil
}

func (d *DB) loadTickets(projectByID map[int64]*models.WorkUnit) (map[int64]*models.WorkUnit, error) {
	ticketByID := make(map[int64]*models.WorkUnit)
	rows, err := d.db.Query(
		`SELECT id, identifier, description, phase, status, claimed_by, last_updated, project_id, parent_branch FROM tickets`,
	)
	if err != nil {
		return nil, fmt.Errorf("load tickets: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var identifier, description, phase, status, parentBranch string
		var claimedBy sql.NullInt64
		var lastUpdated int64
		var projectID sql.NullInt64
		if err := rows.Scan(&id, &identifier, &description, &phase, &status, &claimedBy, &lastUpdated, &projectID, &parentBranch); err != nil {
			return nil, fmt.Errorf("scan ticket: %w", err)
		}
		wu := &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			Status:       models.TicketStatus(status),
			IsProject:    false,
			LastUpdated:  time.Unix(lastUpdated, 0),
			Dependencies: []string{},
			ParentBranch: parentBranch,
		}
		if claimedBy.Valid {
			wu.ClaimedBy = strconv.FormatInt(claimedBy.Int64, 10)
		}
		if projectID.Valid {
			if proj, ok := projectByID[projectID.Int64]; ok {
				wu.Parent = proj.Identifier
			}
		}
		ticketByID[id] = wu
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan tickets: %w", err)
	}
	return ticketByID, nil
}

func (d *DB) loadDependencies(projectByID, ticketByID map[int64]*models.WorkUnit) error {
	rows, err := d.db.Query(
		`SELECT work_unit_type, work_unit_id, dependency_type, dependency_id FROM dependencies`,
	)
	if err != nil {
		return fmt.Errorf("load dependencies: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var wuType, wuID, depType, depID int64
		if err := rows.Scan(&wuType, &wuID, &depType, &depID); err != nil {
			return fmt.Errorf("scan dependency: %w", err)
		}
		var depIdentifier string
		switch depType {
		case workUnitTypeTicket:
			if t, ok := ticketByID[depID]; ok {
				depIdentifier = t.Identifier
			}
		case workUnitTypeProject:
			if p, ok := projectByID[depID]; ok {
				depIdentifier = p.Identifier
			}
		}
		if depIdentifier == "" {
			continue
		}
		var wu *models.WorkUnit
		switch wuType {
		case workUnitTypeTicket:
			wu = ticketByID[wuID]
		case workUnitTypeProject:
			wu = projectByID[wuID]
		}
		if wu != nil {
			wu.Dependencies = append(wu.Dependencies, depIdentifier)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("scan dependencies: %w", err)
	}
	return nil
}

func (d *DB) loadChangeRequests(ticketByID map[int64]*models.WorkUnit) error {
	rows, err := d.db.Query(
		`SELECT id, ticket_id, filename, line_number, commit_hash, status, date, author, description
		 FROM change_requests ORDER BY date ASC`,
	)
	if err != nil {
		return fmt.Errorf("load change requests: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, ticketID int64
		var filename, commitHash, cstatus, author, description string
		var lineNumber int
		var date int64
		if err := rows.Scan(&id, &ticketID, &filename, &lineNumber, &commitHash, &cstatus, &date, &author, &description); err != nil {
			return fmt.Errorf("scan change request: %w", err)
		}
		wu, ok := ticketByID[ticketID]
		if !ok {
			continue
		}
		wu.ChangeRequests = append(wu.ChangeRequests, models.ChangeRequest{
			ID:           strconv.FormatInt(id, 10),
			CommitHash:   commitHash,
			CodeLocation: fmt.Sprintf("%s:%d", filename, lineNumber),
			Status:       cstatus,
			Date:         time.Unix(date, 0),
			Author:       author,
			Description:  description,
		})
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("scan change requests: %w", err)
	}
	return nil
}

// CreateProject inserts a new project (and its dependencies) in a single
// transaction and creates its directory under .code-factory/.
// parentBranch is a git branch name (e.g. "main", "release-v2") that, if
// non-empty, overrides the default branch this project merges into when
// completed. The default is the parent project's worktree branch, or the
// repository's default branch for top-level projects.
func (d *DB) CreateProject(identifier, description string, deps []string, parentBranch string, writeScope []string) error {
	if err := models.ValidateIdentifier(identifier); err != nil {
		return err
	}
	if err := validateParentBranch(parentBranch); err != nil {
		return err
	}
	scopeStr := encodeScope(writeScope)
	if err := d.withTx(func(tx *sql.Tx) error {
		var parentID sql.NullInt64
		if parent, hasParent := models.ParentIdentifierOf(identifier); hasParent {
			pid, err := lookupProjectID(tx, parent)
			if err != nil {
				return err
			}
			parentID = sql.NullInt64{Int64: pid, Valid: true}
		}
		res, err := tx.Exec(
			`INSERT INTO projects (identifier, description, last_updated, project_id, parent_branch, write_scope) VALUES (?, ?, ?, ?, ?, ?)`,
			identifier, description, time.Now().Unix(), parentID, parentBranch, scopeStr,
		)
		if err != nil {
			return fmt.Errorf("insert project: %w", err)
		}
		projectID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		return insertDependencies(tx, workUnitTypeProject, projectID, deps)
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(storage.TicketDirPath(d.ticketsDir, identifier), 0755); err != nil {
		return err
	}
	return d.git.CreateWorktree(d.repoRoot, d.worktreePath(identifier), identifier)
}

// CreateTicket inserts a new ticket (and its dependencies) in a single
// transaction, then creates a git worktree for it immediately.
// parentBranch is a git branch name (e.g. "main", "release-v2") that, if
// non-empty, overrides the default branch this ticket merges into when
// completed. The default is the parent project's worktree branch, or the
// repository's default branch for top-level tickets.
func (d *DB) CreateTicket(identifier, description string, deps []string, parentBranch string, writeScope []string) error {
	if err := models.ValidateIdentifier(identifier); err != nil {
		return err
	}
	if err := validateParentBranch(parentBranch); err != nil {
		return err
	}
	scopeStr := encodeScope(writeScope)
	if err := d.withTx(func(tx *sql.Tx) error {
		phase := models.PhaseImplement
		if len(deps) > 0 {
			allDone, err := allDependenciesDone(tx, deps)
			if err != nil {
				return err
			}
			if !allDone {
				phase = models.PhaseBlocked
			}
		}

		var projectID sql.NullInt64
		if parent, hasParent := models.ParentIdentifierOf(identifier); hasParent {
			pid, err := lookupProjectID(tx, parent)
			if err != nil {
				return err
			}
			projectID = sql.NullInt64{Int64: pid, Valid: true}
		}

		res, err := tx.Exec(
			`INSERT INTO tickets (identifier, description, phase, status, last_updated, project_id, parent_branch, write_scope) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			identifier, description, string(phase), string(models.StatusIdle), time.Now().Unix(), projectID, parentBranch, scopeStr,
		)
		if err != nil {
			return fmt.Errorf("insert ticket: %w", err)
		}
		ticketID, err := res.LastInsertId()
		if err != nil {
			return err
		}
		return insertDependencies(tx, workUnitTypeTicket, ticketID, deps)
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(storage.TicketDirPath(d.ticketsDir, identifier), 0755); err != nil {
		return err
	}
	return d.git.CreateWorktree(d.repoRoot, d.worktreePath(identifier), identifier)
}

// SetStatus updates the phase (and optionally status) of a ticket. When phase
// is "done", the ticket's branch is rebased onto its parent and fast-forwarded
// in, then the worktree is removed.
func (d *DB) SetStatus(identifier string, phase models.TicketPhase, status models.TicketStatus) error {
	if !models.IsValidTicketPhase(string(phase)) {
		return fmt.Errorf("invalid ticket phase %q", phase)
	}
	if !models.IsValidTicketStatus(string(status)) {
		return fmt.Errorf("invalid ticket status %q", status)
	}

	var ticketID int64
	var projectID sql.NullInt64
	var parentBranch string
	err := d.db.QueryRow(
		`SELECT id, project_id, parent_branch FROM tickets WHERE identifier = ?`, identifier,
	).Scan(&ticketID, &projectID, &parentBranch)
	if err == sql.ErrNoRows {
		var projID int64
		if err2 := d.db.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, identifier).Scan(&projID); err2 == nil {
			return fmt.Errorf("%q is a project, not a ticket", identifier)
		}
		return fmt.Errorf("ticket %q not found", identifier)
	}
	if err != nil {
		return err
	}

	if phase == models.PhaseDone {
		return d.markTicketDone(ticketID, identifier, projectID, parentBranch)
	}

	return d.withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`UPDATE tickets SET phase = ?, status = ? WHERE id = ?`,
			string(phase), string(status), ticketID,
		)
		return err
	})
}

// RebaseTicketOnParent rebases the ticket's worktree branch onto the current
// HEAD of its parent's branch. parentBranch is a git branch name that, if
// non-empty, overrides the default target. For tickets under a project, the
// default is the project's worktree branch; for top-level tickets, it is
// whatever branch is checked out at the repo root (typically main).
func (d *DB) RebaseTicketOnParent(identifier, parentIdentifier, parentBranch string) error {
	worktreePath := d.worktreePath(identifier)

	var ontoBranch string
	if parentBranch != "" {
		ontoBranch = parentBranch
	} else if parentIdentifier != "" {
		ontoBranch = strings.ReplaceAll(parentIdentifier, "/", "_")
	} else {
		branch, err := d.git.GetCurrentBranch(d.repoRoot)
		if err != nil {
			return fmt.Errorf("rebase: detect default branch: %w", err)
		}
		ontoBranch = branch
	}

	if err := d.git.RebaseOnto(worktreePath, ontoBranch); err != nil {
		_ = d.git.AbortRebase(worktreePath)
		return err
	}
	return nil
}

// rebaseAndFastForward combines a work unit's branch into its parent using a
// rebase strategy: rebase the child branch onto the parent's current HEAD,
// then fast-forward the parent to the rebased tip. The result is linear
// history with no merge commit. If the rebase hits conflicts, it is left in
// progress in the child's worktree so the user can resolve them manually,
// and the returned MergeConflictError points at that worktree.
//
// Ticket branches are squashed to a single commit before rebasing so the
// parent sees one diff per ticket instead of the full implement/refactor/
// respond commit fan. Project branches keep their full child commit history.
func (d *DB) rebaseAndFastForward(identifier, mergeTarget string) error {
	childWorktree := d.worktreePath(identifier)

	targetBranch, err := d.git.GetCurrentBranch(mergeTarget)
	if err != nil {
		return fmt.Errorf("detect target branch for %s: %w", mergeTarget, err)
	}

	unitType, _, err := d.classifyIdentifier(identifier)
	if err != nil {
		return fmt.Errorf("classify %s: %w", identifier, err)
	}
	if unitType == workUnitTypeTicket {
		markers, err := d.git.FindForbiddenMarkers(childWorktree, targetBranch)
		if err != nil {
			return fmt.Errorf("scan markers on %s: %w", identifier, err)
		}
		if len(markers) > 0 {
			return &ForbiddenMarkersError{
				Identifier:   identifier,
				WorktreePath: childWorktree,
				Markers:      markers,
			}
		}
		if err := d.git.SquashSinceMergeBase(childWorktree, targetBranch, identifier); err != nil {
			return fmt.Errorf("squash %s: %w", identifier, err)
		}
	}

	// Enable rerere so that conflict resolutions are recorded in the
	// shared rr-cache. When a sibling or parent rebase encounters the
	// same conflict, git replays the recorded resolution automatically.
	// Since linked worktrees share the parent repo's git config and
	// rr-cache, enabling rerere on any worktree enables it for all
	// worktrees in the same repository. Errors are non-fatal: rerere is
	// an optimisation, not a correctness requirement.
	_ = d.git.EnableRerere(childWorktree)

	if err := d.git.RebaseOnto(childWorktree, targetBranch); err != nil {
		return &MergeConflictError{WorktreePath: childWorktree, Branch: identifier, Err: err}
	}

	if err := d.git.MergeBranch(mergeTarget, identifier); err != nil {
		return &MergeConflictError{WorktreePath: mergeTarget, Branch: identifier, Err: err}
	}
	return nil
}

// mergeLock returns the mutex that serializes merges into the given target
// directory. Two tickets merging into the same parent will block on the same
// lock, preventing interleaved git operations.
func (d *DB) mergeLock(target string) *sync.Mutex {
	d.mergeMu.Lock()
	defer d.mergeMu.Unlock()
	if d.mergeLocks == nil {
		d.mergeLocks = make(map[string]*sync.Mutex)
	}
	mu, ok := d.mergeLocks[target]
	if !ok {
		mu = &sync.Mutex{}
		d.mergeLocks[target] = mu
	}
	return mu
}

// markTicketDone rebases the ticket's branch onto the parent project's
// worktree branch (or repoRoot's branch if there is no parent),
// fast-forwards the parent onto the rebased tip, updates the DB, and
// removes the worktree. If parentBranch is non-empty, it overrides the
// default target.
//
// This entry point handles the ticket only; it does not cascade parent
// projects. Workflow callers should use MarkTicketDoneCascading instead.
func (d *DB) markTicketDone(ticketID int64, identifier string, projectID sql.NullInt64, parentBranch string) error {
	mergeTarget := d.mergeTargetDir(parentBranch, projectID)

	mu := d.mergeLock(mergeTarget)
	mu.Lock()
	defer mu.Unlock()

	if err := d.rebaseAndFastForward(identifier, mergeTarget); err != nil {
		return err
	}

	if err := d.withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`UPDATE tickets SET phase = ?, status = ?, claimed_by = NULL WHERE id = ?`,
			string(models.PhaseDone), string(models.StatusIdle), ticketID,
		)
		return err
	}); err != nil {
		return fmt.Errorf("update ticket: %w", err)
	}

	if err := d.unblockDependents(workUnitTypeTicket, ticketID); err != nil {
		return fmt.Errorf("unblock dependents: %w", err)
	}

	if err := d.git.RemoveWorktree(d.repoRoot, d.worktreePath(identifier), identifier); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// completionStep represents one work unit (ticket or project) that will be
// rebased and marked done as part of a cascading completion.
type completionStep struct {
	Identifier  string
	IsProject   bool
	WorkUnitID  int64
	MergeTarget string
}

// candidate represents a claimable ticket during the claim selection process.
type candidate struct {
	id         int64
	identifier string
	projectID  sql.NullInt64
	scope      []string
}

// MarkTicketDoneCascading transitions a ticket to "done" and cascades parent
// projects to "done" when all of their direct children are complete. All git
// rebases up the tree are performed before any database updates: if any rebase
// fails (e.g. a merge conflict at the grandparent level), no work unit is
// marked done and the user can resolve the conflict and retry. The git
// rebases themselves are idempotent, so a retry that re-walks levels which
// previously succeeded is safe.
func (d *DB) MarkTicketDoneCascading(identifier string) error {
	return d.MergeChain(context.Background(), identifier, nil)
}

// mergeChainMaxConflictAttempts caps how many times MergeChain will hand a
// step back to onConflict before giving up. A pass can legitimately surface
// a fresh conflict in a different worktree (e.g. resolving a fast-forward
// conflict in a parent worktree advances the parent's tip and exposes new
// content conflicts when the child rebase is retried), so a single pass is
// not enough.
const mergeChainMaxConflictAttempts = 3

// MergeChain runs the cascading rebase that turns a ticket and its
// fully-completed parent projects into "done". On a rebase/fast-forward
// conflict, onConflict is invoked with the path of the worktree holding
// the in-progress state (this may be the ticket's own worktree or a
// parent's). After onConflict returns, the worktree is rechecked; if it
// is clean the cascade resumes from the same step (the rebase is
// idempotent, so a no-op retry is fine if the agent already ran
// `git rebase --continue` to completion). The retry can itself produce a
// fresh conflict — possibly in a different worktree — so onConflict is
// re-invoked with the new conflict, up to mergeChainMaxConflictAttempts
// times. If the worktree is still dirty after onConflict returns, or the
// attempt budget is exhausted, MergeUnresolvedError is returned (carrying
// the worktree of the unresolved conflict, not the original) and no work
// unit is finalized.
//
// onConflict may be nil, in which case any conflict surfaces directly as
// the underlying *MergeConflictError (matching MarkTicketDoneCascading's
// pre-merging-phase behaviour).
//
// On success every work unit in the chain is finalized: marked done in
// the DB, dependents unblocked, and the ticket worktree removed.
func (d *DB) MergeChain(ctx context.Context, identifier string, onConflict func(stepIdentifier, worktreePath string) error) error {
	chain, err := d.computeCompletionChain(identifier)
	if err != nil {
		return err
	}

	// Acquire merge locks for every distinct merge target in the chain.
	// The chain is bottom-up (ticket → root), giving a consistent lock
	// ordering across concurrent callers, so we cannot deadlock.
	seen := make(map[string]bool)
	var held []*sync.Mutex
	for _, step := range chain {
		if seen[step.MergeTarget] {
			continue
		}
		seen[step.MergeTarget] = true
		mu := d.mergeLock(step.MergeTarget)
		mu.Lock()
		held = append(held, mu)
	}
	defer func() {
		for i := len(held) - 1; i >= 0; i-- {
			held[i].Unlock()
		}
	}()

	// Phase 1: every rebase + fast-forward in the chain must succeed.
	// rebaseAndFastForward squashes ticket branches and leaves project
	// branches with their full commit history; see its doc comment.
	for _, step := range chain {
		err := d.rebaseAndFastForward(step.Identifier, step.MergeTarget)
		if err == nil {
			continue
		}
		if onConflict == nil {
			return err
		}

		// Hand the conflict to the agent, then retry rebaseAndFastForward.
		// A retry can surface a *new* conflict (different worktree, fresh
		// content), so re-engage the agent on whatever conflict the retry
		// produces, bounded by mergeChainMaxConflictAttempts.
		for attempt := 0; ; attempt++ {
			var conflictErr *MergeConflictError
			if !errors.As(err, &conflictErr) {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt >= mergeChainMaxConflictAttempts {
				return &MergeUnresolvedError{Identifier: step.Identifier, WorktreePath: conflictErr.WorktreePath}
			}
			if cbErr := onConflict(step.Identifier, conflictErr.WorktreePath); cbErr != nil {
				return cbErr
			}
			clean, cleanErr := git.IsWorktreeClean(conflictErr.WorktreePath)
			if cleanErr != nil {
				return cleanErr
			}
			if !clean {
				return &MergeUnresolvedError{Identifier: step.Identifier, WorktreePath: conflictErr.WorktreePath}
			}
			// rebaseAndFastForward is idempotent: a no-op rebase plus a
			// no-op fast-forward when the target is already at the tip.
			// Squash is also idempotent (no-op once the branch has ≤ 1
			// commit since the merge base).
			err = d.rebaseAndFastForward(step.Identifier, step.MergeTarget)
			if err == nil {
				break
			}
		}
	}

	// Phase 2: only now apply database updates and remove the ticket worktree.
	return d.finalizeCompletionChain(chain)
}

// computeCompletionChain returns the ordered list of work units that will
// transition to "done" when the given ticket is completed: the ticket itself,
// followed by any parent projects (walking up the tree) whose remaining
// children would all be done after the earlier steps in the chain finish.
// The walk stops at the first parent that is already done or that would
// still have un-done children.
func (d *DB) computeCompletionChain(ticketIdentifier string) ([]completionStep, error) {
	var ticketID int64
	var ticketProjectID sql.NullInt64
	var ticketParentBranch string
	err := d.db.QueryRow(
		`SELECT id, project_id, parent_branch FROM tickets WHERE identifier = ?`,
		ticketIdentifier,
	).Scan(&ticketID, &ticketProjectID, &ticketParentBranch)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket %q not found", ticketIdentifier)
	}
	if err != nil {
		return nil, err
	}

	chain := []completionStep{{
		Identifier:  ticketIdentifier,
		IsProject:   false,
		WorkUnitID:  ticketID,
		MergeTarget: d.mergeTargetDir(ticketParentBranch, ticketProjectID),
	}}

	pending := map[string]bool{ticketIdentifier: true}
	cur := ticketIdentifier
	for {
		parentID, hasParent := models.ParentIdentifierOf(cur)
		if !hasParent {
			break
		}

		var parentDBID int64
		var grandparentDBID sql.NullInt64
		var grandparentBranch, parentPhase string
		err := d.db.QueryRow(
			`SELECT id, project_id, parent_branch, phase FROM projects WHERE identifier = ?`,
			parentID,
		).Scan(&parentDBID, &grandparentDBID, &grandparentBranch, &parentPhase)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			return nil, err
		}

		// If this parent is already done, no extra cascading work is needed.
		if parentPhase == string(models.ProjectPhaseDone) {
			break
		}

		wouldBeDone, err := d.wouldProjectBeFullyDone(parentDBID, pending)
		if err != nil {
			return nil, err
		}
		if !wouldBeDone {
			break
		}

		chain = append(chain, completionStep{
			Identifier:  parentID,
			IsProject:   true,
			WorkUnitID:  parentDBID,
			MergeTarget: d.mergeTargetDir(grandparentBranch, grandparentDBID),
		})
		pending[parentID] = true
		cur = parentID
	}

	return chain, nil
}

// wouldProjectBeFullyDone reports whether the project with the given row id
// would have all of its direct children done if the identifiers in the
// pending set were treated as already done. The project must have at least
// one child to be considered done.
func (d *DB) wouldProjectBeFullyDone(projectID int64, pending map[string]bool) (bool, error) {
	rows, err := d.db.Query(`
		SELECT identifier, phase FROM tickets WHERE project_id = ?
		UNION ALL
		SELECT identifier, phase FROM projects WHERE project_id = ?
	`, projectID, projectID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var identifier, phase string
		if err := rows.Scan(&identifier, &phase); err != nil {
			return false, err
		}
		total++
		if phase == "done" {
			continue
		}
		if pending[identifier] {
			continue
		}
		return false, nil
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return total > 0, nil
}

// finalizeCompletionChain applies the database updates for every work unit
// in the chain (ticket + parent projects), unblocks dependents, and removes
// the ticket's worktree. Called only after every rebase in the chain has
// succeeded.
func (d *DB) finalizeCompletionChain(chain []completionStep) error {
	if len(chain) == 0 {
		return nil
	}

	now := time.Now().Unix()
	if err := d.withTx(func(tx *sql.Tx) error {
		for _, step := range chain {
			if step.IsProject {
				if _, err := tx.Exec(
					`UPDATE projects SET phase = ?, last_updated = ? WHERE id = ?`,
					string(models.ProjectPhaseDone), now, step.WorkUnitID,
				); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec(
					`UPDATE tickets SET phase = ?, status = ?, claimed_by = NULL WHERE id = ?`,
					string(models.PhaseDone), string(models.StatusIdle), step.WorkUnitID,
				); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("finalize completion chain: %w", err)
	}

	for _, step := range chain {
		unitType := workUnitTypeTicket
		if step.IsProject {
			unitType = workUnitTypeProject
		}
		if err := d.unblockDependents(unitType, step.WorkUnitID); err != nil {
			return fmt.Errorf("unblock dependents for %s: %w", step.Identifier, err)
		}
	}

	ticketStep := chain[0]
	if err := d.git.RemoveWorktree(d.repoRoot, d.worktreePath(ticketStep.Identifier), ticketStep.Identifier); err != nil {
		return fmt.Errorf("remove worktree: %w", err)
	}
	return nil
}

// unblockDependents finds all blocked tickets that depended on the just-completed
// work unit (identified by type and id) and transitions any whose dependencies
// are now ALL done from "blocked" to "implement/idle".
func (d *DB) unblockDependents(completedType int, completedID int64) error {
	// Find blocked tickets that list the completed work unit as a dependency.
	rows, err := d.db.Query(`
		SELECT DISTINCT d.work_unit_id
		FROM dependencies d
		JOIN tickets t ON t.id = d.work_unit_id
		WHERE d.work_unit_type = 1
		  AND d.dependency_type = ?
		  AND d.dependency_id = ?
		  AND t.phase = 'blocked'
	`, completedType, completedID)
	if err != nil {
		return fmt.Errorf("unblockDependents: query dependents: %w", err)
	}
	defer rows.Close()

	var candidateIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("unblockDependents: scan: %w", err)
		}
		candidateIDs = append(candidateIDs, id)
	}

	for _, ticketID := range candidateIDs {
		// Check whether ALL dependencies of this ticket are now done.
		var pending int
		err := d.db.QueryRow(`
			SELECT COUNT(*) FROM dependencies d
			WHERE d.work_unit_type = 1 AND d.work_unit_id = ?
			  AND NOT (
			    (d.dependency_type = 1 AND EXISTS (SELECT 1 FROM tickets WHERE id = d.dependency_id AND phase = 'done'))
			    OR
			    (d.dependency_type = 2 AND EXISTS (SELECT 1 FROM projects WHERE id = d.dependency_id AND phase = 'done'))
			  )
		`, ticketID).Scan(&pending)
		if err != nil {
			return fmt.Errorf("unblockDependents: check pending for ticket %d: %w", ticketID, err)
		}
		if pending == 0 {
			if _, err := d.db.Exec(
				`UPDATE tickets SET phase = ?, status = ? WHERE id = ?`,
				string(models.PhaseImplement), string(models.StatusIdle), ticketID,
			); err != nil {
				return fmt.Errorf("unblockDependents: unblock ticket %d: %w", ticketID, err)
			}
			d.notifyWorkAvailable()
		}
	}
	return nil
}

// Claim assigns the first claimable ticket to the given PID and returns it.
func (d *DB) Claim(pid int) (*models.WorkUnit, error) {
	var result *models.WorkUnit
	err := d.withTx(func(tx *sql.Tx) error {
		id, wu, err := claimTicketRow(tx, pid)
		if err != nil {
			return err
		}
		if err := loadTicketDeps(tx, id, wu); err != nil {
			return err
		}
		result = wu
		return nil
	})
	return result, err
}

// claimTicketRow selects the first claimable ticket for pid and claims it.
//
// Candidates are selected with ORDER BY id for deterministic ordering, then
// filtered in Go against a pre-fetched sibling scope map. The chosen
// candidate is claimed with an UPDATE that includes "AND claimed_by IS NULL"
// so that a concurrent transaction that claimed the same row first causes
// RowsAffected == 0 instead of silently overwriting. When that happens the
// loop advances to the next candidate.
func claimTicketRow(tx *sql.Tx, pid int) (int64, *models.WorkUnit, error) {
	now := time.Now().Unix()

	// Find all candidate tickets (not blocked/done, idle/responding, unclaimed),
	// ordered by id for deterministic selection.
	rows, err := tx.Query(`
		SELECT id, identifier, project_id, write_scope
		FROM tickets
		WHERE phase NOT IN ('blocked', 'done')
		  AND status IN ('idle', 'responding')
		  AND claimed_by IS NULL
		ORDER BY id
	`)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var c candidate
		var scopeStr string
		if err := rows.Scan(&c.id, &c.identifier, &c.projectID, &scopeStr); err != nil {
			return 0, nil, err
		}
		c.scope = decodeScope(scopeStr)
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return 0, nil, err
	}

	if len(candidates) == 0 {
		return 0, nil, fmt.Errorf("no claimable ticket available")
	}

	// Fetch all claimed siblings' scopes in a single query, grouped by
	// project_id, so the per-candidate overlap check is done in memory.
	claimedScopes, err := loadClaimedSiblingScopes(tx)
	if err != nil {
		return 0, nil, err
	}

	// For each candidate, check sibling exclusion: if the candidate has a
	// write_scope AND shares a project_id with a currently-claimed sibling
	// whose write_scope overlaps, skip it. If the UPDATE hits 0 rows
	// (concurrent claim), try the next candidate.
	for i := range candidates {
		c := &candidates[i]
		if isScopeBlockedBySiblingMap(c, claimedScopes) {
			continue
		}

		// Claim the chosen ticket. The "AND claimed_by IS NULL" guard
		// prevents overwriting a concurrent claim.
		res, err := tx.Exec(`
			UPDATE tickets SET claimed_by = ?, last_updated = ?
			WHERE id = ? AND claimed_by IS NULL
		`, pid, now, c.id)
		if err != nil {
			return 0, nil, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return 0, nil, err
		}
		if n == 0 {
			// Another transaction claimed this ticket; try next candidate.
			continue
		}

		// Read back the ticket we just claimed.
		var id int64
		var identifier, description, phase, status, parentBranch string
		var projectID sql.NullInt64
		err = tx.QueryRow(`
			SELECT id, identifier, description, phase, status, project_id, parent_branch
			FROM tickets
			WHERE id = ?
		`, c.id).Scan(&id, &identifier, &description, &phase, &status, &projectID, &parentBranch)
		if err != nil {
			return 0, nil, err
		}

		parent, err := projectIdentifierByID(tx, projectID)
		if err != nil {
			return 0, nil, err
		}

		wu := &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			Status:       models.TicketStatus(status),
			ClaimedBy:    strconv.Itoa(pid),
			LastUpdated:  time.Unix(now, 0),
			IsProject:    false,
			Dependencies: []string{},
			Parent:       parent,
			ParentBranch: parentBranch,
			WriteScope:   c.scope,
		}
		return id, wu, nil
	}

	return 0, nil, fmt.Errorf("no claimable ticket available")
}

// projectIdentifierByID resolves an optional project row ID to its identifier string.
func projectIdentifierByID(tx *sql.Tx, projectID sql.NullInt64) (string, error) {
	if !projectID.Valid {
		return "", nil
	}
	var identifier string
	err := tx.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&identifier)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return identifier, err
}

// loadTicketDeps appends dependency identifiers to wu.Dependencies.
func loadTicketDeps(tx *sql.Tx, ticketID int64, wu *models.WorkUnit) error {
	rows, err := tx.Query(`
		SELECT dependency_type, dependency_id FROM dependencies
		WHERE work_unit_type = ? AND work_unit_id = ?
	`, workUnitTypeTicket, ticketID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var depType int
		var depID int64
		if err := rows.Scan(&depType, &depID); err != nil {
			return err
		}
		var depIdentifier string
		var scanErr error
		switch depType {
		case workUnitTypeTicket:
			scanErr = tx.QueryRow(`SELECT identifier FROM tickets WHERE id = ?`, depID).Scan(&depIdentifier)
		case workUnitTypeProject:
			scanErr = tx.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, depID).Scan(&depIdentifier)
		}
		if scanErr != nil && scanErr != sql.ErrNoRows {
			return scanErr
		}
		if depIdentifier != "" {
			wu.Dependencies = append(wu.Dependencies, depIdentifier)
		}
	}
	return rows.Err()
}

// Release clears the claim on the given ticket.
func (d *DB) Release(identifier string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE tickets SET claimed_by = NULL, last_updated = ? WHERE identifier = ?`,
			time.Now().Unix(), identifier,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("ticket %q not found", identifier)
		}
		return nil
	})
}

// ResetTicket atomically clears the claim so the ticket is reclaimable.
// Working/needs-attention tickets revert to idle; responding tickets stay
// responding so the pending /cf-respond run is retried instead of falling
// back to the prior phase. Used by housekeeping for stale tickets where the
// worker is presumed dead.
func (d *DB) ResetTicket(identifier string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE tickets
			 SET status = CASE WHEN status = 'responding' THEN 'responding' ELSE 'idle' END,
			     claimed_by = NULL,
			     last_updated = ?
			 WHERE identifier = ?`,
			time.Now().Unix(), identifier,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("ticket %q not found", identifier)
		}
		return nil
	})
}

// RecoverOrphanedTickets resets all tickets stuck in a running state
// (working, responding, or needs-attention) so they become re-claimable.
// Working and needs-attention tickets revert to idle; responding tickets
// stay in responding so the pending /cf-respond run is retried. Called at
// startup to recover from hard kills where workers died without cleaning
// up. Returns the number of tickets recovered.
func (d *DB) RecoverOrphanedTickets() (int, error) {
	var count int
	err := d.withTx(func(tx *sql.Tx) error {
		now := time.Now().Unix()
		res, err := tx.Exec(
			`UPDATE tickets SET status = 'idle', claimed_by = NULL, last_updated = ?
			 WHERE status IN ('working', 'needs-attention')`,
			now,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		res, err = tx.Exec(
			`UPDATE tickets SET claimed_by = NULL, last_updated = ?
			 WHERE status = 'responding'`,
			now,
		)
		if err != nil {
			return err
		}
		m, err := res.RowsAffected()
		if err != nil {
			return err
		}
		count = int(n + m)
		return nil
	})
	return count, err
}

// AddChangeRequest adds a new change request for the given ticket at the
// specified code location.
func (d *DB) AddChangeRequest(identifier, codeLocation, author, description string) error {
	filename, lineNumber, err := parseCodeLocation(codeLocation)
	if err != nil {
		return err
	}

	return d.withTx(func(tx *sql.Tx) error {
		var ticketID int64
		if err := tx.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&ticketID); err == sql.ErrNoRows {
			return fmt.Errorf("ticket %q not found", identifier)
		} else if err != nil {
			return err
		}

		commitHash, _ := d.git.GetHeadCommit(d.worktreePath(identifier))

		_, err = tx.Exec(
			`INSERT INTO change_requests (ticket_id, filename, line_number, commit_hash, status, date, author, description)
			 VALUES (?, ?, ?, ?, 'open', ?, ?, ?)`,
			ticketID, filename, lineNumber, commitHash,
			time.Now().Unix(), author, description,
		)
		return err
	})
}

// ChangeRequestInput holds the fields for a single change request in a batch.
type ChangeRequestInput struct {
	CodeLocation string `json:"code_location"`
	Description  string `json:"description"`
}

// BatchAddChangeRequests inserts multiple change requests for a ticket in a
// single transaction. This is more efficient than calling AddChangeRequest
// in a loop because it resolves the ticket ID and commit hash once.
func (d *DB) BatchAddChangeRequests(identifier, author string, crs []ChangeRequestInput) error {
	if len(crs) == 0 {
		return nil
	}

	// Parse all code locations up front so we fail fast on bad input.
	type parsed struct {
		filename    string
		lineNumber  int
		description string
	}
	items := make([]parsed, len(crs))
	for i, cr := range crs {
		f, ln, err := parseCodeLocation(cr.CodeLocation)
		if err != nil {
			return fmt.Errorf("change request %d: %w", i, err)
		}
		items[i] = parsed{filename: f, lineNumber: ln, description: cr.Description}
	}

	return d.withTx(func(tx *sql.Tx) error {
		var ticketID int64
		if err := tx.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&ticketID); err == sql.ErrNoRows {
			return fmt.Errorf("ticket %q not found", identifier)
		} else if err != nil {
			return err
		}

		commitHash, _ := d.git.GetHeadCommit(d.worktreePath(identifier))
		now := time.Now().Unix()

		for _, item := range items {
			if _, err := tx.Exec(
				`INSERT INTO change_requests (ticket_id, filename, line_number, commit_hash, status, date, author, description)
				 VALUES (?, ?, ?, ?, 'open', ?, ?, ?)`,
				ticketID, item.filename, item.lineNumber, commitHash,
				now, author, item.description,
			); err != nil {
				return err
			}
		}
		return nil
	})
}

// setChangeRequestStatus updates the status of a single change request by id.
func (d *DB) setChangeRequestStatus(id int64, status string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE change_requests SET status = ? WHERE id = ?`, status, id,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("change request %d not found", id)
		}
		return nil
	})
}

// CloseChangeRequest sets the status of the change request with the given id to "closed".
func (d *DB) CloseChangeRequest(id int64) error {
	return d.setChangeRequestStatus(id, models.ChangeRequestClosed)
}

// DismissChangeRequest sets the status of the change request with the given id to "dismissed".
func (d *DB) DismissChangeRequest(id int64) error {
	return d.setChangeRequestStatus(id, models.ChangeRequestDismissed)
}

// ReopenChangeRequest sets the status of the change request with the given id to "open".
func (d *DB) ReopenChangeRequest(id int64) error {
	return d.setChangeRequestStatus(id, models.ChangeRequestOpen)
}

// DeleteChangeRequestsForTicket deletes all change requests associated with
// the given ticket identifier.
func (d *DB) DeleteChangeRequestsForTicket(identifier string) error {
	return d.withTx(func(tx *sql.Tx) error {
		var ticketID int64
		err := tx.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&ticketID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("ticket %q not found", identifier)
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(`DELETE FROM change_requests WHERE ticket_id = ?`, ticketID)
		return err
	})
}

// GetWriteScope returns the write_scope for a work unit (ticket or project).
// If the identifier is not found, it returns an empty slice and an error.
func (d *DB) GetWriteScope(identifier string) ([]string, error) {
	var scopeStr string
	err := d.db.QueryRow(`SELECT write_scope FROM tickets WHERE identifier = ?`, identifier).Scan(&scopeStr)
	if err == sql.ErrNoRows {
		err = d.db.QueryRow(`SELECT write_scope FROM projects WHERE identifier = ?`, identifier).Scan(&scopeStr)
	}
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("work unit %q not found", identifier)
	}
	if err != nil {
		return nil, err
	}
	return decodeScope(scopeStr), nil
}

// SetWriteScope updates the write_scope for a work unit (ticket or project).
// It tries the tickets table first, then projects, matching the lookup order
// of GetWriteScope.
func (d *DB) SetWriteScope(identifier string, scope []string) error {
	scopeStr := encodeScope(scope)
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`UPDATE tickets SET write_scope = ? WHERE identifier = ?`, scopeStr, identifier)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		res, err = tx.Exec(`UPDATE projects SET write_scope = ? WHERE identifier = ?`, scopeStr, identifier)
		if err != nil {
			return err
		}
		n, err = res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("work unit %q not found", identifier)
		}
		return nil
	})
}

// encodeScope serializes a scope slice into the comma-delimited string stored
// in the database. An empty/nil slice produces "".
func encodeScope(scope []string) string {
	return strings.Join(scope, ",")
}

// decodeScope deserializes the comma-delimited scope string from the database
// into a slice. An empty string produces nil.
func decodeScope(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// scopesOverlap reports whether two scope lists have any overlapping paths.
// Overlap is defined by prefix matching: "internal/db/" overlaps with
// "internal/db/schema.go" and vice versa.
func scopesOverlap(a, b []string) bool {
	for _, ap := range a {
		for _, bp := range b {
			if strings.HasPrefix(ap, bp) || strings.HasPrefix(bp, ap) {
				return true
			}
		}
	}
	return false
}

// loadClaimedSiblingScopes fetches the write_scope of every currently-claimed
// ticket that has a non-empty scope, grouped by project_id. The result is a
// map from project_id to a slice of decoded scopes (one []string per sibling).
// This allows the caller to check scope overlap for each candidate in memory
// instead of issuing a SQL query per candidate.
func loadClaimedSiblingScopes(tx *sql.Tx) (map[int64][][]string, error) {
	rows, err := tx.Query(`
		SELECT project_id, write_scope FROM tickets
		WHERE claimed_by IS NOT NULL
		  AND write_scope != ''
		  AND phase NOT IN ('done')
		  AND project_id IS NOT NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64][][]string)
	for rows.Next() {
		var projectID int64
		var scopeStr string
		if err := rows.Scan(&projectID, &scopeStr); err != nil {
			return nil, err
		}
		result[projectID] = append(result[projectID], decodeScope(scopeStr))
	}
	return result, rows.Err()
}

// isScopeBlockedBySiblingMap reports whether a candidate ticket's write_scope
// overlaps with any currently-claimed sibling under the same project, using
// the pre-fetched claimedScopes map instead of a per-candidate SQL query.
func isScopeBlockedBySiblingMap(c *candidate, claimedScopes map[int64][][]string) bool {
	if len(c.scope) == 0 || !c.projectID.Valid {
		return false
	}
	for _, sibScope := range claimedScopes[c.projectID.Int64] {
		if scopesOverlap(c.scope, sibScope) {
			return true
		}
	}
	return false
}

// OpenChangeRequests returns all change requests with status "open" for the
// ticket with the given identifier, ordered by date ascending.
func (d *DB) OpenChangeRequests(identifier string) ([]models.ChangeRequest, error) {
	var ticketID int64
	err := d.db.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&ticketID)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket %q not found", identifier)
	}
	if err != nil {
		return nil, err
	}

	rows, err := d.db.Query(`
		SELECT id, filename, line_number, commit_hash, status, date, author, description
		FROM change_requests
		WHERE ticket_id = ? AND status = ?
		ORDER BY date ASC
	`, ticketID, models.ChangeRequestOpen)
	if err != nil {
		return nil, fmt.Errorf("query open change requests: %w", err)
	}
	defer rows.Close()

	var results []models.ChangeRequest
	for rows.Next() {
		var id int64
		var filename, commitHash, status, author, description string
		var lineNumber int
		var date int64
		if err := rows.Scan(&id, &filename, &lineNumber, &commitHash, &status, &date, &author, &description); err != nil {
			return nil, fmt.Errorf("scan change request: %w", err)
		}
		results = append(results, models.ChangeRequest{
			ID:           strconv.FormatInt(id, 10),
			CommitHash:   commitHash,
			CodeLocation: fmt.Sprintf("%s:%d", filename, lineNumber),
			Status:       status,
			Date:         time.Unix(date, 0),
			Author:       author,
			Description:  description,
		})
	}
	return results, rows.Err()
}

// UpdateChangeRequestDescription updates the description of the change request with the given id.
// The id is accepted as a string to match models.ChangeRequest.ID; the conversion
// to the underlying integer primary key is handled internally.
func (d *DB) UpdateChangeRequestDescription(id string, description string) error {
	numID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid change request id %q: %w", id, err)
	}
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE change_requests SET description = ? WHERE id = ?`, description, numID,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("change request %d not found", numID)
		}
		return nil
	})
}

// AppendChangeRequestDescription appends text to the description of the
// change request with the given id, separated by a newline and "---" divider.
func (d *DB) AppendChangeRequestDescription(id int64, text string) error {
	return d.withTx(func(tx *sql.Tx) error {
		var current string
		err := tx.QueryRow(`SELECT description FROM change_requests WHERE id = ?`, id).Scan(&current)
		if err == sql.ErrNoRows {
			return fmt.Errorf("change request %d not found", id)
		}
		if err != nil {
			return err
		}
		updated := current + "\n\n---\n" + text
		_, err = tx.Exec(`UPDATE change_requests SET description = ? WHERE id = ?`, updated, id)
		return err
	})
}

const maxLogEntries = 200

// InsertLog inserts a log entry with the current timestamp and prunes entries
// beyond maxLogEntries by deleting the oldest ones.
func (d *DB) InsertLog(workerNumber int, message string, logfile string) error {
	return d.withTx(func(tx *sql.Tx) error {
		now := time.Now().Unix()
		var nullLogfile sql.NullString
		if logfile != "" {
			nullLogfile = sql.NullString{String: logfile, Valid: true}
		}
		if _, err := tx.Exec(
			`INSERT INTO logs (timestamp, worker_number, message, logfile) VALUES (?, ?, ?, ?)`,
			now, workerNumber, message, nullLogfile,
		); err != nil {
			return fmt.Errorf("insert log: %w", err)
		}

		// Prune oldest entries beyond maxLogEntries.
		_, err := tx.Exec(`
			DELETE FROM logs WHERE id IN (
				SELECT id FROM logs ORDER BY timestamp ASC LIMIT MAX(0, (SELECT COUNT(*) FROM logs) - ?)
			)`, maxLogEntries)
		return err
	})
}

// FindInProgressTickets returns all tickets currently in an active state
// (working or responding).
// The caller is responsible for determining which of these are stale (e.g. by
// checking logfile modification times).
func (d *DB) FindInProgressTickets() ([]*models.WorkUnit, error) {
	rows, err := d.db.Query(`
		SELECT identifier, description, phase, status, claimed_by, last_updated
		FROM tickets
		WHERE status IN ('working', 'responding')
	`)
	if err != nil {
		return nil, fmt.Errorf("find in-progress tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*models.WorkUnit
	for rows.Next() {
		var identifier, description, phase, status string
		var claimedBy sql.NullInt64
		var lastUpdated int64
		if err := rows.Scan(&identifier, &description, &phase, &status, &claimedBy, &lastUpdated); err != nil {
			return nil, fmt.Errorf("scan in-progress ticket: %w", err)
		}
		wu := &models.WorkUnit{
			Identifier:  identifier,
			Description: description,
			Phase:       models.TicketPhase(phase),
			Status:      models.TicketStatus(status),
			LastUpdated: time.Unix(lastUpdated, 0),
			IsProject:   false,
		}
		if claimedBy.Valid {
			wu.ClaimedBy = strconv.FormatInt(claimedBy.Int64, 10)
		}
		tickets = append(tickets, wu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan in-progress tickets: %w", err)
	}
	return tickets, nil
}

// TicketStats holds aggregate ticket counts used by the status pane.
type TicketStats struct {
	Total int
	Done  int
}

// TicketStats returns the total number of tickets and how many are in the
// "done" phase.
func (d *DB) TicketStats() (TicketStats, error) {
	var stats TicketStats
	err := d.db.QueryRow(`SELECT COUNT(*) FROM tickets`).Scan(&stats.Total)
	if err != nil {
		return stats, fmt.Errorf("ticket stats total: %w", err)
	}
	err = d.db.QueryRow(`SELECT COUNT(*) FROM tickets WHERE phase = 'done'`).Scan(&stats.Done)
	if err != nil {
		return stats, fmt.Errorf("ticket stats done: %w", err)
	}
	return stats, nil
}

// UpdateDescription updates the description of a work unit (ticket or project)
// identified by identifier.
func (d *DB) UpdateDescription(identifier string, newDescription string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE tickets SET description = ? WHERE identifier = ?`,
			newDescription, identifier,
		)
		if err != nil {
			return fmt.Errorf("update ticket description: %w", err)
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			return nil
		}
		res, err = tx.Exec(
			`UPDATE projects SET description = ? WHERE identifier = ?`,
			newDescription, identifier,
		)
		if err != nil {
			return fmt.Errorf("update project description: %w", err)
		}
		n, _ = res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("work unit %q not found", identifier)
		}
		return nil
	})
}

// ActionableTickets returns tickets with status 'needs-attention' or
// 'user-review', ordered so that 'needs-attention' tickets come first and
// within each group tickets are sorted by last_updated ascending (oldest first).
func (d *DB) ActionableTickets() ([]*models.WorkUnit, error) {
	rows, err := d.db.Query(`
		SELECT identifier, description, phase, status, claimed_by, last_updated
		FROM tickets
		WHERE status IN ('needs-attention', 'user-review')
		ORDER BY CASE status WHEN 'needs-attention' THEN 0 ELSE 1 END, last_updated ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("actionable tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*models.WorkUnit
	for rows.Next() {
		var identifier, description, phase, status string
		var claimedBy sql.NullInt64
		var lastUpdated int64
		if err := rows.Scan(&identifier, &description, &phase, &status, &claimedBy, &lastUpdated); err != nil {
			return nil, fmt.Errorf("scan actionable ticket: %w", err)
		}
		wu := &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			Status:       models.TicketStatus(status),
			LastUpdated:  time.Unix(lastUpdated, 0),
			IsProject:    false,
			Dependencies: []string{},
		}
		if claimedBy.Valid {
			wu.ClaimedBy = strconv.FormatInt(claimedBy.Int64, 10)
		}
		tickets = append(tickets, wu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan actionable tickets: %w", err)
	}
	return tickets, nil
}

// GetTicket returns a single ticket by identifier with its dependencies and
// change requests populated. It is more efficient than Status() when only one
// ticket is needed.
func (d *DB) GetTicket(identifier string) (*models.WorkUnit, error) {
	var id int64
	var description, phase, status, parentBranch string
	var claimedBy sql.NullInt64
	var lastUpdated int64
	var projectID sql.NullInt64
	err := d.db.QueryRow(
		`SELECT id, description, phase, status, claimed_by, last_updated, project_id, parent_branch
		 FROM tickets WHERE identifier = ?`, identifier,
	).Scan(&id, &description, &phase, &status, &claimedBy, &lastUpdated, &projectID, &parentBranch)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("ticket %q not found", identifier)
	}
	if err != nil {
		return nil, fmt.Errorf("get ticket: %w", err)
	}

	wu := &models.WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Phase:        models.TicketPhase(phase),
		Status:       models.TicketStatus(status),
		IsProject:    false,
		LastUpdated:  time.Unix(lastUpdated, 0),
		Dependencies: []string{},
		ParentBranch: parentBranch,
	}
	if claimedBy.Valid {
		wu.ClaimedBy = strconv.FormatInt(claimedBy.Int64, 10)
	}
	if projectID.Valid {
		var parentIdent string
		err := d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&parentIdent)
		if err == nil {
			wu.Parent = parentIdent
		}
	}

	// Load dependencies for this ticket.
	depRows, err := d.db.Query(
		`SELECT dependency_type, dependency_id FROM dependencies
		 WHERE work_unit_type = ? AND work_unit_id = ?`,
		workUnitTypeTicket, id,
	)
	if err != nil {
		return nil, fmt.Errorf("get ticket dependencies: %w", err)
	}
	defer depRows.Close()
	for depRows.Next() {
		var depType, depID int64
		if err := depRows.Scan(&depType, &depID); err != nil {
			return nil, fmt.Errorf("scan ticket dependency: %w", err)
		}
		var depIdentifier string
		switch depType {
		case workUnitTypeTicket:
			err = d.db.QueryRow(`SELECT identifier FROM tickets WHERE id = ?`, depID).Scan(&depIdentifier)
		case workUnitTypeProject:
			err = d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, depID).Scan(&depIdentifier)
		}
		if err == nil && depIdentifier != "" {
			wu.Dependencies = append(wu.Dependencies, depIdentifier)
		}
	}
	if err := depRows.Err(); err != nil {
		return nil, fmt.Errorf("scan ticket dependencies: %w", err)
	}

	// Load change requests for this ticket.
	crRows, err := d.db.Query(
		`SELECT id, filename, line_number, commit_hash, status, date, author, description
		 FROM change_requests WHERE ticket_id = ? ORDER BY date ASC`, id,
	)
	if err != nil {
		return nil, fmt.Errorf("get ticket change requests: %w", err)
	}
	defer crRows.Close()
	for crRows.Next() {
		var crID int64
		var filename, commitHash, cstatus, author, crDescription string
		var lineNumber int
		var date int64
		if err := crRows.Scan(&crID, &filename, &lineNumber, &commitHash, &cstatus, &date, &author, &crDescription); err != nil {
			return nil, fmt.Errorf("scan ticket change request: %w", err)
		}
		wu.ChangeRequests = append(wu.ChangeRequests, models.ChangeRequest{
			ID:           strconv.FormatInt(crID, 10),
			CommitHash:   commitHash,
			CodeLocation: fmt.Sprintf("%s:%d", filename, lineNumber),
			Status:       cstatus,
			Date:         time.Unix(date, 0),
			Author:       author,
			Description:  crDescription,
		})
	}
	if err := crRows.Err(); err != nil {
		return nil, fmt.Errorf("scan ticket change requests: %w", err)
	}

	return wu, nil
}

// GetTicketPhase returns the current phase of the ticket with the given
// identifier, or an error if it is not found.
func (d *DB) GetTicketPhase(identifier string) (models.TicketPhase, error) {
	var phase string
	err := d.db.QueryRow(`SELECT phase FROM tickets WHERE identifier = ?`, identifier).Scan(&phase)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("ticket %q not found", identifier)
	}
	if err != nil {
		return "", err
	}
	return models.TicketPhase(phase), nil
}

// SetProjectPhase updates the phase of the project with the given identifier.
// When phase is "done", the project's branch is rebased onto its parent and
// fast-forwarded in (or into repoRoot if there is no parent), producing linear
// history with no merge commit.
func (d *DB) SetProjectPhase(identifier, phase string) error {
	var projectID int64
	var parentProjectID sql.NullInt64
	var parentBranch string
	if err := d.db.QueryRow(
		`SELECT id, project_id, parent_branch FROM projects WHERE identifier = ?`, identifier,
	).Scan(&projectID, &parentProjectID, &parentBranch); err != nil {
		return fmt.Errorf("project %q not found", identifier)
	}

	if phase == string(models.ProjectPhaseDone) {
		mergeTarget := d.mergeTargetDir(parentBranch, parentProjectID)

		mu := d.mergeLock(mergeTarget)
		mu.Lock()
		defer mu.Unlock()

		if err := d.rebaseAndFastForward(identifier, mergeTarget); err != nil {
			return err
		}
	}

	if err := d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE projects SET phase = ?, last_updated = ? WHERE identifier = ?`,
			phase, time.Now().Unix(), identifier,
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("project %q not found", identifier)
		}
		return nil
	}); err != nil {
		return err
	}

	if phase == string(models.ProjectPhaseDone) {
		if err := d.unblockDependents(workUnitTypeProject, projectID); err != nil {
			return fmt.Errorf("unblock dependents: %w", err)
		}
	}
	return nil
}

// AllChildrenDone reports whether all direct children (tickets and subprojects)
// of the project with the given identifier have phase "done".
func (d *DB) AllChildrenDone(projectIdentifier string) (bool, error) {
	var projectID int64
	err := d.db.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, projectIdentifier).Scan(&projectID)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("project %q not found", projectIdentifier)
	}
	if err != nil {
		return false, err
	}

	// Count tickets that are NOT done and belong directly to this project.
	var notDone int
	err = d.db.QueryRow(
		`SELECT COUNT(*) FROM tickets WHERE project_id = ? AND phase != 'done'`,
		projectID,
	).Scan(&notDone)
	if err != nil {
		return false, err
	}
	if notDone > 0 {
		return false, nil
	}

	// Count subprojects that are NOT done and belong directly to this project.
	err = d.db.QueryRow(
		`SELECT COUNT(*) FROM projects WHERE project_id = ? AND phase != 'done'`,
		projectID,
	).Scan(&notDone)
	if err != nil {
		return false, err
	}
	if notDone > 0 {
		return false, nil
	}

	// Make sure there is at least one child (avoid marking empty projects done).
	var total int
	err = d.db.QueryRow(`
		SELECT (SELECT COUNT(*) FROM tickets WHERE project_id = ?) +
		       (SELECT COUNT(*) FROM projects WHERE project_id = ?)
	`, projectID, projectID).Scan(&total)
	if err != nil {
		return false, err
	}
	return total > 0, nil
}

// GetLogs returns all log entries ordered by timestamp ascending.
func (d *DB) GetLogs() ([]models.LogEntry, error) {
	rows, err := d.db.Query(
		`SELECT id, timestamp, worker_number, message, logfile FROM logs ORDER BY timestamp ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}
	defer rows.Close()

	var entries []models.LogEntry
	for rows.Next() {
		var e models.LogEntry
		var ts int64
		var nullLogfile sql.NullString
		if err := rows.Scan(&e.ID, &ts, &e.WorkerNumber, &e.Message, &nullLogfile); err != nil {
			return nil, fmt.Errorf("scan log entry: %w", err)
		}
		e.Timestamp = time.Unix(ts, 0)
		if nullLogfile.Valid {
			e.Logfile = nullLogfile.String
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan logs: %w", err)
	}
	return entries, nil
}
