package daemon

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
	"github.com/fimmtiu/tickets/internal/storage"
)

// mergeTargetBranch returns the branch that wu should be merged into: the
// parent's identifier when one exists, or "main" for top-level work units.
func mergeTargetBranch(parentIdentifier string, hasParent bool) string {
	if hasParent {
		return parentIdentifier
	}
	return "main"
}

// parseDependencies splits a comma-separated dependency string into a slice,
// discarding empty entries.
func parseDependencies(raw string) []string {
	var deps []string
	for _, dep := range strings.Split(raw, ",") {
		dep = strings.TrimSpace(dep)
		if dep != "" {
			deps = append(deps, dep)
		}
	}
	return deps
}

// parentIdentifierOf returns the parent identifier portion of a slash-separated
// identifier (e.g. "proj/sub/ticket" → "proj/sub"), and whether one exists.
func parentIdentifierOf(identifier string) (string, bool) {
	idx := strings.LastIndex(identifier, "/")
	if idx < 0 {
		return "", false
	}
	return identifier[:idx], true
}

// RegisterCommands registers the exit, status, create-project,
// create-ticket, get-work, review-ready, get-review, and done command
// handlers on the given worker. The ping handler is registered automatically
// by NewWorker.
func RegisterCommands(w *Worker, d *Daemon) {
	w.RegisterHandler("exit", makeExitHandler(w))
	w.RegisterHandler("status", makeStatusHandler(d))
	w.RegisterHandler("create-project", makeCreateProjectHandler(d))
	w.RegisterHandler("create-ticket", makeCreateTicketHandler(d))
	w.RegisterHandler("get-work", makeGetWorkHandler(d))
	w.RegisterHandler("review-ready", makeReviewReadyHandler(d))
	w.RegisterHandler("get-review", makeGetReviewHandler(d))
	w.RegisterHandler("done", makeDoneHandler(d))
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

		if parentID, ok := parentIdentifierOf(identifier); ok {
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

		deps := parseDependencies(depsRaw)

		if parentID, ok := parentIdentifierOf(identifier); ok {
			if _, ok := d.state.Get(parentID); !ok {
				return protocol.Response{
					Success: false,
					Error:   fmt.Sprintf("parent project %q not found", parentID),
				}
			}
		}

		wu := models.NewTicket(identifier, description)
		wu.SetDependencies(deps)

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

		repoRoot := d.RepoRoot()
		ticketDir := storage.TicketDirPath(d.ticketsDir, ticket.Identifier)
		worktreePath := storage.TicketWorktreePath(ticketDir)
		if err := d.gitClient.CreateWorktree(repoRoot, worktreePath, ticket.Identifier); err != nil {
			return protocol.Response{Success: false, Error: "failed to create worktree: " + err.Error()}
		}

		ticket.Status = models.StatusInProgress
		if err := d.state.Update(ticket); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		d.state.MarkAncestorsInProgress(ticket)

		data, err := json.Marshal(ticket)
		if err != nil {
			return protocol.Response{Success: false, Error: "failed to marshal ticket: " + err.Error()}
		}
		return protocol.Response{Success: true, Data: json.RawMessage(data)}
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
		if !wu.IsCompletable() {
			return protocol.Response{
				Success: false,
				Error:   fmt.Sprintf("ticket %q is not in-review or in-progress (status: %s)", identifier, wu.Status),
			}
		}

		repoRoot := d.RepoRoot()

		parent, hasParent := d.state.Parent(wu)
		var parentID string
		if hasParent {
			parentID = parent.Identifier
		}
		intoBranch := mergeTargetBranch(parentID, hasParent)

		if err := d.gitClient.MergeBranch(repoRoot, wu.Identifier, intoBranch); err != nil {
			return protocol.Response{Success: false, Error: "merge failed: " + err.Error()}
		}

		wu.Status = models.StatusDone
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}

		ticketDir := storage.TicketDirPath(d.ticketsDir, wu.Identifier)
		worktreePath := storage.TicketWorktreePath(ticketDir)
		d.gitClient.RemoveWorktree(repoRoot, worktreePath, wu.Identifier) //nolint:errcheck

		if hasParent {
			if err := d.cascadeDone(repoRoot, wu); err != nil {
				return protocol.Response{Success: false, Error: "cascade done failed: " + err.Error()}
			}
		}

		return protocol.Response{Success: true}
	}
}

// cascadeDone checks whether the parent of wu has all children done. If so,
// marks the parent done, merges its branch into its own parent (or main),
// removes its worktree, and recurses upward.
func (d *Daemon) cascadeDone(repoRoot string, wu *models.WorkUnit) error {
	parent, hasParent := d.state.Parent(wu)
	if !hasParent {
		return nil
	}
	if !d.state.AllDone(parent.Identifier) {
		return nil
	}

	parent.Status = models.ProjectDone
	if err := d.state.Update(parent); err != nil {
		return err
	}

	grandparent, hasGrandparent := d.state.Parent(parent)
	var grandparentID string
	if hasGrandparent {
		grandparentID = grandparent.Identifier
	}
	intoBranch := mergeTargetBranch(grandparentID, hasGrandparent)

	if err := d.gitClient.MergeBranch(repoRoot, parent.Identifier, intoBranch); err != nil {
		return err
	}

	projectDir := storage.TicketDirPath(d.ticketsDir, parent.Identifier)
	worktreePath := storage.TicketWorktreePath(projectDir)
	d.gitClient.RemoveWorktree(repoRoot, worktreePath, parent.Identifier) //nolint:errcheck

	if hasGrandparent {
		return d.cascadeDone(repoRoot, parent)
	}
	return nil
}
