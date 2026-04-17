// Package workflow provides high-level ticket lifecycle operations.
package workflow

import (
	"fmt"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
)

// Approve advances the given ticket to its next phase, or queues it for
// /cf-respond if the reviewer left open change requests.
//
// If the ticket has any open change requests, its status is set to
// "responding" (phase unchanged) so a worker will pick it up and run the
// /cf-respond skill. When that finishes the worker sets the ticket back to
// "user-review" for the next round of approval.
//
// Otherwise the phase advances:
//
//	implement → refactor
//	refactor  → review
//	review    → done (rebases the ticket up the project tree; only marked
//	            done if every rebase up to the root succeeds, so a conflict
//	            at any level leaves the ticket re-approvable after the user
//	            resolves it)
//
// Returns an error if the ticket is not found or is in a phase that cannot be
// approved (blocked, done).
func Approve(database *db.DB, identifier string) error {
	phase, err := database.GetTicketPhase(identifier)
	if err != nil {
		return err
	}

	switch phase {
	case models.PhaseImplement, models.PhaseRefactor, models.PhaseReview:
		crs, err := database.OpenChangeRequests(identifier)
		if err != nil {
			return err
		}
		if len(crs) > 0 {
			return database.SetStatus(identifier, phase, models.StatusResponding)
		}
	}

	switch phase {
	case models.PhaseImplement:
		return database.SetStatus(identifier, models.PhaseRefactor, models.StatusIdle)
	case models.PhaseRefactor:
		return database.SetStatus(identifier, models.PhaseReview, models.StatusIdle)
	case models.PhaseReview:
		return database.MarkTicketDoneCascading(identifier)
	default:
		return fmt.Errorf("ticket %q is in phase %q which cannot be approved", identifier, phase)
	}
}
