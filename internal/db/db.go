// Package db provides access to the tickets SQLite database.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"

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

// DB provides read/write access to the tickets SQLite database.
type DB struct {
	db         *sql.DB
	ticketsDir string
	repoRoot   string
	git        gitutil.GitClient
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
var migrations = []string{}

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

// worktreePath returns the git worktree path for a ticket identifier.
func (d *DB) worktreePath(identifier string) string {
	return storage.TicketWorktreePathIn(d.ticketsDir, identifier)
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

// Status returns all work units (projects and tickets) with their dependencies
// and comment threads populated.
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
	rows, err := d.db.Query(`SELECT id, identifier, description, last_updated, phase FROM projects`)
	if err != nil {
		return nil, fmt.Errorf("load projects: %w", err)
	}
	for rows.Next() {
		var id int64
		var identifier, description, phase string
		var lastUpdated int64
		if err := rows.Scan(&id, &identifier, &description, &lastUpdated, &phase); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projectByID[id] = &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			IsProject:    true,
			LastUpdated:  time.Unix(lastUpdated, 0),
			Dependencies: []string{},
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("scan projects: %w", err)
	}
	rows.Close()

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
		`SELECT id, identifier, description, phase, status, claimed_by, last_updated, project_id FROM tickets`,
	)
	if err != nil {
		return nil, fmt.Errorf("load tickets: %w", err)
	}
	for rows.Next() {
		var id int64
		var identifier, description, phase, status string
		var claimedBy sql.NullInt64
		var lastUpdated int64
		var projectID sql.NullInt64
		if err := rows.Scan(&id, &identifier, &description, &phase, &status, &claimedBy, &lastUpdated, &projectID); err != nil {
			rows.Close()
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
		rows.Close()
		return nil, fmt.Errorf("scan tickets: %w", err)
	}
	rows.Close()
	return ticketByID, nil
}

func (d *DB) loadDependencies(projectByID, ticketByID map[int64]*models.WorkUnit) error {
	rows, err := d.db.Query(
		`SELECT work_unit_type, work_unit_id, dependency_type, dependency_id FROM dependencies`,
	)
	if err != nil {
		return fmt.Errorf("load dependencies: %w", err)
	}
	for rows.Next() {
		var wuType, wuID, depType, depID int64
		if err := rows.Scan(&wuType, &wuID, &depType, &depID); err != nil {
			rows.Close()
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
		rows.Close()
		return fmt.Errorf("scan dependencies: %w", err)
	}
	rows.Close()
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
	for rows.Next() {
		var id, ticketID int64
		var filename, commitHash, cstatus, author, description string
		var lineNumber int
		var date int64
		if err := rows.Scan(&id, &ticketID, &filename, &lineNumber, &commitHash, &cstatus, &date, &author, &description); err != nil {
			rows.Close()
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
		rows.Close()
		return fmt.Errorf("scan change requests: %w", err)
	}
	rows.Close()
	return nil
}

// CreateProject inserts a new project (and its dependencies) in a single
// transaction and creates its directory under .tickets/.
func (d *DB) CreateProject(identifier, description string, deps []string) error {
	if err := models.ValidateIdentifier(identifier); err != nil {
		return err
	}
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
			`INSERT INTO projects (identifier, description, last_updated, project_id) VALUES (?, ?, ?, ?)`,
			identifier, description, time.Now().Unix(), parentID,
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
func (d *DB) CreateTicket(identifier, description string, deps []string) error {
	if err := models.ValidateIdentifier(identifier); err != nil {
		return err
	}
	if err := d.withTx(func(tx *sql.Tx) error {
		phase := models.PhaseImplement
		if len(deps) > 0 {
			phase = models.PhaseBlocked
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
			`INSERT INTO tickets (identifier, description, phase, status, last_updated, project_id) VALUES (?, ?, ?, ?, ?, ?)`,
			identifier, description, string(phase), string(models.StatusIdle), time.Now().Unix(), projectID,
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
// is "done", the ticket's branch is merged and its worktree is removed.
func (d *DB) SetStatus(identifier string, phase models.TicketPhase, status models.TicketStatus) error {
	if !models.IsValidTicketPhase(string(phase)) {
		return fmt.Errorf("invalid ticket phase %q", phase)
	}
	if !models.IsValidTicketStatus(string(status)) {
		return fmt.Errorf("invalid ticket status %q", status)
	}

	var ticketID int64
	var projectID sql.NullInt64
	err := d.db.QueryRow(
		`SELECT id, project_id FROM tickets WHERE identifier = ?`, identifier,
	).Scan(&ticketID, &projectID)
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
		return d.markTicketDone(ticketID, identifier, projectID)
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
// HEAD of its parent's branch. For tickets under a project, this is the
// project's worktree branch; for top-level tickets, it is whatever branch is
// checked out at the repo root (typically main). This pulls in work from
// sibling tickets that have already been merged into the parent.
func (d *DB) RebaseTicketOnParent(identifier, parentIdentifier string) error {
	worktreePath := d.worktreePath(identifier)

	var ontoBranch string
	if parentIdentifier != "" {
		ontoBranch = strings.ReplaceAll(parentIdentifier, "/", "_")
	} else {
		branch, err := d.git.GetCurrentBranch(d.repoRoot)
		if err != nil {
			return fmt.Errorf("rebase: detect default branch: %w", err)
		}
		ontoBranch = branch
	}

	return d.git.RebaseOnto(worktreePath, ontoBranch)
}

// markTicketDone merges the ticket's branch into the parent project's worktree
// (or repoRoot if there is no parent), updates the DB, and removes the worktree.
func (d *DB) markTicketDone(ticketID int64, identifier string, projectID sql.NullInt64) error {
	mergeTarget := d.repoRoot
	if projectID.Valid {
		var parentIdentifier string
		if err := d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&parentIdentifier); err == nil {
			mergeTarget = storage.TicketWorktreePathIn(d.ticketsDir, parentIdentifier)
		}
	}

	if err := d.git.MergeBranch(mergeTarget, identifier); err != nil {
		return &MergeConflictError{WorktreePath: mergeTarget, Branch: identifier, Err: err}
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

// claimTicketRow atomically claims the first claimable ticket for pid and
// returns its row id and a partially-populated WorkUnit.
//
// The UPDATE-then-SELECT pattern ensures that only one worker can claim a
// given ticket even under concurrent DEFERRED transactions — the UPDATE's
// WHERE clause acts as an atomic compare-and-swap.
func claimTicketRow(tx *sql.Tx, pid int) (int64, *models.WorkUnit, error) {
	now := time.Now().Unix()

	// Atomically claim the first available ticket. SQLite evaluates the
	// subquery and applies the UPDATE in a single statement, so no other
	// transaction can claim the same row.
	res, err := tx.Exec(`
		UPDATE tickets
		SET claimed_by = ?, last_updated = ?
		WHERE id = (
			SELECT id FROM tickets
			WHERE phase NOT IN ('blocked', 'done')
			  AND status = 'idle'
			  AND claimed_by IS NULL
			LIMIT 1
		)
	`, pid, now)
	if err != nil {
		return 0, nil, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil, err
	}
	if n == 0 {
		return 0, nil, fmt.Errorf("no claimable ticket available")
	}

	// Read back the ticket we just claimed. The status is still 'idle' at this
	// point (the caller transitions it to in-progress), and a worker can only
	// hold one idle ticket at a time, so this is unambiguous.
	var id int64
	var identifier, description, phase, status string
	var projectID sql.NullInt64
	err = tx.QueryRow(`
		SELECT id, identifier, description, phase, status, project_id
		FROM tickets
		WHERE claimed_by = ? AND status = 'idle'
		LIMIT 1
	`, pid).Scan(&id, &identifier, &description, &phase, &status, &projectID)
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
		Parent:       projectIdentifierByID(tx, projectID),
	}
	return id, wu, nil
}

// projectIdentifierByID resolves an optional project row ID to its identifier string.
func projectIdentifierByID(tx *sql.Tx, projectID sql.NullInt64) string {
	if !projectID.Valid {
		return ""
	}
	var identifier string
	_ = tx.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&identifier)
	return identifier
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
		switch depType {
		case workUnitTypeTicket:
			_ = tx.QueryRow(`SELECT identifier FROM tickets WHERE id = ?`, depID).Scan(&depIdentifier)
		case workUnitTypeProject:
			_ = tx.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, depID).Scan(&depIdentifier)
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

// ResetTicket atomically clears the claim and resets the status to idle.
// Used by housekeeping for stale tickets where the worker is presumed dead.
func (d *DB) ResetTicket(identifier string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE tickets SET status = 'idle', claimed_by = NULL, last_updated = ? WHERE identifier = ?`,
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
// (in-progress or needs-attention) back to idle with no claim. This is
// called at startup to recover from hard kills where workers died without
// cleaning up. Returns the number of tickets recovered.
func (d *DB) RecoverOrphanedTickets() (int, error) {
	var count int
	err := d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE tickets SET status = 'idle', claimed_by = NULL, last_updated = ?
			 WHERE status IN ('in-progress', 'needs-attention')`,
			time.Now().Unix(),
		)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		count = int(n)
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
func (d *DB) UpdateChangeRequestDescription(id int64, description string) error {
	return d.withTx(func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`UPDATE change_requests SET description = ? WHERE id = ?`, description, id,
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

// FindInProgressTickets returns all tickets currently in the in-progress state.
// The caller is responsible for determining which of these are stale (e.g. by
// checking logfile modification times).
func (d *DB) FindInProgressTickets() ([]*models.WorkUnit, error) {
	rows, err := d.db.Query(`
		SELECT identifier, description, phase, status, claimed_by, last_updated
		FROM tickets
		WHERE status = 'in-progress'
	`)
	if err != nil {
		return nil, fmt.Errorf("find in-progress tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*models.WorkUnit
	for rows.Next() {
		var identifier, description, phase, status string
		var claimedBy sql.NullString
		var lastUpdated int64
		if err := rows.Scan(&identifier, &description, &phase, &status, &claimedBy, &lastUpdated); err != nil {
			return nil, fmt.Errorf("scan in-progress ticket: %w", err)
		}
		tickets = append(tickets, &models.WorkUnit{
			Identifier:  identifier,
			Description: description,
			Phase:       models.TicketPhase(phase),
			Status:      models.TicketStatus(status),
			ClaimedBy:   claimedBy.String,
			LastUpdated: time.Unix(lastUpdated, 0),
			IsProject:   false,
		})
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
// When phase is "done", the project's branch is merged into the parent project's
// worktree (or repoRoot if there is no parent).
func (d *DB) SetProjectPhase(identifier, phase string) error {
	if phase == string(models.ProjectPhaseDone) {
		mergeTarget := d.repoRoot
		var parentProjectID sql.NullInt64
		if err := d.db.QueryRow(`SELECT project_id FROM projects WHERE identifier = ?`, identifier).Scan(&parentProjectID); err == nil && parentProjectID.Valid {
			var parentIdentifier string
			if err := d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, parentProjectID.Int64).Scan(&parentIdentifier); err == nil {
				mergeTarget = storage.TicketWorktreePathIn(d.ticketsDir, parentIdentifier)
			}
		}
		if err := d.git.MergeBranch(mergeTarget, identifier); err != nil {
			return &MergeConflictError{WorktreePath: mergeTarget, Branch: identifier, Err: err}
		}
	}

	var projectID int64
	if err := d.db.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, identifier).Scan(&projectID); err != nil {
		return fmt.Errorf("project %q not found", identifier)
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
