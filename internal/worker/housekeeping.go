package worker

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
)

// housekeepingInterval is how often the housekeeping goroutine wakes up to
// check for stale tickets.
const housekeepingInterval = 60 * time.Second

// staleThresholdMinutes is the number of minutes after which an in-progress
// ticket is considered stale and will be released.
const staleThresholdMinutes = 10

// runHousekeeping runs until ctx is cancelled. On each wake it queries for
// stale tickets and releases them.
func runHousekeeping(ctx context.Context, pool *Pool, database *db.DB, logCh chan<- LogMessage) {
	timer := time.NewTimer(housekeepingInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			releaseStaleTickets(pool, database, logCh)
			timer.Reset(housekeepingInterval)
		}
	}
}

// releaseStaleTickets finds tickets that have been in-progress for longer than
// staleThresholdMinutes, aborts the owning worker's subprocess, and resets the
// ticket to idle.
func releaseStaleTickets(pool *Pool, database *db.DB, logCh chan<- LogMessage) {
	stale, err := database.FindStaleTickets(staleThresholdMinutes)
	if err != nil {
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error querying stale tickets: %v", err))
		return
	}
	for _, ticket := range stale {
		// Abort the owning worker's subprocess before resetting the DB.
		if ticket.ClaimedBy != "" {
			if num, err := strconv.Atoi(ticket.ClaimedBy); err == nil {
				if w := pool.GetWorker(num); w != nil {
					w.AbortWork()
				}
			}
		}

		if err := database.ResetTicket(ticket.Identifier); err != nil {
			logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error resetting stale ticket %s: %v", ticket.Identifier, err))
			continue
		}
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: reset stale ticket %s to idle", ticket.Identifier))
	}
}
