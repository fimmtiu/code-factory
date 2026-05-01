package worker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
)

// housekeepingInterval is how often the housekeeping goroutine wakes up to
// check for stale tickets.
const housekeepingInterval = 60 * time.Second

// staleThreshold is the duration of logfile inactivity after which an
// in-progress ticket is considered stale and will be released.
const staleThreshold = 10 * time.Minute

// runHousekeeping runs until ctx is cancelled. On each wake it queries for
// stale tickets and releases them.
func runHousekeeping(ctx context.Context, pool *Pool, database *db.DB, logCh chan<- LogMessage, ticketsDir string) {
	timer := time.NewTimer(housekeepingInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			releaseStaleTickets(pool, database, logCh, ticketsDir)
			timer.Reset(housekeepingInterval)
		}
	}
}

// releaseStaleTickets finds in-progress tickets whose logfile has not been
// modified within staleThreshold, aborts the owning worker's subprocess, and
// resets the ticket to idle.
func releaseStaleTickets(pool *Pool, database *db.DB, logCh chan<- LogMessage, ticketsDir string) {
	tickets, err := database.FindInProgressTickets()
	if err != nil {
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error querying in-progress tickets: %v", err))
		return
	}
	now := time.Now()
	for _, ticket := range tickets {
		// A ticket with no claim is just sitting in the queue waiting for a
		// free worker (most often a 'responding' ticket the user approved
		// while every worker was busy). There is no stuck subprocess to
		// abort, and the next free worker will pick it up — staleness only
		// applies to tickets a worker has actually grabbed.
		if ticket.ClaimedBy == "" {
			continue
		}
		// During a /cf-respond run the ticket's phase is still 'review' (or
		// whichever phase was active), but the active log is respond.log.
		// Use that so we don't incorrectly flag a working respond run as
		// stale just because the prior phase log is old.
		logfilePhase := string(ticket.Phase)
		if ticket.Status == models.StatusResponding {
			logfilePhase = "respond"
		}
		logfile := LatestLogfilePath(ticketsDir, ticket.Identifier, logfilePhase)
		if logfile == "" {
			// No logfile for this phase yet (the worker may have just started
			// and not created the file). Fall back to the DB timestamp.
			if now.Sub(ticket.LastUpdated) < staleThreshold {
				continue
			}
		} else if !IsLogfileStale(logfile, now) {
			continue
		}

		// Abort the owning worker's subprocess before resetting the DB.
		if num, err := strconv.Atoi(ticket.ClaimedBy); err == nil {
			if w := pool.GetWorker(num); w != nil {
				w.AbortWork()
			}
		}

		if err := database.ResetTicket(ticket.Identifier); err != nil {
			logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error resetting stale ticket %s: %v", ticket.Identifier, err))
			continue
		}
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: released stale ticket %s (no logfile activity for %s)", ticket.Identifier, staleThreshold))
	}
}

// IsLogfileStale returns true if the logfile is missing or has not been
// modified within staleThreshold of now.
func IsLogfileStale(logfile string, now time.Time) bool {
	if logfile == "" {
		return true
	}
	info, err := os.Stat(logfile)
	if err != nil {
		return true
	}
	return now.Sub(info.ModTime()) >= staleThreshold
}
