// Package workflow provides high-level ticket lifecycle operations.
package workflow

import (
	"fmt"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
)

// Approve advances the given ticket to its next phase. The transitions are:
//
//	implement → refactor
//	refactor  → review
//	review    → respond
//	respond   → done (rebases branch onto parent, fast-forwards the parent,
//	            removes worktree, then checks recursive project completion)
//
// Returns an error if the ticket is not found or is in a phase that cannot be
// approved (blocked, done).
func Approve(database *db.DB, identifier string) error {
	phase, err := database.GetTicketPhase(identifier)
	if err != nil {
		return err
	}

	switch phase {
	case models.PhaseImplement:
		return database.SetStatus(identifier, models.PhaseRefactor, models.StatusIdle)
	case models.PhaseRefactor:
		return database.SetStatus(identifier, models.PhaseReview, models.StatusIdle)
	case models.PhaseReview:
		return database.SetStatus(identifier, models.PhaseRespond, models.StatusIdle)
	case models.PhaseRespond:
		if err := database.SetStatus(identifier, models.PhaseDone, models.StatusIdle); err != nil {
			return err
		}
		return markParentProjectsDone(database, identifier)
	default:
		return fmt.Errorf("ticket %q is in phase %q which cannot be approved", identifier, phase)
	}
}

// markParentProjectsDone walks up the project hierarchy from the given
// identifier, marking each project as done when all its direct children are
// done.
func markParentProjectsDone(database *db.DB, identifier string) error {
	parentID, hasParent := models.ParentIdentifierOf(identifier)
	if !hasParent {
		return nil
	}

	allDone, err := database.AllChildrenDone(parentID)
	if err != nil {
		return err
	}
	if !allDone {
		return nil
	}

	if err := database.SetProjectPhase(parentID, models.ProjectPhaseDone); err != nil {
		return err
	}

	// Recurse up the tree.
	return markParentProjectsDone(database, parentID)
}
