package db

import "github.com/fimmtiu/code-factory/internal/models"

// ProjectContext holds the identifier and description of a project in the
// hierarchy above a ticket.
type ProjectContext struct {
	Identifier  string
	Description string
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
