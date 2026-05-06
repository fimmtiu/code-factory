package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fimmtiu/code-factory/internal/git"
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

// processTicket transitions the claimed ticket to its active status
// (working for a normal phase run, responding for a /cf-respond run), runs
// the Claude ACP subprocess, transitions the ticket to user-review, and
// releases it.
func (w *Worker) processTicket(ctx context.Context, ticket *models.WorkUnit) {
	if ticket.Phase == models.PhaseMerging {
		w.processMerging(ctx, ticket)
		return
	}
	identifier := ticket.Identifier

	// Determine whether this is a responding run (ticket was claimed with
	// status "responding") or a normal phase run (claimed idle).
	isResponding := ticket.Status == models.StatusResponding
	activeStatus := models.StatusWorking
	logfilePhase := string(ticket.Phase)
	displayLabel := string(ticket.Phase)
	if isResponding {
		activeStatus = models.StatusResponding
		logfilePhase = "respond"
		displayLabel = "respond"
	}

	// Create a per-task context so housekeeping can abort just this task
	// without tearing down the entire pool.
	taskCtx, taskCancel := context.WithCancel(ctx)
	w.setCancel(taskCancel)
	defer w.setCancel(nil)
	defer taskCancel()

	w.SetCurrentTicket(displayLabel + " " + identifier)
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

	// Rebase onto the parent branch at the start of every phase so the
	// ticket sees work from sibling tickets that have already been merged.
	// Skip the rebase for responding runs: the previous user review is still
	// in-flight and we don't want to drag in sibling work mid-review.
	if !isResponding {
		if err := w.database.RebaseTicketOnParent(identifier, ticket.Parent, ticket.ParentBranch); err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("warning: rebase failed for %s, continuing on stale base: %v", identifier, err))
		}
	}

	worktreePath := storage.TicketWorktreePathIn(w.ticketsDir, identifier)
	logfilePath := NextLogfilePath(w.ticketsDir, identifier, logfilePhase)

	// For a fresh refactor or implement run, capture HEAD so we can later
	// tell whether the agent actually committed anything. An empty refactor
	// (no "refactor:" commits added) skips user-review and advances directly
	// to the review phase. An empty implement leaves the ticket at
	// implement/user-review but raises a notification so the user knows the
	// run produced no work and can investigate the log.
	var preRefactorHEAD, preImplementHEAD string
	if !isResponding && (ticket.Phase == models.PhaseRefactor || ticket.Phase == models.PhaseImplement) {
		head, err := git.Output(worktreePath, "rev-parse", "HEAD")
		if err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("warning: could not capture pre-%s HEAD on %s: %v; will route to user-review", ticket.Phase, identifier, err))
		} else if ticket.Phase == models.PhaseRefactor {
			preRefactorHEAD = head
		} else {
			preImplementHEAD = head
		}
	}

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
		Status:       activeStatus,
		Prompt:       prompt,
		LogfilePath:  logfilePath,
	})

	if taskCtx.Err() != nil {
		w.handleAbort(ctx, identifier, ticket.Phase, isResponding)
		return
	}

	// On error, restore the pre-active status so the ticket stays claimable
	// (idle for normal runs, responding for a /cf-respond run — both are
	// valid claim states).
	resetStatus := models.StatusIdle
	if isResponding {
		resetStatus = models.StatusResponding
	}

	if acpErr != nil {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("ACP error on %s: %v", identifier, acpErr), logfilePath)
		if err := w.database.SetStatus(identifier, ticket.Phase, resetStatus); err != nil {
			w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error resetting %s after ACP error: %v", identifier, err))
		}
		w.releaseTicket(identifier)
		w.Status = StatusIdle
		return
	}

	w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("completed processing ticket %s", identifier), logfilePath)

	nextPhase, nextStatus := w.postWork(ticket, postWorkContext{
		identifier:       identifier,
		worktreePath:     worktreePath,
		logfilePath:      logfilePath,
		isResponding:     isResponding,
		preRefactorHEAD:  preRefactorHEAD,
		preImplementHEAD: preImplementHEAD,
	})
	if err := w.database.SetStatus(identifier, nextPhase, nextStatus); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error setting %s on %s: %v", nextStatus, identifier, err))
	}
	w.releaseTicket(identifier)
	w.Status = StatusIdle
}

// postWorkContext bundles the per-ticket state needed by post-work methods.
type postWorkContext struct {
	identifier       string
	worktreePath     string
	logfilePath      string
	isResponding     bool
	preRefactorHEAD  string
	preImplementHEAD string
}

// postWork dispatches to the appropriate phase-specific post-work handler
// and returns the next phase and status for the ticket.
func (w *Worker) postWork(ticket *models.WorkUnit, ctx postWorkContext) (models.TicketPhase, models.TicketStatus) {
	if ctx.isResponding {
		return w.postWorkRespond(ticket, ctx)
	}
	switch ticket.Phase {
	case models.PhaseRefactor:
		return w.postWorkRefactor(ctx)
	case models.PhaseReview:
		return w.postWorkReview(ticket, ctx)
	case models.PhaseImplement:
		return w.postWorkImplement(ctx)
	default:
		return ticket.Phase, models.StatusUserReview
	}
}

