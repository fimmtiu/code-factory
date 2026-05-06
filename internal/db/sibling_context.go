package db

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/fimmtiu/code-factory/internal/models"
)

// GetSiblingDescriptions returns the identifiers and descriptions of all
// tickets and subprojects under the same parent project as the given
// identifier, excluding the identifier itself. Both sibling tickets and
// sibling subprojects are included because either can contribute commits
// to the parent branch that cause rebase conflicts.
//
// If the identifier has no parent project (top-level ticket), an empty
// slice is returned.
func (d *DB) GetSiblingDescriptions(identifier string) ([]WorkUnitSummary, error) {
	parent, hasParent := models.ParentIdentifierOf(identifier)
	if !hasParent {
		return nil, nil
	}

	// Look up the parent project's row ID.
	var parentID int64
	err := d.db.QueryRow(
		`SELECT id FROM projects WHERE identifier = ?`, parent,
	).Scan(&parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("GetSiblingDescriptions: look up parent project %q: %w", parent, err)
	}

	// Query all sibling tickets and subprojects under the same parent,
	// excluding the identifier we were given.
	rows, err := d.db.Query(`
		SELECT identifier, description FROM tickets
		WHERE project_id = ? AND identifier != ?
		UNION ALL
		SELECT identifier, description FROM projects
		WHERE project_id = ? AND identifier != ?
	`, parentID, identifier, parentID, identifier)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []WorkUnitSummary
	for rows.Next() {
		var ident, desc string
		if err := rows.Scan(&ident, &desc); err != nil {
			return nil, err
		}
		result = append(result, WorkUnitSummary{
			Identifier:  ident,
			Description: desc,
		})
	}
	return result, rows.Err()
}
