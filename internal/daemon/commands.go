package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fimmtiu/tickets/internal/gitutil"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
	"github.com/fimmtiu/tickets/internal/storage"
)

// RegisterCommands registers the ping, exit, status, create-project,
// create-ticket, get-work, review-ready, get-review, and done command
// handlers on the given worker.
func RegisterCommands(w *Worker, d *Daemon) {
	w.RegisterHandler("ping", handlePingCommand)
	w.RegisterHandler("exit", makeExitHandler(w))
	w.RegisterHandler("status", makeStatusHandler(d))
	w.RegisterHandler("create-project", makeCreateProjectHandler(d))
	w.RegisterHandler("create-ticket", makeCreateTicketHandler(d))
	w.RegisterHandler("get-work", makeGetWorkHandler(d))
	w.RegisterHandler("review-ready", makeReviewReadyHandler(d))
	w.RegisterHandler("get-review", makeGetReviewHandler(d))
	w.RegisterHandler("done", makeDoneHandler(d))
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

// makeGetWorkHandler returns a handler that finds an open ticket with all
// dependencies satisfied, creates a git worktree for it, marks it
// in-progress, cascades in-progress to all ancestor projects, and returns
// the ticket JSON.
func makeGetWorkHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		ticket := d.state.FindOpen()
		if ticket == nil {
			return protocol.Response{Success: false, Error: "no work available"}
		}

		repoRoot := filepath.Dir(d.ticketsDir)
		if err := d.gitClient.CreateWorktree(repoRoot, ticket.Identifier); err != nil {
			return protocol.Response{Success: false, Error: "failed to create worktree: " + err.Error()}
		}

		ticket.Status = models.StatusInProgress
		if err := d.state.Update(ticket); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		markAncestorsInProgress(d.state, ticket)

		data, err := json.Marshal(ticket)
		if err != nil {
			return protocol.Response{Success: false, Error: "failed to marshal ticket: " + err.Error()}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(data)}
	}
}

// markAncestorsInProgress walks up the parent chain from ticket and marks
// every ancestor project as in-progress if not already.
func markAncestorsInProgress(state *State, ticket *models.WorkUnit) {
	parent, ok := state.Parent(ticket)
	for ok {
		if parent.Status != models.ProjectInProgress {
			parent.Status = models.ProjectInProgress
			state.Update(parent) //nolint:errcheck
		}
		parent, ok = state.Parent(parent)
	}
}

// makeReviewReadyHandler returns a handler that marks the identified ticket
// as review-ready.
func makeReviewReadyHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]

		wu, ok := d.state.Get(identifier)
		if !ok {
			return protocol.Response{Success: false, Error: fmt.Sprintf("ticket %q not found", identifier)}
		}

		wu.Status = models.StatusReviewReady
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		return protocol.Response{Success: true}
	}
}

// makeGetReviewHandler returns a handler that finds a review-ready ticket,
// marks it in-review, and returns the ticket JSON.
func makeGetReviewHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		ticket := d.state.FindReviewReady()
		if ticket == nil {
			return protocol.Response{Success: false, Error: "no reviews available"}
		}

		ticket.Status = models.StatusInReview
		if err := d.state.Update(ticket); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		data, err := json.Marshal(ticket)
		if err != nil {
			return protocol.Response{Success: false, Error: "failed to marshal ticket: " + err.Error()}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(data)}
	}
}

// makeDoneHandler returns a handler that merges the ticket's branch into its
// parent's branch (or main for top-level), marks the ticket done, removes
// its worktree, and cascades done to ancestor projects when all siblings
// are done.
func makeDoneHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]

		wu, ok := d.state.Get(identifier)
		if !ok {
			return protocol.Response{Success: false, Error: fmt.Sprintf("ticket %q not found", identifier)}
		}

		// Accept in-review or in-progress (lenient).
		if wu.Status != models.StatusInReview && wu.Status != models.StatusInProgress {
			return protocol.Response{
				Success: false,
				Error:   fmt.Sprintf("ticket %q is not in-review or in-progress (status: %s)", identifier, wu.Status),
			}
		}

		repoRoot := filepath.Dir(d.ticketsDir)

		// Determine the target branch for the merge.
		parent, hasParent := d.state.Parent(wu)
		intoBranch := "main"
		if hasParent {
			intoBranch = parent.Identifier
		}

		if err := d.gitClient.MergeBranch(repoRoot, wu.Identifier, intoBranch); err != nil {
			return protocol.Response{Success: false, Error: "merge failed: " + err.Error()}
		}

		wu.Status = models.StatusDone
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		d.gitClient.RemoveWorktree(repoRoot, wu.Identifier) //nolint:errcheck

		// Cascade done up the project hierarchy.
		if hasParent {
			if err := cascadeDone(d.state, d.gitClient, repoRoot, wu); err != nil {
				return protocol.Response{Success: false, Error: "cascade done failed: " + err.Error()}
			}
		}

		return protocol.Response{Success: true}
	}
}

// cascadeDone checks whether the parent of wu has all children done. If so,
// marks the parent done, merges its branch into its own parent (or main),
// removes its worktree, and recurses upward.
func cascadeDone(state *State, git gitutil.GitClient, repoRoot string, wu *models.WorkUnit) error {
	parent, hasParent := state.Parent(wu)
	if !hasParent {
		return nil
	}
	if !state.AllDone(parent.Identifier) {
		return nil
	}

	parent.Status = models.ProjectDone
	if err := state.Update(parent); err != nil {
		return err
	}

	grandparent, hasGrandparent := state.Parent(parent)
	intoBranch := "main"
	if hasGrandparent {
		intoBranch = grandparent.Identifier
	}

	if err := git.MergeBranch(repoRoot, parent.Identifier, intoBranch); err != nil {
		return err
	}

	git.RemoveWorktree(repoRoot, parent.Identifier) //nolint:errcheck

	if hasGrandparent {
		return cascadeDone(state, git, repoRoot, parent)
	}
	return nil
}
