package worker

import "time"

// LogMessage carries a noteworthy event from a worker to the main goroutine
// via the shared log channel.
type LogMessage struct {
	Timestamp    time.Time
	WorkerNumber int
	Message      string
	Logfile      string // optional; empty string if none
}

// NewLogMessage creates a LogMessage with the current timestamp and no
// associated logfile.
func NewLogMessage(workerNumber int, message string) LogMessage {
	return LogMessage{
		Timestamp:    time.Now(),
		WorkerNumber: workerNumber,
		Message:      message,
		Logfile:      "",
	}
}

// NewLogMessageWithFile creates a LogMessage with the current timestamp and
// an associated logfile path.
func NewLogMessageWithFile(workerNumber int, message string, logfile string) LogMessage {
	return LogMessage{
		Timestamp:    time.Now(),
		WorkerNumber: workerNumber,
		Message:      message,
		Logfile:      logfile,
	}
}
