package worker

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fimmtiu/code-factory/internal/storage"
)

// AllLogfilePaths returns all existing logfiles for a ticket phase in
// chronological order (oldest first): implement.log, implement.log.1, …
// Returns nil if none exist.
func AllLogfilePaths(ticketsDir, identifier, phase string) []string {
	ticketDir := storage.TicketDirPath(ticketsDir, identifier)
	base := filepath.Join(ticketDir, phase+".log")

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return nil
	}

	paths := []string{base}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			break
		}
		paths = append(paths, candidate)
	}
	return paths
}

// LatestLogfilePath returns the path of the most recently written logfile for
// a ticket phase, or an empty string if none exists.
func LatestLogfilePath(ticketsDir, identifier, phase string) string {
	ticketDir := storage.TicketDirPath(ticketsDir, identifier)
	base := filepath.Join(ticketDir, phase+".log")

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return ""
	}

	latest := base
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			break
		}
		latest = candidate
	}
	return latest
}

// NextLogfilePath returns the next available logfile path for a ticket phase.
// Logfiles are stored at .code-factory/<identifier>/<phase>.log. If that file
// already exists, a monotonically increasing numeric suffix is appended:
// <phase>.log.1, <phase>.log.2, etc.
func NextLogfilePath(ticketsDir, identifier, phase string) string {
	ticketDir := storage.TicketDirPath(ticketsDir, identifier)
	base := filepath.Join(ticketDir, phase+".log")

	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}

	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}
