package worker

import (
	"context"
	"sync"

	"github.com/fimmtiu/code-factory/internal/db"
)

// workerChannelBuffer is the buffer size for the worker communication channels.
const workerChannelBuffer = 16

// OutputLines is the number of recent output lines each worker keeps for
// display in the Workers view.
const OutputLines = 4

// PermissionOption is one choice in a pending permission request.
type PermissionOption struct {
	Name     string
	Kind     string
	OptionID string
}

// PendingPermissionRequest holds the details of an in-flight permission
// request that is awaiting a response from the user.
type PendingPermissionRequest struct {
	Title   string
	Options []PermissionOption
}

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

	// CurrentTicket is the identifier of the ticket currently being processed,
	// or empty if the worker is idle. Protected by mu.
	CurrentTicket string

	// LastOutput holds the last three lines of agent output, used for display
	// in the worker status view. Protected by mu.
	LastOutput []string

	// pendingPermission holds the in-flight permission request waiting for a
	// user response, or nil if none is pending. Protected by mu.
	pendingPermission *PendingPermissionRequest

	// cancelWork cancels the per-task context, aborting the current subprocess.
	// nil when the worker is idle. Protected by mu.
	cancelWork context.CancelFunc

	// mu guards CurrentTicket, LastOutput, pendingPermission, and cancelWork
	// for concurrent access between the worker goroutine (writer) and the UI
	// goroutine (reader).
	mu sync.RWMutex

	// database, logCh, notifCh, ticketsDir, and workFn are set by Pool.Start
	// before the goroutine is launched and remain constant for the worker's lifetime.
	database   *db.DB
	logCh      chan<- LogMessage
	notifCh    chan<- string // sends notification text to the TUI
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
		LastOutput: []string{},
	}
}

// GetCurrentTicket returns the identifier of the ticket being processed, or
// empty if idle. Safe for concurrent access from the UI goroutine.
func (w *Worker) GetCurrentTicket() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.CurrentTicket
}

// SetCurrentTicket updates the identifier of the ticket being processed.
func (w *Worker) SetCurrentTicket(identifier string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.CurrentTicket = identifier
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

// GetPendingPermission returns a copy of the pending permission request, or
// nil if none is currently in flight. Safe for concurrent access.
func (w *Worker) GetPendingPermission() *PendingPermissionRequest {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.pendingPermission
}

// SetPendingPermission replaces the pending permission request under the write
// lock. Pass nil to clear it after the user responds.
func (w *Worker) SetPendingPermission(req *PendingPermissionRequest) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pendingPermission = req
}

// AbortWork cancels the worker's current subprocess, if any. This is called
// by housekeeping when a stale ticket is being reclaimed. The worker's main
// loop will handle the context cancellation and clean up.
func (w *Worker) AbortWork() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancelWork != nil {
		w.cancelWork()
	}
}

// setCancel stores (or clears, if nil) the cancel function for the current
// per-task context.
func (w *Worker) setCancel(cancel context.CancelFunc) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.cancelWork = cancel
}

// SendResponse sends a response message to the worker and marks it as busy.
// This is the correct way for the UI layer to respond to a worker's question
// or permission request.
func (w *Worker) SendResponse(text string) {
	w.ToWorker <- MainToWorkerMessage{Kind: MsgResponse, Payload: text}
	w.Status = StatusBusy
}
