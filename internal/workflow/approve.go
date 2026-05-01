// Package workflow provides high-level ticket lifecycle operations.
package workflow

import (
	"fmt"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

// MessyWorktreeError is returned by Approve when the user attempts to
// finalize a merging ticket whose worktree still has uncommitted state.
// The UI translates this into a "Messy worktree for X, not merging"
// notification rather than a hard error.
type MessyWorktreeError struct {
	Identifier string
}

func (e *MessyWorktreeError) Error() string {
	return fmt.Sprintf("worktree for %q has uncommitted state; not merging", e.Identifier)
}

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
//	review    → merging (a worker then runs the cascading rebase that
//	            merges the ticket up the project tree, invoking a Claude
//	            agent to fix any conflicts)
//	merging   → re-queued as merging/idle so a worker resumes the cascade
//	            (after a previous attempt left the ticket in user-review).
//	            If the worktree still has uncommitted state, no transition
//	            is made and *MessyWorktreeError is returned.
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
		return database.SetStatus(identifier, models.PhaseMerging, models.StatusIdle)
	case models.PhaseMerging:
		// Resume a merge that was previously left in user-review because
		// the agent couldn't auto-resolve a conflict. Only re-queue once
		// the user has reached a clean worktree.
		worktreePath, err := storage.WorktreePathForIdentifier(identifier)
		if err != nil {
			return err
		}
		clean, err := git.IsWorktreeClean(worktreePath)
		if err != nil {
			return err
		}
		if !clean {
			return &MessyWorktreeError{Identifier: identifier}
		}
		return database.SetStatus(identifier, models.PhaseMerging, models.StatusIdle)
	default:
		return fmt.Errorf("ticket %q is in phase %q which cannot be approved", identifier, phase)
	}
}
