package models

import "time"

// LogEntry represents a single log message stored in the logs table.
type LogEntry struct {
	ID           int64
	Timestamp    time.Time
	WorkerNumber int
	Message      string
	Logfile      string
}
