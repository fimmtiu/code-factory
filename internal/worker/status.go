package worker

// WorkerStatus represents the current state of a worker goroutine.
type WorkerStatus string

const (
	// StatusIdle means the worker is not currently running an agent.
	StatusIdle WorkerStatus = "idle"

	// StatusAwaitingResponse means the agent has asked a question and is
	// waiting for the main goroutine to supply an answer.
	StatusAwaitingResponse WorkerStatus = "awaiting-response"

	// StatusBusy means the worker is actively running an agent subprocess.
	StatusBusy WorkerStatus = "busy"
)

// String returns a human-readable representation of the worker status.
func (s WorkerStatus) String() string {
	switch s {
	case StatusIdle:
		return "idle"
	case StatusAwaitingResponse:
		return "awaiting response"
	case StatusBusy:
		return "busy"
	default:
		return string(s)
	}
}
