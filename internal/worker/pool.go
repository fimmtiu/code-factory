package worker

import (
	"context"
	"sync"

	"github.com/fimmtiu/tickets/internal/db"
)

// logChannelBuffer is the buffer size for the shared log channel. It is kept
// generous to avoid blocking workers during bursts of log activity.
const logChannelBuffer = 100

// WorkFn is the signature for the function that does the actual work on a
// claimed ticket. The default is runACP; set to MockWorkFn for UI testing
// without a running Claude process.
type WorkFn func(ctx context.Context, w *Worker, database dbInterface, logCh chan<- LogMessage, worktreePath, identifier, phase, prompt, logfilePath string) error

// Pool holds the collection of workers and the shared log channel. It is the
// single point of management for the main goroutine.
type Pool struct {
	Workers      []*Worker
	LogChannel   chan LogMessage
	PoolSize     int
	PollInterval int // seconds

	// WorkFn is called by each worker to do the actual work on a claimed ticket.
	// Defaults to runACP; set to MockWorkFn for UI testing.
	WorkFn WorkFn

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPool creates a Pool with size workers numbered 1 through size, and a
// buffered log channel. pollInterval is the number of seconds between polls.
func NewPool(size int, pollInterval int) *Pool {
	workers := make([]*Worker, size)
	for i := 0; i < size; i++ {
		workers[i] = NewWorker(i + 1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		Workers:      workers,
		LogChannel:   make(chan LogMessage, logChannelBuffer),
		PoolSize:     size,
		PollInterval: pollInterval,
		WorkFn:       runACP,
		ctx:          ctx,
		cancel:       cancel,
	}
}

// GetWorker returns the worker with the given 1-based number, or nil if the
// number is out of range.
func (p *Pool) GetWorker(number int) *Worker {
	if number < 1 || number > len(p.Workers) {
		return nil
	}
	return p.Workers[number-1]
}

// Start launches one goroutine per worker. Each goroutine runs the worker's
// main loop. The pool's shared context is used for shutdown signaling.
func (p *Pool) Start(database *db.DB, ticketsDir string) {
	for _, w := range p.Workers {
		w.database = database
		w.logCh = p.LogChannel
		w.ticketsDir = ticketsDir
		w.workFn = p.WorkFn // nil = real ACP
		p.wg.Add(1)
		go func(w *Worker) {
			defer p.wg.Done()
			w.run(p.ctx, p.PollInterval)
		}(w)
	}
}

// Stop signals all worker and housekeeping goroutines to shut down and waits
// for them to exit.
func (p *Pool) Stop() {
	p.cancel()
	p.wg.Wait()
}

// StartHousekeeping launches the background goroutine that releases stale
// tickets. It shares the pool's context so Stop() also terminates it.
func (p *Pool) StartHousekeeping(database *db.DB) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		runHousekeeping(p.ctx, database, p.LogChannel)
	}()
}

// PauseWorker sends a MsgPause message to the worker with the given 1-based number.
func (p *Pool) PauseWorker(number int) {
	if w := p.GetWorker(number); w != nil {
		w.ToWorker <- MainToWorkerMessage{Kind: MsgPause}
	}
}

// UnpauseWorker sends a MsgUnpause message to the worker with the given 1-based number.
func (p *Pool) UnpauseWorker(number int) {
	if w := p.GetWorker(number); w != nil {
		w.ToWorker <- MainToWorkerMessage{Kind: MsgUnpause}
	}
}
