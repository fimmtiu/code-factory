package worker

import (
	"sync"

	"github.com/fimmtiu/code-factory/internal/db"
)

// workerChannelBuffer is the buffer size for the worker communication channels.
const workerChannelBuffer = 16

// Worker represents a single worker slot in the pool. Each worker has a
// 1-based identifier number, a current status, a paused flag, and a pair of
// typed channels for bidirectional communication with the main goroutine.
type Worker struct {
	// Number is a 1-based identifier for this worker.
	Number int

	// Status is the current operational state of the worker.
	Status WorkerStatus

	// Paused indicates that the worker should not pick up new tickets after
	// completing its current work.
	Paused bool

	// ToWorker carries messages from the main goroutine to this worker.
	ToWorker chan MainToWorkerMessage

	// FromWorker carries messages from this worker to the main goroutine.
	FromWorker chan WorkerToMainMessage

	// LastOutput holds the last three lines of agent output, used for display
	// in the worker status view. Protected by mu.
	LastOutput []string

	// mu guards LastOutput for concurrent access between the worker goroutine
	// (writer) and the UI goroutine (reader).
	mu sync.RWMutex

	// database, logCh, ticketsDir, and workFn are set by Pool.Start before the
	// goroutine is launched and remain constant for the worker's lifetime.
	database   *db.DB
	logCh      chan<- LogMessage
	ticketsDir string
	workFn     WorkFn
}

// NewWorker creates a new Worker with the given 1-based number. The worker
// starts in StatusIdle with Paused false and buffered communication channels.
func NewWorker(number int) *Worker {
	return &Worker{
		Number:     number,
		Status:     StatusIdle,
		Paused:     false,
		ToWorker:   make(chan MainToWorkerMessage, workerChannelBuffer),
		FromWorker: make(chan WorkerToMainMessage, workerChannelBuffer),
		LastOutput: []string{},
	}
}

// GetLastOutput returns a copy of the worker's last output lines, safe for
// concurrent access from the UI goroutine.
func (w *Worker) GetLastOutput() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]string, len(w.LastOutput))
	copy(out, w.LastOutput)
	return out
}

// SetLastOutput replaces the worker's last output slice under the write lock.
func (w *Worker) SetLastOutput(lines []string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.LastOutput = lines
}
