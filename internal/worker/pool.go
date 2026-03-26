package worker

// logChannelBuffer is the buffer size for the shared log channel. It is kept
// generous to avoid blocking workers during bursts of log activity.
const logChannelBuffer = 100

// Pool holds the collection of workers and the shared log channel. It is the
// single point of management for the main goroutine.
type Pool struct {
	Workers      []*Worker
	LogChannel   chan LogMessage
	PoolSize     int
	PollInterval int // seconds
}

// NewPool creates a Pool with size workers numbered 1 through size, and a
// buffered log channel. pollInterval is the number of seconds between polls.
func NewPool(size int, pollInterval int) *Pool {
	workers := make([]*Worker, size)
	for i := 0; i < size; i++ {
		workers[i] = NewWorker(i + 1)
	}
	return &Pool{
		Workers:      workers,
		LogChannel:   make(chan LogMessage, logChannelBuffer),
		PoolSize:     size,
		PollInterval: pollInterval,
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
