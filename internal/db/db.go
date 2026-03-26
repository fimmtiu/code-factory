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

	"github.com/fimmtiu/tickets/internal/gitutil"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
)

const (
	workUnitTypeTicket  = 1
	workUnitTypeProject = 2
)

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

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// withTx executes fn inside a transaction, committing on success and rolling
// back on any error returned by fn.
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

func (d *DB) createSchema() error {
	return d.withTx(func(tx *sql.Tx) error {
		for _, stmt := range schemaStatements {
			if _, err := tx.Exec(stmt); err != nil {
				return err
			}
		}
		return nil
	})
}

// --- Helper functions ---

// worktreePath returns the git worktree path for a ticket identifier.
func (d *DB) worktreePath(identifier string) string {
	return storage.TicketWorktreePath(storage.TicketDirPath(d.ticketsDir, identifier))
}

// parentIdentifierOf returns the parent portion of a slash-separated identifier
// (e.g. "proj/ticket" → "proj", true) and whether one was found.
func parentIdentifierOf(identifier string) (string, bool) {
	idx := strings.LastIndex(identifier, "/")
	if idx < 0 {
		return "", false
	}
	return identifier[:idx], true
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

// --- Public operations ---

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
			Phase:        phase,
			IsProject:    true,
			LastUpdated:  time.Unix(lastUpdated, 0).UTC(),
			Dependencies: []string{},
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("scan projects: %w", err)
	}
	rows.Close()

	for _, wu := range projectByID {
		if parent, ok := parentIdentifierOf(wu.Identifier); ok {
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
			LastUpdated:  time.Unix(lastUpdated, 0).UTC(),
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
			Date:         time.Unix(date, 0).UTC(),
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
		if parent, hasParent := parentIdentifierOf(identifier); hasParent {
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
// transaction and creates its directory under .tickets/.
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
		if parent, hasParent := parentIdentifierOf(identifier); hasParent {
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
	return os.MkdirAll(storage.TicketDirPath(d.ticketsDir, identifier), 0755)
}

// SetStatus updates the phase (and optionally status) of a ticket. When phase
// is "done", the ticket's branch is merged and its worktree is removed. When
// phase is "implement" and status is "in-progress", a git worktree is created
// if one does not already exist.
func (d *DB) SetStatus(identifier, phase, status string) error {
	if !models.IsValidTicketPhase(phase) {
		return fmt.Errorf("invalid ticket phase %q", phase)
	}
	if !models.IsValidTicketStatus(status) {
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

	if phase == string(models.PhaseDone) {
		return d.markTicketDone(ticketID, identifier, projectID)
	}

	if phase == string(models.PhaseImplement) && status == string(models.StatusInProgress) {
		wtp := d.worktreePath(identifier)
		if _, err := d.git.GetHeadCommit(wtp); err != nil {
			if err := d.git.CreateWorktree(d.repoRoot, wtp, identifier); err != nil {
				return fmt.Errorf("create worktree: %w", err)
			}
		}
	}

	return d.withTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`UPDATE tickets SET phase = ?, status = ? WHERE id = ?`,
			phase, status, ticketID,
		)
		return err
	})
}

// markTicketDone merges the ticket's branch, updates it to done in the DB, and
// removes its worktree.
func (d *DB) markTicketDone(ticketID int64, identifier string, projectID sql.NullInt64) error {
	intoBranch := "main"
	if projectID.Valid {
		var parentIdentifier string
		if err := d.db.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&parentIdentifier); err == nil {
			intoBranch = parentIdentifier
		}
	}

	if err := d.git.MergeBranch(d.repoRoot, identifier, intoBranch); err != nil {
		return fmt.Errorf("merge failed: %w", err)
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

	if err := d.git.RemoveWorktree(d.repoRoot, d.worktreePath(identifier), identifier); err != nil {
		panic(err)
	}
	return nil
}

// Claim assigns the first claimable ticket to the given PID and returns it.
func (d *DB) Claim(pid int) (*models.WorkUnit, error) {
	var result *models.WorkUnit
	err := d.withTx(func(tx *sql.Tx) error {
		var id int64
		var identifier, description, phase, status string
		var lastUpdated int64
		var projectID sql.NullInt64
		err := tx.QueryRow(`
			SELECT id, identifier, description, phase, status, last_updated, project_id
			FROM tickets
			WHERE phase NOT IN ('blocked', 'done')
			  AND status = 'idle'
			  AND claimed_by IS NULL
			LIMIT 1
		`).Scan(&id, &identifier, &description, &phase, &status, &lastUpdated, &projectID)
		if err == sql.ErrNoRows {
			return fmt.Errorf("no claimable ticket available")
		}
		if err != nil {
			return err
		}

		now := time.Now().Unix()
		if _, err := tx.Exec(
			`UPDATE tickets SET claimed_by = ?, last_updated = ? WHERE id = ?`,
			pid, now, id,
		); err != nil {
			return err
		}

		wu := &models.WorkUnit{
			Identifier:   identifier,
			Description:  description,
			Phase:        models.TicketPhase(phase),
			Status:       models.TicketStatus(status),
			ClaimedBy:    strconv.Itoa(pid),
			LastUpdated:  time.Unix(now, 0).UTC(),
			IsProject:    false,
			Dependencies: []string{},
		}
		if projectID.Valid {
			var parentIdentifier string
			if err := tx.QueryRow(`SELECT identifier FROM projects WHERE id = ?`, projectID.Int64).Scan(&parentIdentifier); err == nil {
				wu.Parent = parentIdentifier
			}
		}

		depRows, err := tx.Query(`
			SELECT dependency_type, dependency_id FROM dependencies
			WHERE work_unit_type = ? AND work_unit_id = ?
		`, workUnitTypeTicket, id)
		if err != nil {
			return err
		}
		defer depRows.Close()
		for depRows.Next() {
			var depType int
			var depID int64
			if err := depRows.Scan(&depType, &depID); err != nil {
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
		if err := depRows.Err(); err != nil {
			return err
		}

		result = wu
		return nil
	})
	return result, err
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

// FindStaleTickets returns all tickets that have been in-progress for longer
// than thresholdMinutes minutes. These are candidates for release by the
// housekeeping thread.
func (d *DB) FindStaleTickets(thresholdMinutes int) ([]*models.WorkUnit, error) {
	cutoff := time.Now().Add(-time.Duration(thresholdMinutes) * time.Minute).Unix()
	rows, err := d.db.Query(`
		SELECT identifier, description, phase, status, last_updated
		FROM tickets
		WHERE status = 'in-progress' AND last_updated < ?
	`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("find stale tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*models.WorkUnit
	for rows.Next() {
		var identifier, description, phase, status string
		var lastUpdated int64
		if err := rows.Scan(&identifier, &description, &phase, &status, &lastUpdated); err != nil {
			return nil, fmt.Errorf("scan stale ticket: %w", err)
		}
		tickets = append(tickets, &models.WorkUnit{
			Identifier:  identifier,
			Description: description,
			Phase:       models.TicketPhase(phase),
			Status:      models.TicketStatus(status),
			LastUpdated: time.Unix(lastUpdated, 0).UTC(),
			IsProject:   false,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan stale tickets: %w", err)
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
			LastUpdated:  time.Unix(lastUpdated, 0).UTC(),
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
		e.Timestamp = time.Unix(ts, 0).UTC()
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
