package models

import (
	"time"
)

// LogCategory identifies a semantic category of log message for colour mapping.
type LogCategory string

const (
	LogCategoryError    LogCategory = "error"
	LogCategoryPermReq  LogCategory = "perm_request"
	LogCategoryPermResp LogCategory = "perm_response"
	LogCategoryCommit   LogCategory = "commit"
	LogCategoryClaim    LogCategory = "claim"
	LogCategoryRelease  LogCategory = "release"
	LogCategoryDefault  LogCategory = "default"
)

// LogEntry represents a single log message stored in the logs table.
type LogEntry struct {
	ID           int64
	Timestamp    time.Time
	WorkerNumber int
	Message      string
	Logfile      string
}
