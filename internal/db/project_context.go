package db

import (
	"database/sql"
	"fmt"

	"github.com/fimmtiu/code-factory/internal/models"
)

// WorkUnitSummary holds the identifier and description of a work unit
// (ticket, subproject, or ancestor project). Used for both sibling lookups
// and ancestor-context lookups.
type WorkUnitSummary struct {
	Identifier  string
	Description string
}

// ProjectContext is an alias retained for readability in call sites that
// deal specifically with ancestor project context.
type ProjectContext = WorkUnitSummary

// GetDependencyContext returns the identifier and description of every work
// unit (ticket or project) that the given identifier depends on. Used by the
// implement-phase prompt to remind the agent which prerequisite APIs already
// exist in the worktree, so it consumes them instead of inventing parallel
// stubs.
//
// Returns an empty slice when the identifier is unknown or has no
// dependencies. The order matches the dependencies table's insertion order.
func (d *DB) GetDependencyContext(identifier string) ([]WorkUnitSummary, error) {
	unitType, unitID, err := d.classifyIdentifier(identifier)
	if err != nil {
		return nil, err
	}
	if unitType == 0 {
		return nil, nil
	}

	rows, err := d.db.Query(
		`SELECT dependency_type, dependency_id
		 FROM dependencies
		 WHERE work_unit_type = ? AND work_unit_id = ?
		 ORDER BY id`,
		unitType, unitID,
	)
	if err != nil {
		return nil, fmt.Errorf("GetDependencyContext: list deps: %w", err)
	}
	defer rows.Close()

	type depRef struct {
		typ int
		id  int64
	}
	var refs []depRef
	for rows.Next() {
		var typ int
		var id int64
		if err := rows.Scan(&typ, &id); err != nil {
			return nil, fmt.Errorf("GetDependencyContext: scan dep: %w", err)
		}
		refs = append(refs, depRef{typ: typ, id: id})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("GetDependencyContext: iterate deps: %w", err)
	}

	result := make([]WorkUnitSummary, 0, len(refs))
	for _, ref := range refs {
		var table string
		switch ref.typ {
		case workUnitTypeTicket:
			table = "tickets"
		case workUnitTypeProject:
			table = "projects"
		default:
			continue
		}
		var summary WorkUnitSummary
		err := d.db.QueryRow(
			`SELECT identifier, description FROM `+table+` WHERE id = ?`,
			ref.id,
		).Scan(&summary.Identifier, &summary.Description)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("GetDependencyContext: load %s: %w", table, err)
		}
		result = append(result, summary)
	}
	return result, nil
}

// classifyIdentifier reports whether identifier names a ticket or a project
// and returns its primary key id. Returns (0, 0, nil) when the identifier is
// unknown.
func (d *DB) classifyIdentifier(identifier string) (int, int64, error) {
	var id int64
	if err := d.db.QueryRow(`SELECT id FROM tickets WHERE identifier = ?`, identifier).Scan(&id); err == nil {
		return workUnitTypeTicket, id, nil
	} else if err != sql.ErrNoRows {
		return 0, 0, fmt.Errorf("classifyIdentifier: tickets: %w", err)
	}
	if err := d.db.QueryRow(`SELECT id FROM projects WHERE identifier = ?`, identifier).Scan(&id); err == nil {
		return workUnitTypeProject, id, nil
	} else if err != sql.ErrNoRows {
		return 0, 0, fmt.Errorf("classifyIdentifier: projects: %w", err)
	}
	return 0, 0, nil
}

// GetProjectContext returns the project identifier and description for the
// ticket's parent, grandparent, etc., walking up the tree from the immediate
// parent to the root. The first element is the direct parent; subsequent
// elements are further ancestors.
//
// If the ticket has no parent project, or the identifier is a project without
// any parent, an empty slice is returned.
func (d *DB) GetProjectContext(identifier string) ([]ProjectContext, error) {
	var result []ProjectContext

	current := identifier
	for {
		parent, hasParent := models.ParentIdentifierOf(current)
		if !hasParent {
			break
		}

		var projIdentifier, description string
		err := d.db.QueryRow(
			`SELECT identifier, description FROM projects WHERE identifier = ?`, parent,
		).Scan(&projIdentifier, &description)
		if err != nil {
			// Parent not found in projects table; stop climbing.
			break
		}

		result = append(result, ProjectContext{
			Identifier:  projIdentifier,
			Description: description,
		})
		current = parent
	}

	return result, nil
}
