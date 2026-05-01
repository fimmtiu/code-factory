package worker

import "github.com/fimmtiu/code-factory/internal/db"

// ReleaseStaleTickets exposes the unexported housekeeping sweep for tests.
func ReleaseStaleTickets(pool *Pool, database *db.DB, logCh chan<- LogMessage, ticketsDir string) {
	releaseStaleTickets(pool, database, logCh, ticketsDir)
}
