package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

// run is the main loop for a worker goroutine. It claims tickets, processes
// them via the ACP Claude subprocess, and releases them, respecting pause/unpause
// and shutdown signals.
func (w *Worker) run(ctx context.Context, pollIntervalSecs int) {
	pollInterval := time.Duration(pollIntervalSecs) * time.Second

	for {
		w.drainMessages()

		select {
		case <-ctx.Done():
			return
		default:
		}

		if !w.Paused {
			ticket, err := w.database.Claim(w.Number)
			if err == nil {
				w.processTicket(ctx, ticket)
				continue
			}
		}

		w.waitForNextPoll(ctx, pollInterval)
	}
}

// processTicket sets the ticket to in-progress, runs the Claude ACP subprocess,
// transitions the ticket to user-review, and releases it.
func (w *Worker) processTicket(ctx context.Context, ticket *models.WorkUnit) {
	identifier := ticket.Identifier

	// Create a per-task context so housekeeping can abort just this task
	// without tearing down the entire pool.
	taskCtx, taskCancel := context.WithCancel(ctx)
	w.setCancel(taskCancel)
	defer w.setCancel(nil)
	defer taskCancel()

	w.SetCurrentTicket(string(ticket.Phase) + " " + identifier)
	w.Status = StatusBusy
	if err := w.database.SetStatus(identifier, ticket.Phase, models.StatusInProgress); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting in-progress on %s: %v", identifier, err))
		w.SetCurrentTicket("")
		w.Status = StatusIdle
		return
	}

	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("claimed ticket %s", identifier))

	// Rebase onto the parent branch at the start of every phase so the
	// ticket sees work from sibling tickets that have already been merged.
	if err := w.database.RebaseTicketOnParent(identifier, ticket.Parent, ticket.ParentBranch); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("warning: rebase failed for %s, continuing on stale base: %v", identifier, err))
	}

	worktreePath := storage.TicketWorktreePathIn(w.ticketsDir, identifier)
	logfilePath := NextLogfilePath(w.ticketsDir, identifier, string(ticket.Phase))

	prompt, err := BuildPrompt(ticket, w.database, w.ticketsDir)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error building prompt for %s: %v", identifier, err))
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	acpErr := w.workFn(taskCtx, w, w.database, w.logCh, WorkParams{
		WorktreePath: worktreePath,
		Identifier:   identifier,
		Phase:        ticket.Phase,
		Prompt:       prompt,
		LogfilePath:  logfilePath,
	})

	if taskCtx.Err() != nil {
		w.handleAbort(ctx, identifier, ticket.Phase)
		return
	}

	if acpErr != nil {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("ACP error on %s: %v", identifier, acpErr), logfilePath)
		if err := w.database.SetStatus(identifier, ticket.Phase, models.StatusIdle); err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error resetting %s after ACP error: %v", identifier, err))
		}
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("completed processing ticket %s", identifier), logfilePath)
	if err := w.database.SetStatus(identifier, ticket.Phase, models.StatusUserReview); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
	}
	w.releaseTicket(identifier)
	w.Status = StatusIdle
}

// releaseTicket clears the claim on a ticket, clears display state, and
// sends a log message.
func (w *Worker) releaseTicket(identifier string) {
	w.SetCurrentTicket("")
	w.SetActivity("")
	w.SetLastActivityAt(time.Time{})
	if err := w.database.Release(identifier); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error releasing ticket %s: %v", identifier, err))
		return
	}
	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("released ticket %s", identifier))
}

// handleAbort cleans up after a task context cancellation. If the pool context
// is also cancelled (shutdown), the ticket is still ours and we reset it. If
// only the task context was cancelled (housekeeping abort), the ticket was
// already reset in the DB and possibly reclaimed, so we only clean up local state.
func (w *Worker) handleAbort(poolCtx context.Context, identifier string, phase models.TicketPhase) {
	if poolCtx.Err() != nil {
		_ = w.database.SetStatus(identifier, phase, models.StatusIdle)
		w.releaseTicket(identifier)
	} else {
		w.SetCurrentTicket("")
		w.SetActivity("")
		w.SetLastActivityAt(time.Time{})
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("aborted stale ticket %s", identifier))
	}
	w.Status = StatusIdle
}

// waitForNextPoll waits for the poll interval to elapse, processing any
// incoming messages during the wait. It returns when the interval expires, a
// shutdown is signaled, or the context is cancelled.
func (w *Worker) waitForNextPoll(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		case msg := <-w.ToWorker:
			w.handleMessage(msg)
		}
	}
}

// drainMessages reads all currently-queued messages from ToWorker without
// blocking. This is called at the top of the main loop so that pause/unpause
// messages sent before or between iterations take effect immediately.
func (w *Worker) drainMessages() {
	for {
		select {
		case msg := <-w.ToWorker:
			w.handleMessage(msg)
		default:
			return
		}
	}
}

// handleMessage processes a single message received on the worker's ToWorker
// channel.
func (w *Worker) handleMessage(msg MainToWorkerMessage) {
	switch msg.Kind {
	case MsgPause:
		w.Paused = true
	case MsgUnpause:
		w.Paused = false
	}
}
