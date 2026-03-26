package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/models"
)

// stubWorkDuration is the amount of time the worker sleeps to simulate doing
// real ACP work. Replaced by actual Claude subprocess execution in phase 4.
const stubWorkDuration = 100 * time.Millisecond

// run is the main loop for a worker goroutine. It claims tickets, processes
// them (stub), and releases them, respecting pause/unpause and shutdown signals.
func (w *Worker) run(ctx context.Context, database *db.DB, logCh chan<- LogMessage, pollIntervalSecs int) {
	pollInterval := time.Duration(pollIntervalSecs) * time.Second

	for {
		// Process any pending messages before deciding what to do next.
		w.drainMessages()

		// Check for shutdown.
		select {
		case <-ctx.Done():
			return
		default:
		}

		// If not paused, try to claim a ticket.
		if !w.Paused {
			ticket, err := database.Claim(w.Number)
			if err == nil {
				// Successfully claimed a ticket — process it.
				w.processTicket(ctx, database, logCh, ticket)
				continue
			}
			// No ticket available — fall through to the poll sleep below.
		}

		// Sleep for the poll interval, but remain responsive to messages and
		// shutdown signals.
		w.waitForNextPoll(ctx, pollInterval)
	}
}

// processTicket sets the ticket to in-progress, does the (stub) work, then
// sets it to user-review and releases it.
func (w *Worker) processTicket(ctx context.Context, database *db.DB, logCh chan<- LogMessage, ticket *models.WorkUnit) {
	identifier := ticket.Identifier

	// Mark the worker and ticket as busy.
	w.Status = StatusBusy
	if err := database.SetStatus(identifier, string(ticket.Phase), models.StatusInProgress); err != nil {
		logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting in-progress on %s: %v", identifier, err))
		w.Status = StatusIdle
		return
	}

	logCh <- NewLogMessage(w.Number, fmt.Sprintf("claimed ticket %s", identifier))

	// Stub work: sleep briefly to simulate processing. In phase 4 this becomes
	// the actual Claude subprocess execution.
	select {
	case <-ctx.Done():
		// Shutdown arrived during work — release the ticket and exit.
		releaseTicket(database, logCh, w.Number, identifier)
		w.Status = StatusIdle
		return
	case <-time.After(stubWorkDuration):
	}

	logCh <- NewLogMessage(w.Number, fmt.Sprintf("completed processing ticket %s", identifier))

	// Transition ticket to user-review and release the claim.
	if err := database.SetStatus(identifier, string(ticket.Phase), models.StatusUserReview); err != nil {
		logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting user-review on %s: %v", identifier, err))
	}
	releaseTicket(database, logCh, w.Number, identifier)

	w.Status = StatusIdle
}

// releaseTicket clears the claim on a ticket and sends a log message.
func releaseTicket(database *db.DB, logCh chan<- LogMessage, workerNumber int, identifier string) {
	if err := database.Release(identifier); err != nil {
		logCh <- NewLogMessage(workerNumber, fmt.Sprintf("error releasing ticket %s: %v", identifier, err))
		return
	}
	logCh <- NewLogMessage(workerNumber, fmt.Sprintf("released ticket %s", identifier))
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
