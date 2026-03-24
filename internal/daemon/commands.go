package daemon

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
	"github.com/fimmtiu/tickets/internal/storage"
)

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

// RegisterCommands registers the exit, status, create-project, create-ticket,
// set-status, claim, release, add-comment, and close-thread command handlers
// on the given worker. The ping handler is registered automatically by NewWorker.
func RegisterCommands(w *Worker, d *Daemon) {
	w.RegisterHandler("exit", makeExitHandler(w))
	w.RegisterHandler("status", makeStatusHandler(d))
	w.RegisterHandler("create-project", makeCreateProjectHandler(d))
	w.RegisterHandler("create-ticket", makeCreateTicketHandler(d))
	w.RegisterHandler("set-status", makeSetStatusHandler(d))
	w.RegisterHandler("claim", makeClaimHandler(d))
	w.RegisterHandler("release", makeReleaseHandler(d))
	w.RegisterHandler("add-comment", makeAddCommentHandler(d))
	w.RegisterHandler("close-thread", makeCloseThreadHandler(d))
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
// directory and project.json file, then updates the in-memory state.
func makeCreateProjectHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		description := cmd.Params["description"]

		if err := models.ValidateIdentifier(identifier); err != nil {
			return protocol.Response{Success: false, Error: err.Error()}
		}

		parentID, hasParent := parentIdentifierOf(identifier)
		if hasParent {
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
		if hasParent {
			wu.Parent = parentID
		}
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

		parentID, hasParent := parentIdentifierOf(identifier)
		if hasParent {
			if _, ok := d.state.Get(parentID); !ok {
				return protocol.Response{
					Success: false,
					Error:   fmt.Sprintf("parent project %q not found", parentID),
				}
			}
		}

		wu := models.NewTicket(identifier, description)
		wu.SetDependencies(deps)
		if hasParent {
			wu.Parent = parentID
		}

		if err := d.state.Add(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to add ticket to state: " + err.Error()}
		}

		return protocol.Response{Success: true}
	}
}

// makeSetStatusHandler returns a handler that sets a ticket's status to the
// requested value. When the new status is "done" it additionally merges the
// ticket's branch, removes its worktree, and cascades done to ancestor
// projects when all siblings are complete. When the new status is
// "in-progress" it creates a git worktree for the ticket (if one does not
// already exist) and cascades in-progress to ancestor projects.
func makeSetStatusHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		newStatus := cmd.Params["status"]

		wu, ok := d.state.Get(identifier)
		if !ok {
			return protocol.Response{Success: false, Error: fmt.Sprintf("ticket %q not found", identifier)}
		}
		if wu.IsProject {
			return protocol.Response{Success: false, Error: fmt.Sprintf("%q is a project, not a ticket", identifier)}
		}

		if !models.IsValidTicketStatus(newStatus) {
			return protocol.Response{Success: false, Error: fmt.Sprintf("invalid ticket status %q", newStatus)}
		}

		repoRoot := d.RepoRoot()

		if newStatus == models.StatusDone {
			intoBranch := wu.MergeTargetBranch()
			if err := d.gitClient.MergeBranch(repoRoot, wu.Identifier, intoBranch); err != nil {
				return protocol.Response{Success: false, Error: "merge failed: " + err.Error()}
			}
			wu.Status = models.StatusDone
			wu.ClaimedBy = ""
			if err := d.state.Update(wu); err != nil {
				return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
			}
			ticketDir := storage.TicketDirPath(d.ticketsDir, wu.Identifier)
			if err := d.gitClient.RemoveWorktree(repoRoot, storage.TicketWorktreePath(ticketDir), wu.Identifier); err != nil {
				panic(err)
			}
			if wu.Parent != "" {
				if err := d.cascadeDone(repoRoot, wu); err != nil {
					return protocol.Response{Success: false, Error: "cascade done failed: " + err.Error()}
				}
			}
			return protocol.Response{Success: true}
		}

		if newStatus == models.StatusInProgress {
			ticketDir := storage.TicketDirPath(d.ticketsDir, wu.Identifier)
			worktreePath := storage.TicketWorktreePath(ticketDir)
			// Create the worktree only if it doesn't already exist.
			if _, err := d.gitClient.GetHeadCommit(worktreePath); err != nil {
				if err := d.gitClient.CreateWorktree(repoRoot, worktreePath, wu.Identifier); err != nil {
					return protocol.Response{Success: false, Error: "failed to create worktree: " + err.Error()}
				}
			}
			wu.Status = models.StatusInProgress
			if err := d.state.Update(wu); err != nil {
				return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
			}
			d.state.MarkAncestorsInProgress(wu)
			return protocol.Response{Success: true}
		}

		wu.Status = newStatus
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}
		return protocol.Response{Success: true}
	}
}