// postWorkRespond checks whether all change requests have been addressed.
// If any remain open, the ticket stays in 'responding' so a worker re-claims
// it and continues, instead of advancing to user-review.
func (w *Worker) postWorkRespond(ticket *models.WorkUnit, ctx postWorkContext) (models.TicketPhase, models.TicketStatus) {
	open, err := w.database.OpenChangeRequests(ctx.identifier)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("error checking open change requests on %s: %v", ctx.identifier, err))
	} else if len(open) > 0 {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("%d change requests still open on %s; re-queuing for respond", len(open), ctx.identifier), ctx.logfilePath)
		return ticket.Phase, models.StatusResponding
	}
	return ticket.Phase, models.StatusUserReview
}

// postWorkRefactor checks whether the refactor agent produced any new
// commits. An empty refactor (no "refactor:" commits) skips user-review
// and advances directly to the review phase.
func (w *Worker) postWorkRefactor(ctx postWorkContext) (models.TicketPhase, models.TicketStatus) {
	if ctx.preRefactorHEAD == "" {
		return models.PhaseRefactor, models.StatusUserReview
	}
	added, err := refactorCommitsAdded(ctx.worktreePath, ctx.preRefactorHEAD)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("warning: could not check refactor commits on %s: %v; routing to user-review", ctx.identifier, err))
		return models.PhaseRefactor, models.StatusUserReview
	}
	if !added {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("refactor on %s added no new commits; advancing to review", ctx.identifier), ctx.logfilePath)
		return models.PhaseReview, models.StatusIdle
	}
	return models.PhaseRefactor, models.StatusUserReview
}

// postWorkReview performs a pre-merge rebase onto the parent branch while
// the agent still has context. The result is advisory — the ticket still
// advances to user-review even on conflict — but a notification is sent
// so the user can investigate early.
func (w *Worker) postWorkReview(ticket *models.WorkUnit, ctx postWorkContext) (models.TicketPhase, models.TicketStatus) {
	if err := w.database.RebaseTicketOnParent(ctx.identifier, ticket.Parent, ticket.ParentBranch); err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("pre-merge dry run conflict on %s: %v", ctx.identifier, err))
		if w.notifCh != nil {
			select {
			case w.notifCh <- "Pre-merge dry run conflict on " + ctx.identifier:
			default:
			}
		}
	} else {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("pre-merge dry run succeeded for %s", ctx.identifier))
	}
	return models.PhaseReview, models.StatusUserReview
}

// postWorkImplement checks whether the implement agent produced any commits.
// An empty implement (no commits added) raises a notification so the user
// can investigate, but the ticket still routes to user-review.
func (w *Worker) postWorkImplement(ctx postWorkContext) (models.TicketPhase, models.TicketStatus) {
	if ctx.preImplementHEAD == "" {
		return models.PhaseImplement, models.StatusUserReview
	}
	added, err := commitsAddedSince(ctx.worktreePath, ctx.preImplementHEAD)
	if err != nil {
		w.logCh <- NewLogMessage(w.Number, fmt.Sprintf("warning: could not check implement commits on %s: %v; routing to user-review", ctx.identifier, err))
		return models.PhaseImplement, models.StatusUserReview
	}
	if !added {
		w.logCh <- NewLogMessageWithFile(w.Number, fmt.Sprintf("implement on %s produced no commits; flagging for user attention", ctx.identifier), ctx.logfilePath)
		if w.notifCh != nil {
			select {
			case w.notifCh <- "Empty implement on " + ctx.identifier:
			default:
			}
		}
	}
	return models.PhaseImplement, models.StatusUserReview
}

// refactorCommitsAdded reports whether any commit reachable from HEAD but
// not from preHEAD has a subject beginning with "refactor:". The refactor
// agent always uses that prefix, so its absence means the run produced no
// new work.
func refactorCommitsAdded(worktreePath, preHEAD string) (bool, error) {
	out, err := git.Output(worktreePath, "log", preHEAD+"..HEAD", "--format=%s")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "refactor:") {
			return true, nil
		}
	}
	return false, nil
}

// commitsAddedSince reports whether HEAD has any commit not reachable from
// preHEAD. Used to detect runs that ended their turn without producing any
// committed work for the next phase to act on (e.g. an implement agent that
// emitted only a preamble before returning end_turn).
func commitsAddedSince(worktreePath, preHEAD string) (bool, error) {
	out, err := git.Output(worktreePath, "rev-list", "--count", preHEAD+"..HEAD")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "0", nil
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
func (w *Worker) handleAbort(poolCtx context.Context, identifier string, phase models.TicketPhase, isResponding bool) {
	if poolCtx.Err() != nil {
		resetStatus := models.StatusIdle
		if isResponding {
			resetStatus = models.StatusResponding
		}
		_ = w.database.SetStatus(identifier, phase, resetStatus)
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
// shutdown is signaled, the context is cancelled, or new work becomes available.
func (w *Worker) waitForNextPoll(ctx context.Context, interval time.Duration) {
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			return
		case <-w.workAvailable:
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
