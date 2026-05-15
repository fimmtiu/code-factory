package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
)

// processMerging runs the cascading rebase for a ticket in PhaseMerging.
// On clean success the ticket (and any parent projects whose children have
// all completed) are marked done and their worktrees removed. If a rebase
// conflict cannot be auto-resolved by the agent, the ticket is left in
// merging/user-review with a "Merge conflict on <ticket>" notification so
// the user can take over. If the ticket contains incomplete-work markers the
// merge is blocked before any git operations and the notification reads
// "Merge blocked: forbidden markers on <ticket>" instead.
func (w *Worker) processMerging(ctx context.Context, ticket *models.WorkUnit) {
	identifier := ticket.Identifier
	activeStatus := models.StatusWorking

	taskCtx, taskCancel := context.WithCancel(ctx)
	w.setCancel(taskCancel)
	defer w.setCancel(nil)
	defer taskCancel()

	w.SetCurrentTicket("merging " + identifier)
	w.SetActiveTicketStatus(activeStatus)
	defer w.SetActiveTicketStatus("")
	w.Status = StatusBusy
	if err := w.database.SetStatus(identifier, ticket.Phase, activeStatus); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting %s on %s: %v", activeStatus, identifier, err))
		w.SetCurrentTicket("")
		w.Status = StatusIdle
		return
	}

	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("claimed ticket %s", identifier))

	logfilePath := NextLogfilePath(w.ticketsDir, identifier, string(models.PhaseMerging))

	siblings, err := w.database.GetSiblingDescriptions(identifier)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error fetching sibling descriptions for %s: %v", identifier, err))
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}
	contexts, err := w.database.GetProjectContext(identifier)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error fetching project context for %s: %v", identifier, err))
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}
	prompt := BuildMergingPrompt(ticket, siblings, contexts)

	onConflict := func(stepIdentifier, stepWorktree string) error {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("merge conflict on %s; running agent in %s", stepIdentifier, stepWorktree), logfilePath)
		return w.workFn(taskCtx, w, w.database, w.logCh, WorkParams{
			WorktreePath: stepWorktree,
			Identifier:   identifier,
			Phase:        models.PhaseMerging,
			Status:       activeStatus,
			Prompt:       prompt,
			LogfilePath:  logfilePath,
		})
	}

	mergeErr := w.database.MergeChain(taskCtx, identifier, onConflict)

	if taskCtx.Err() != nil {
		w.handleAbort(ctx, identifier, ticket.Phase, false)
		return
	}

	if mergeErr == nil {
		// MergeChain finalized the chain — the ticket is already marked
		// done and its worktree removed. Just release the worker slot.
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("merged ticket %s into parent", identifier), logfilePath)
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	var unresolved *db.MergeUnresolvedError
	if errors.As(mergeErr, &unresolved) {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("unresolved merge conflict on %s; awaiting user", identifier), logfilePath)
		if err := w.database.SetStatus(identifier, models.PhaseMerging, models.StatusUserReview); err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
		}
		if w.notifCh != nil {
			select {
			case w.notifCh <- "Merge conflict on " + identifier:
			default:
			}
		}
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	var forbidden *db.ForbiddenMarkersError
	if errors.As(mergeErr, &forbidden) {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("merge blocked on %s: forbidden markers", identifier), logfilePath)
		if err := w.database.SetStatus(identifier, models.PhaseMerging, models.StatusUserReview); err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
		}
		if w.notifCh != nil {
			select {
			case w.notifCh <- "Merge blocked: forbidden markers on " + identifier:
			default:
			}
		}
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	// Anything else (DB error, unexpected git failure) — log it, leave the
	// ticket in user-review with a notification so the user can investigate.
	w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("merging error on %s: %v", identifier, mergeErr), logfilePath)
	if err := w.database.SetStatus(identifier, models.PhaseMerging, models.StatusUserReview); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
	}
	if w.notifCh != nil {
		select {
		case w.notifCh <- "Merging error on " + identifier:
		default:
		}
	}
	w.releaseTicket(identifier)
	w.Status = StatusIdle
}