// makeClaimHandler returns a handler that assigns the first available
// claimable ticket to the requesting process (identified by pid). A ticket
// is claimable when it is not a project, not blocked, not done, and not
// already claimed. The handler returns the claimed ticket as JSON.
func makeClaimHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		pid := cmd.Params["pid"]
		if pid == "" {
			return protocol.Response{Success: false, Error: "claim: pid is required"}
		}

		ticket := d.state.FindClaimable()
		if ticket == nil {
			return protocol.Response{Success: false, Error: "no claimable ticket available"}
		}

		ticket.ClaimedBy = pid
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

// makeReleaseHandler returns a handler that clears the claim on a ticket,
// making it available for other processes to claim.
func makeReleaseHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		if identifier == "" {
			return protocol.Response{Success: false, Error: "release: identifier is required"}
		}

		wu, ok := d.state.Get(identifier)
		if !ok {
			return protocol.Response{Success: false, Error: fmt.Sprintf("ticket %q not found", identifier)}
		}

		wu.ClaimedBy = ""
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update ticket: " + err.Error()}
		}
		return protocol.Response{Success: true}
	}
}

// makeAddCommentHandler returns a handler that appends a comment to an existing
// open comment thread at the given code location, or creates a new thread if
// none exists. The commit hash is read from the work unit's worktree HEAD.
func makeAddCommentHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		identifier := cmd.Params["identifier"]
		codeLocation := cmd.Params["code_location"]
		author := cmd.Params["author"]
		text := cmd.Params["text"]

		if identifier == "" {
			return protocol.Response{Success: false, Error: "add-comment: identifier is required"}
		}
		if codeLocation == "" {
			return protocol.Response{Success: false, Error: "add-comment: code_location is required"}
		}
		if author == "" {
			return protocol.Response{Success: false, Error: "add-comment: author is required"}
		}
		if text == "" {
			return protocol.Response{Success: false, Error: "add-comment: text is required"}
		}

		wu, ok := d.state.Get(identifier)
		if !ok {
			return protocol.Response{Success: false, Error: fmt.Sprintf("work unit %q not found", identifier)}
		}

		// Try to get HEAD commit from the worktree; fall back to empty string
		// when the worktree does not exist yet.
		ticketDir := storage.TicketDirPath(d.ticketsDir, identifier)
		worktreePath := storage.TicketWorktreePath(ticketDir)
		commitHash, _ := d.gitClient.GetHeadCommit(worktreePath)

		comment := models.Comment{
			Date:   time.Now().UTC(),
			Author: author,
			Text:   text,
		}

		// Find the first open thread for this code location.
		for i := range wu.CommentThreads {
			if wu.CommentThreads[i].CodeLocation == codeLocation &&
				wu.CommentThreads[i].Status == models.ThreadOpen {
				wu.CommentThreads[i].Comments = append(wu.CommentThreads[i].Comments, comment)
				if err := d.state.Update(wu); err != nil {
					return protocol.Response{Success: false, Error: "failed to update work unit: " + err.Error()}
				}
				return protocol.Response{Success: true}
			}
		}

		// No open thread for this location — create a new one.
		threadID, err := models.NewCommentThreadID()
		if err != nil {
			return protocol.Response{Success: false, Error: "failed to generate thread ID: " + err.Error()}
		}
		thread := models.CommentThread{
			ID:           threadID,
			CommitHash:   commitHash,
			CodeLocation: codeLocation,
			Status:       models.ThreadOpen,
			Comments:     []models.Comment{comment},
		}
		wu.CommentThreads = append(wu.CommentThreads, thread)
		if err := d.state.Update(wu); err != nil {
			return protocol.Response{Success: false, Error: "failed to update work unit: " + err.Error()}
		}
		return protocol.Response{Success: true}
	}
}

// makeCloseThreadHandler returns a handler that finds the comment thread with
// the given ID (across all work units) and sets its status to closed.
func makeCloseThreadHandler(d *Daemon) HandlerFunc {
	return func(cmd protocol.Command) protocol.Response {
		threadID := cmd.Params["thread_id"]
		if threadID == "" {
			return protocol.Response{Success: false, Error: "close-thread: thread_id is required"}
		}

		for _, wu := range d.state.All() {
			for i := range wu.CommentThreads {
				if wu.CommentThreads[i].ID == threadID {
					wu.CommentThreads[i].Status = models.ThreadClosed
					if err := d.state.Update(wu); err != nil {
						return protocol.Response{Success: false, Error: "failed to update work unit: " + err.Error()}
					}
					return protocol.Response{Success: true}
				}
			}
		}

		return protocol.Response{Success: false, Error: fmt.Sprintf("comment thread %q not found", threadID)}
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

	intoBranch := parent.MergeTargetBranch()

	if err := d.gitClient.MergeBranch(repoRoot, parent.Identifier, intoBranch); err != nil {
		return err
	}

	projectDir := storage.TicketDirPath(d.ticketsDir, parent.Identifier)
	worktreePath := storage.TicketWorktreePath(projectDir)
	if err := d.gitClient.RemoveWorktree(repoRoot, worktreePath, parent.Identifier); err != nil {
		panic(err)
	}

	if parent.Parent != "" {
		return d.cascadeDone(repoRoot, parent)
	}
	return nil
}
