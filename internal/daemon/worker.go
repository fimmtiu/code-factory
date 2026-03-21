package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fimmtiu/tickets/internal/protocol"
)

// HandlerFunc processes a command and returns a response.
type HandlerFunc func(cmd protocol.Command) protocol.Response

// housekeepingCommands is the set of command names that do not update the
// last-non-housekeeping-command timestamp.
var housekeepingCommands = map[string]bool{
	"ping": true,
}

// Worker consumes commands from the Daemon's queue one at a time, dispatches
// each command to the appropriate HandlerFunc, and sends the response back on
// the QueueItem's response channel.
type Worker struct {
	daemon   *Daemon
	mu       sync.RWMutex
	lastCmd  time.Time
	handlers map[string]HandlerFunc
}

// NewWorker creates a Worker attached to the given Daemon and registers the
// built-in command handlers.
func NewWorker(d *Daemon) *Worker {
	w := &Worker{
		daemon:   d,
		handlers: make(map[string]HandlerFunc),
	}
	w.registerBuiltins()
	return w
}

// RegisterHandler adds or replaces the handler for the named command.
func (w *Worker) RegisterHandler(name string, fn HandlerFunc) {
	w.handlers[name] = fn
}

// LastNonHousekeepingCmd returns the time of the last command that is not
// considered a housekeeping command (e.g. ping). The zero value is returned
// if no such command has been processed yet.
func (w *Worker) LastNonHousekeepingCmd() time.Time {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastCmd
}

// Run consumes items from the daemon's queue until the context is cancelled
// or the queue channel is closed. Each item is dispatched synchronously.
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-w.daemon.queue:
			if !ok {
				return
			}
			resp := w.dispatch(item.Cmd)
			if !housekeepingCommands[item.Cmd.Name] {
				w.mu.Lock()
				w.lastCmd = time.Now()
				w.mu.Unlock()
			}
			item.Response <- resp
		}
	}
}

// dispatch looks up and calls the handler for cmd. If no handler is
// registered, it returns a failure response.
func (w *Worker) dispatch(cmd protocol.Command) protocol.Response {
	fn, ok := w.handlers[cmd.Name]
	if !ok {
		return protocol.Response{
			Success: false,
			Error:   fmt.Sprintf("unknown command: %q", cmd.Name),
		}
	}
	return fn(cmd)
}

// registerBuiltins registers the built-in command handlers.
func (w *Worker) registerBuiltins() {
	w.handlers["ping"] = w.handlePing
}

// handlePing is the built-in ping handler. It returns a success response
// containing the current process's PID.
func (w *Worker) handlePing(cmd protocol.Command) protocol.Response {
	data, err := json.Marshal(map[string]int{"pid": os.Getpid()})
	if err != nil {
		return protocol.Response{Success: false, Error: "internal error marshaling ping response"}
	}
	return protocol.Response{
		Success: true,
		Data:    json.RawMessage(data),
	}
}
