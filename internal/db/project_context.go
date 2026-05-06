package db

import "github.com/fimmtiu/code-factory/internal/models"

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
