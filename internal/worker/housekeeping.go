package worker

import (
	"context"
	"fmt"
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
func runHousekeeping(ctx context.Context, database *db.DB, logCh chan<- LogMessage) {
	timer := time.NewTimer(housekeepingInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			releaseStaleTickets(database, logCh)
			timer.Reset(housekeepingInterval)
		}
	}
}

// releaseStaleTickets finds tickets that have been in-progress for longer than
// staleThresholdMinutes and releases them.
func releaseStaleTickets(database *db.DB, logCh chan<- LogMessage) {
	stale, err := database.FindStaleTickets(staleThresholdMinutes)
	if err != nil {
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error querying stale tickets: %v", err))
		return
	}
	for _, ticket := range stale {
		if err := database.Release(ticket.Identifier); err != nil {
			logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: error releasing stale ticket %s: %v", ticket.Identifier, err))
			continue
		}
		logCh <- NewLogMessage(0, fmt.Sprintf("housekeeping: released stale ticket %s", ticket.Identifier))
	}
}
