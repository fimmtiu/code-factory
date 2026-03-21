package daemon

import (
	"encoding/json"
	"os"

	"github.com/fimmtiu/tickets/internal/protocol"
)

// RegisterCommands registers the ping, exit, and status command handlers on
// the given worker.
func RegisterCommands(w *Worker, d *Daemon) {
	w.RegisterHandler("ping", handlePingCommand)
	w.RegisterHandler("exit", makeExitHandler(w))
	w.RegisterHandler("status", makeStatusHandler(d))
}

// handlePingCommand returns a success response containing the current process PID.
func handlePingCommand(cmd protocol.Command) protocol.Response {
	data, err := json.Marshal(map[string]int{"pid": os.Getpid()})
	if err != nil {
		return protocol.Response{Success: false, Error: "internal error marshaling ping response"}
	}
	return protocol.Response{
		Success: true,
		Data:    json.RawMessage(data),
	}
}

// makeExitHandler returns a handler that calls the worker's stopFn
// asynchronously and immediately returns success.
func makeExitHandler(w *Worker) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		go w.stopFn()
		return protocol.Response{Success: true}
	}
}

// makeStatusHandler returns a handler that returns all work units as a JSON
// array with parent fields set.
func makeStatusHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		units := d.state.All()
		data, err := json.Marshal(units)
		if err != nil {
			return protocol.Response{
				Success: false,
				Error:   "failed to marshal status: " + err.Error(),
			}
		}
		return protocol.Response{
			Success: true,
			Data:    json.RawMessage(data),
		}
	}
}
