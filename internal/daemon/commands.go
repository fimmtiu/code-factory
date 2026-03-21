package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
	"github.com/fimmtiu/tickets/internal/storage"
)

// RegisterCommands registers the ping, exit, status, create-project, and
// create-ticket command handlers on the given worker.
func RegisterCommands(w *Worker, d *Daemon) {
	w.RegisterHandler("ping", handlePingCommand)
	w.RegisterHandler("exit", makeExitHandler(w))
	w.RegisterHandler("status", makeStatusHandler(d))
	w.RegisterHandler("create-project", makeCreateProjectHandler(d))
	w.RegisterHandler("create-ticket", makeCreateTicketHandler(d))
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

// makeCreateProjectHandler returns a handler that creates a new project
// directory and .project.json file, then updates the in-memory state.
func makeCreateProjectHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		description := cmd.Params["description"]

		if err := models.ValidateIdentifier(identifier); err != nil {
			return protocol.Response{Success: false, Error: err.Error()}
		}

		// For subprojects, check that the parent project exists.
		if idx := strings.LastIndex(identifier, "/"); idx >= 0 {
			parentID := identifier[:idx]
			if _, ok := d.state.Get(parentID); !ok {
				return protocol.Response{
					Success: false,
					Error:   fmt.Sprintf("parent project %q not found", parentID),
				}
			}
		}

		if err := storage.CreateProjectDir(d.ticketsDir, identifier); err != nil {
			return protocol.Response{Success: false, Error: err.Error()}
		}

		wu := models.NewProject(identifier, description)
		wu.Status = models.ProjectOpen

		if err := d.state.Add(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to add project to state: " + err.Error()}
		}

		return protocol.Response{Success: true}
	}
}

// makeCreateTicketHandler returns a handler that creates a new ticket JSON
// file and updates the in-memory state.
func makeCreateTicketHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		description := cmd.Params["description"]
		depsRaw := cmd.Params["dependencies"]

		if err := models.ValidateIdentifier(identifier); err != nil {
			return protocol.Response{Success: false, Error: err.Error()}
		}

		// Parse comma-separated dependencies, filtering empty strings.
		var deps []string
		for _, dep := range strings.Split(depsRaw, ",") {
			dep = strings.TrimSpace(dep)
			if dep != "" {
				deps = append(deps, dep)
			}
		}

		// For tickets under a project, check that the parent project exists.
		if idx := strings.LastIndex(identifier, "/"); idx >= 0 {
			parentID := identifier[:idx]
			if _, ok := d.state.Get(parentID); !ok {
				return protocol.Response{
					Success: false,
					Error:   fmt.Sprintf("parent project %q not found", parentID),
				}
			}
		}

		initialStatus := models.StatusOpen
		if len(deps) > 0 {
			initialStatus = models.StatusBlocked
		}

		wu := models.NewTicket(identifier, description)
		wu.Dependencies = deps
		wu.Status = initialStatus

		if err := d.state.Add(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to add ticket to state: " + err.Error()}
		}

		return protocol.Response{Success: true}
	}
}
