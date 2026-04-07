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

	w.Status = StatusBusy
	if err := w.database.SetStatus(identifier, ticket.Phase, models.StatusInProgress); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting in-progress on %s: %v", identifier, err))
		w.Status = StatusIdle
		return
	}

	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("claimed ticket %s", identifier))

	worktreePath := storage.TicketWorktreePathIn(w.ticketsDir, identifier)
	logfilePath := NextLogfilePath(w.ticketsDir, identifier, string(ticket.Phase))

	prompt, err := BuildPrompt(ticket, w.database, w.ticketsDir)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error building prompt for %s: %v", identifier, err))
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	acpErr := w.workFn(ctx, w, w.database, w.logCh, WorkParams{
		WorktreePath: worktreePath,
		Identifier:   identifier,
		Phase:        ticket.Phase,
		Prompt:       prompt,
		LogfilePath:  logfilePath,
	})

	// On graceful shutdown the context is cancelled before the work finishes.
	// Reset the ticket to idle so it is re-processed on the next run rather
	// than incorrectly advancing to user-review.
	if ctx.Err() != nil {
		_ = w.database.SetStatus(identifier, ticket.Phase, models.StatusIdle)
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	if acpErr != nil {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("ACP error on %s: %v", identifier, acpErr), logfilePath)
	}
	w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("completed processing ticket %s", identifier), logfilePath)

	if err := w.database.SetStatus(identifier, ticket.Phase, models.StatusUserReview); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
	}
	w.releaseTicket(identifier)

	w.Status = StatusIdle
}

// releaseTicket clears the claim on a ticket and sends a log message.
func (w *Worker) releaseTicket(identifier string) {
	if err := w.database.Release(identifier); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error releasing ticket %s: %v", identifier, err))
		return
	}
	w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("released ticket %s", identifier))
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
