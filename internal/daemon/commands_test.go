package daemon_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/protocol"
)

// MockGitClient is a test double for gitutil.GitClient that records calls
// and can be configured to return errors.
type MockGitClient struct {
	CreatedWorktrees  []string
	MergedBranches    []string // "from->into"
	RemovedWorktrees  []string
	CreateWorktreeErr error
	MergeBranchErr    error
	RemoveWorktreeErr error
}

func (m *MockGitClient) CreateWorktree(repoRoot, worktreePath, branchName string) error {
	m.CreatedWorktrees = append(m.CreatedWorktrees, branchName)
	return m.CreateWorktreeErr
}

func (m *MockGitClient) MergeBranch(repoRoot, fromBranch, intoBranch string) error {
	m.MergedBranches = append(m.MergedBranches, fromBranch+"->"+intoBranch)
	return m.MergeBranchErr
}

func (m *MockGitClient) RemoveWorktree(repoRoot, worktreePath, branchName string) error {
	m.RemovedWorktrees = append(m.RemovedWorktrees, branchName)
	return m.RemoveWorktreeErr
}

func (m *MockGitClient) GetRepoRoot(path string) (string, error) {
	return "/fake/repo", nil
}

// newTestDaemonWithMockGit creates a Daemon backed by a MockGitClient.
func newTestDaemonWithMockGit(t *testing.T, ticketsDir string) (*daemon.Daemon, *MockGitClient) {
	t.Helper()
	d := daemon.NewDaemon(tempSocketPath(t), ticketsDir)
	mock := &MockGitClient{}
	d.SetGitClient(mock)
	return d, mock
}

// newTestDaemonWithDir creates a Daemon using the given ticketsDir without
// starting its listener. Suitable for command handler tests.
func newTestDaemonWithDir(t *testing.T, ticketsDir string) *daemon.Daemon {
	t.Helper()
	return daemon.NewDaemon(tempSocketPath(t), ticketsDir)
}

// TestCommandPing verifies that the ping command returns a success response
// containing the current process PID.
func TestCommandPing(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)

	var stopped bool
	stopFn := func() { stopped = true }

	w := daemon.NewWorker(d, stopFn)
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "ping"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("ping: expected Success=true, got false (%q)", resp.Error)
		}
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("ping: unmarshal data: %v", err)
		}
		pid, ok := data["pid"]
		if !ok {
			t.Fatal("ping: expected 'pid' in response data")
		}
		if pid.(float64) != float64(os.Getpid()) {
			t.Errorf("ping: expected pid=%d, got %v", os.Getpid(), pid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ping: timed out waiting for response")
	}

	if stopped {
		t.Error("ping should not call stopFn")
	}
}

// TestCommandExit verifies that the exit command returns success and triggers
// the stop function asynchronously.
func TestCommandExit(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)

	stopCh := make(chan struct{})
	stopFn := func() { close(stopCh) }

	w := daemon.NewWorker(d, stopFn)
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "exit"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("exit: expected Success=true, got false (%q)", resp.Error)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("exit: timed out waiting for response")
	}

	// stopFn must be called (asynchronously) after the response is sent.
	select {
	case <-stopCh:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("exit: timed out waiting for stopFn to be called")
	}
}

// TestCommandStatus verifies that the status command returns a JSON array of
// all work units.
func TestCommandStatus(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu1 := models.NewTicket("ticket-one", "first ticket")
	wu2 := models.NewTicket("ticket-two", "second ticket")
	writeTicket(t, ticketsDir, wu1)
	writeTicket(t, ticketsDir, wu2)

	d := newTestDaemonWithDir(t, ticketsDir)
	// Load state.
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}

	w := daemon.NewWorker(d, func() {})
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "status"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("status: expected Success=true, got false (%q)", resp.Error)
		}
		var units []map[string]interface{}
		if err := json.Unmarshal(resp.Data, &units); err != nil {
			t.Fatalf("status: unmarshal data: %v", err)
		}
		if len(units) != 2 {
			t.Errorf("status: expected 2 units, got %d", len(units))
		}
		// Verify each unit has an identifier field.
		for _, u := range units {
			if _, ok := u["identifier"]; !ok {
				t.Error("status: expected 'identifier' field in unit")
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("status: timed out waiting for response")
	}
}

// TestCommandStatusParentField verifies that nested tickets have the parent field set.
func TestCommandStatusParentField(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a project with one nested ticket.
	projDir := makeTempProjectDir(t, ticketsDir, "my-proj")
	_ = projDir

	d := newTestDaemonWithDir(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}

	w := daemon.NewWorker(d, func() {})
	daemon.RegisterCommands(w, d)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      protocol.Command{Name: "status"},
		Response: respCh,
	}

	select {
	case resp := <-respCh:
		if !resp.Success {
			t.Fatalf("status: expected Success=true")
		}
		var units []map[string]interface{}
		if err := json.Unmarshal(resp.Data, &units); err != nil {
			t.Fatalf("status: unmarshal: %v", err)
		}
		// Find the nested ticket and verify it has parent set.
		for _, u := range units {
			id, _ := u["identifier"].(string)
			if id == "my-proj/child-ticket" {
				parent, _ := u["parent"].(string)
				if parent != "my-proj" {
					t.Errorf("expected parent='my-proj', got %q", parent)
				}
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("status: timed out")
	}
}

// makeTempProjectDir creates a project directory with one child ticket.
func makeTempProjectDir(t *testing.T, ticketsDir, projectID string) string {
	t.Helper()
	projDir := ticketsDir + "/" + projectID
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	proj := models.NewProject(projectID, "test project")
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}

	child := models.NewTicket(projectID+"/child-ticket", "child")
	writeTicketToDir(t, projDir, "child-ticket", child)

	return projDir
}

func writeProjectFile(t *testing.T, projDir string, wu *models.WorkUnit) error {
	t.Helper()
	path := projDir + "/project.json"
	return writeWorkUnitToPath(t, path, wu)
}

func writeTicketToDir(t *testing.T, dir, name string, wu *models.WorkUnit) {
	t.Helper()
	ticketDir := dir + "/" + name
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("writeTicketToDir MkdirAll: %v", err)
	}
	path := ticketDir + "/ticket.json"
	if err := writeWorkUnitToPath(t, path, wu); err != nil {
		t.Fatalf("writeTicketToDir: %v", err)
	}
}

func writeWorkUnitToPath(t *testing.T, path string, wu *models.WorkUnit) error {
	t.Helper()
	data, err := json.MarshalIndent(wu, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// sendCommand is a helper that sends a command to the worker via the daemon
// queue and waits for a response with a timeout.
func sendCommand(t *testing.T, d *daemon.Daemon, cmd protocol.Command) protocol.Response {
	t.Helper()
	respCh := make(chan protocol.Response, 1)
	d.Queue() <- &daemon.QueueItem{
		Cmd:      cmd,
		Response: respCh,
	}
	select {
	case resp := <-respCh:
		return resp
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for response")
		return protocol.Response{}
	}
}

// startWorker creates a worker, registers commands, and starts it. Returns
// a cancel function to stop the worker.
func startWorker(t *testing.T, d *daemon.Daemon) context.CancelFunc {
	t.Helper()
	w := daemon.NewWorker(d, func() {})
	daemon.RegisterCommands(w, d)
	ctx, cancel := context.WithCancel(context.Background())
	go w.Run(ctx)
	return cancel
}

// TestCreateProject_Success verifies that a top-level project can be created
// and is present in state afterwards.
func TestCreateProject_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  "my-project",
			"description": "a test project",
		},
	})
	if !resp.Success {
		t.Fatalf("create-project: expected success, got error: %q", resp.Error)
	}

	wu, ok := d.State().Get("my-project")
	if !ok {
		t.Fatal("create-project: expected project in state after create")
	}
	if wu.Description != "a test project" {
		t.Errorf("create-project: expected description 'a test project', got %q", wu.Description)
	}
	if !wu.IsProject {
		t.Error("create-project: expected IsProject=true")
	}
}

// TestCreateProject_Subproject verifies that a subproject can be created under
// an existing parent project.
func TestCreateProject_Subproject(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	// Create parent first.
	resp := sendCommand(t, d, protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  "parent-proj",
			"description": "parent project",
		},
	})
	if !resp.Success {
		t.Fatalf("create-project parent: expected success, got %q", resp.Error)
	}

	// Create subproject under it.
	resp = sendCommand(t, d, protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  "parent-proj/child-proj",
			"description": "child project",
		},
	})
	if !resp.Success {
		t.Fatalf("create-project subproject: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("parent-proj/child-proj")
	if !ok {
		t.Fatal("create-project subproject: expected subproject in state")
	}
	if wu.Description != "child project" {
		t.Errorf("create-project subproject: expected description 'child project', got %q", wu.Description)
	}
	if wu.Parent != "parent-proj" {
		t.Errorf("create-project subproject: expected Parent 'parent-proj', got %q", wu.Parent)
	}
}

// TestCreateProject_MissingParent verifies that creating a subproject when the
// parent does not exist returns an error response.
func TestCreateProject_MissingParent(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  "nonexistent-parent/child-proj",
			"description": "orphan child",
		},
	})
	if resp.Success {
		t.Fatal("create-project missing parent: expected failure, got success")
	}
	if resp.Error == "" {
		t.Error("create-project missing parent: expected non-empty error message")
	}
}

// TestCreateTicket_Success verifies that a ticket can be created under an
// existing project and appears in state with open status.
func TestCreateTicket_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	// Create project first.
	resp := sendCommand(t, d, protocol.Command{
		Name: "create-project",
		Params: map[string]string{
			"identifier":  "my-proj",
			"description": "a project",
		},
	})
	if !resp.Success {
		t.Fatalf("create-project: expected success, got %q", resp.Error)
	}

	// Create ticket under it.
	resp = sendCommand(t, d, protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":  "my-proj/my-ticket",
			"description": "a ticket",
		},
	})
	if !resp.Success {
		t.Fatalf("create-ticket: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-proj/my-ticket")
	if !ok {
		t.Fatal("create-ticket: expected ticket in state after create")
	}
	if wu.Status != models.StatusOpen {
		t.Errorf("create-ticket: expected status open, got %q", wu.Status)
	}
	if wu.Description != "a ticket" {
		t.Errorf("create-ticket: expected description 'a ticket', got %q", wu.Description)
	}
	if wu.Parent != "my-proj" {
		t.Errorf("create-ticket: expected Parent 'my-proj', got %q", wu.Parent)
	}
}

// TestCreateTicket_BlockedByDeps verifies that a ticket created with
// dependencies gets the "blocked" status.
func TestCreateTicket_BlockedByDeps(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	// Create a top-level dep ticket first.
	resp := sendCommand(t, d, protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":  "dep-ticket",
			"description": "a dependency",
		},
	})
	if !resp.Success {
		t.Fatalf("create dep ticket: expected success, got %q", resp.Error)
	}

	// Create ticket that depends on dep-ticket.
	resp = sendCommand(t, d, protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":   "blocked-ticket",
			"description":  "blocked by dep",
			"dependencies": "dep-ticket",
		},
	})
	if !resp.Success {
		t.Fatalf("create-ticket blocked: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("blocked-ticket")
	if !ok {
		t.Fatal("create-ticket blocked: expected ticket in state")
	}
	if wu.Status != models.StatusBlocked {
		t.Errorf("create-ticket blocked: expected status blocked, got %q", wu.Status)
	}
	if len(wu.Dependencies) != 1 || wu.Dependencies[0] != "dep-ticket" {
		t.Errorf("create-ticket blocked: expected dependencies=[dep-ticket], got %v", wu.Dependencies)
	}
}

// TestCreateTicket_TopLevel verifies that a top-level ticket (no parent
// project) can be created successfully.
func TestCreateTicket_TopLevel(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":  "top-level-ticket",
			"description": "no parent needed",
		},
	})
	if !resp.Success {
		t.Fatalf("create-ticket top-level: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("top-level-ticket")
	if !ok {
		t.Fatal("create-ticket top-level: expected ticket in state")
	}
	if wu.Status != models.StatusOpen {
		t.Errorf("create-ticket top-level: expected status open, got %q", wu.Status)
	}
}

// TestCreateTicket_MissingParent verifies that creating a ticket under a
// nonexistent project returns an error response.
func TestCreateTicket_MissingParent(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	d := newTestDaemonWithDir(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name: "create-ticket",
		Params: map[string]string{
			"identifier":  "ghost-proj/my-ticket",
			"description": "orphan ticket",
		},
	})
	if resp.Success {
		t.Fatal("create-ticket missing parent: expected failure, got success")
	}
	if resp.Error == "" {
		t.Error("create-ticket missing parent: expected non-empty error message")
	}
}

// --------------------------------------------------------------------------
// get-work tests
// --------------------------------------------------------------------------

// TestGetWork_Success verifies that get-work returns an open ticket, creates
// a worktree, marks the ticket in-progress, and cascades in-progress to the
// parent project.
func TestGetWork_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a project with one open ticket.
	projDir := ticketsDir + "/my-proj"
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("my-proj", "parent project")
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}
	ticket := models.NewTicket("my-proj/work-ticket", "do some work")
	writeTicketToDir(t, projDir, "work-ticket", ticket)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "get-work"})
	if !resp.Success {
		t.Fatalf("get-work: expected success, got error: %q", resp.Error)
	}

	// Response should be the ticket JSON.
	var wu models.WorkUnit
	if err := json.Unmarshal(resp.Data, &wu); err != nil {
		t.Fatalf("get-work: unmarshal data: %v", err)
	}
	if wu.Identifier != "my-proj/work-ticket" {
		t.Errorf("get-work: expected identifier 'my-proj/work-ticket', got %q", wu.Identifier)
	}

	// Ticket should be in-progress.
	ticketState, ok := d.State().Get("my-proj/work-ticket")
	if !ok {
		t.Fatal("get-work: ticket not found in state")
	}
	if ticketState.Status != models.StatusInProgress {
		t.Errorf("get-work: expected ticket status in-progress, got %q", ticketState.Status)
	}

	// Parent project should be in-progress.
	projState, ok := d.State().Get("my-proj")
	if !ok {
		t.Fatal("get-work: project not found in state")
	}
	if projState.Status != models.ProjectInProgress {
		t.Errorf("get-work: expected project status in-progress, got %q", projState.Status)
	}

	// Worktree should have been created.
	if len(mock.CreatedWorktrees) != 1 || mock.CreatedWorktrees[0] != "my-proj/work-ticket" {
		t.Errorf("get-work: expected CreateWorktree called with 'my-proj/work-ticket', got %v", mock.CreatedWorktrees)
	}
}

// TestGetWork_NoWork verifies that get-work returns failure when no open
// tickets are available.
func TestGetWork_NoWork(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "get-work"})
	if resp.Success {
		t.Fatal("get-work no-work: expected failure, got success")
	}
	if resp.Error == "" {
		t.Error("get-work no-work: expected non-empty error message")
	}
}

// TestGetWork_ParentCascade verifies that get-work cascades in-progress up
// through nested parent projects.
func TestGetWork_ParentCascade(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create grandparent > parent > ticket hierarchy.
	grandDir := ticketsDir + "/grand"
	parentDir := grandDir + "/child-proj"
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	grand := models.NewProject("grand", "grandparent")
	if err := writeProjectFile(t, grandDir, grand); err != nil {
		t.Fatalf("writeProjectFile grand: %v", err)
	}

	parent := models.NewProject("grand/child-proj", "parent")
	if err := writeProjectFile(t, parentDir, parent); err != nil {
		t.Fatalf("writeProjectFile parent: %v", err)
	}

	ticket := models.NewTicket("grand/child-proj/leaf-ticket", "leaf")
	writeTicketToDir(t, parentDir, "leaf-ticket", ticket)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "get-work"})
	if !resp.Success {
		t.Fatalf("get-work cascade: expected success, got %q", resp.Error)
	}

	// Both ancestor projects should be in-progress.
	for _, id := range []string{"grand", "grand/child-proj"} {
		wu, ok := d.State().Get(id)
		if !ok {
			t.Fatalf("get-work cascade: %q not found in state", id)
		}
		if wu.Status != models.ProjectInProgress {
			t.Errorf("get-work cascade: expected %q status in-progress, got %q", id, wu.Status)
		}
	}

	if len(mock.CreatedWorktrees) != 1 {
		t.Errorf("get-work cascade: expected 1 worktree created, got %d", len(mock.CreatedWorktrees))
	}
}

// --------------------------------------------------------------------------
// review-ready tests
// --------------------------------------------------------------------------

// TestReviewReady_Success verifies that review-ready marks a ticket as
// review-ready.
func TestReviewReady_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("fix-bug", "fix a bug")
	ticket.Status = models.StatusInProgress
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "review-ready",
		Params: map[string]string{"identifier": "fix-bug"},
	})
	if !resp.Success {
		t.Fatalf("review-ready: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("fix-bug")
	if !ok {
		t.Fatal("review-ready: ticket not found")
	}
	if wu.Status != models.StatusReviewReady {
		t.Errorf("review-ready: expected status review-ready, got %q", wu.Status)
	}
}

// TestReviewReady_NotFound verifies that review-ready returns an error when
// the identifier does not exist.
func TestReviewReady_NotFound(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "review-ready",
		Params: map[string]string{"identifier": "nonexistent"},
	})
	if resp.Success {
		t.Fatal("review-ready not-found: expected failure, got success")
	}
}

// --------------------------------------------------------------------------
// get-review tests
// --------------------------------------------------------------------------

// TestGetReview_Success verifies that get-review returns a review-ready ticket
// and marks it in-review.
func TestGetReview_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("review-me", "needs review")
	ticket.Status = models.StatusReviewReady
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "get-review"})
	if !resp.Success {
		t.Fatalf("get-review: expected success, got %q", resp.Error)
	}

	var wu models.WorkUnit
	if err := json.Unmarshal(resp.Data, &wu); err != nil {
		t.Fatalf("get-review: unmarshal: %v", err)
	}
	if wu.Identifier != "review-me" {
		t.Errorf("get-review: expected 'review-me', got %q", wu.Identifier)
	}

	ticketState, ok := d.State().Get("review-me")
	if !ok {
		t.Fatal("get-review: ticket not found in state")
	}
	if ticketState.Status != models.StatusInReview {
		t.Errorf("get-review: expected status in-review, got %q", ticketState.Status)
	}
}

// TestGetReview_NoReviews verifies that get-review returns failure when no
// review-ready tickets exist.
func TestGetReview_NoReviews(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "get-review"})
	if resp.Success {
		t.Fatal("get-review no-reviews: expected failure, got success")
	}
}

// --------------------------------------------------------------------------
// done tests
// --------------------------------------------------------------------------

// TestDone_Success verifies that done merges the ticket branch, marks the
// ticket done, and removes its worktree.
func TestDone_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a project with one ticket in-review.
	projDir := ticketsDir + "/my-proj"
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("my-proj", "project")
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}
	ticket := models.NewTicket("my-proj/fix-bug", "fix bug")
	ticket.Status = models.StatusInReview
	writeTicketToDir(t, projDir, "fix-bug", ticket)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "my-proj/fix-bug"},
	})
	if !resp.Success {
		t.Fatalf("done: expected success, got %q", resp.Error)
	}

	// Ticket should be marked done.
	wu, ok := d.State().Get("my-proj/fix-bug")
	if !ok {
		t.Fatal("done: ticket not found")
	}
	if wu.Status != models.StatusDone {
		t.Errorf("done: expected status done, got %q", wu.Status)
	}

	// Branch should have been merged into the parent project branch.
	if len(mock.MergedBranches) == 0 {
		t.Fatal("done: expected MergeBranch to be called")
	}
	if mock.MergedBranches[0] != "my-proj/fix-bug->my-proj" {
		t.Errorf("done: expected merge 'my-proj/fix-bug->my-proj', got %q", mock.MergedBranches[0])
	}

	// Worktree should have been removed.
	if len(mock.RemovedWorktrees) == 0 || mock.RemovedWorktrees[0] != "my-proj/fix-bug" {
		t.Errorf("done: expected RemoveWorktree('my-proj/fix-bug'), got %v", mock.RemovedWorktrees)
	}
}

// TestDone_NotFound verifies that done returns error for unknown identifier.
func TestDone_NotFound(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "nonexistent"},
	})
	if resp.Success {
		t.Fatal("done not-found: expected failure, got success")
	}
}

// TestDone_MergeFailure verifies that done returns an error when the git
// merge fails.
func TestDone_MergeFailure(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("my-ticket", "some work")
	ticket.Status = models.StatusInReview
	writeTicket(t, ticketsDir, ticket)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	mock.MergeBranchErr = errors.New("merge conflict")
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "my-ticket"},
	})
	if resp.Success {
		t.Fatal("done merge-failure: expected failure, got success")
	}
	if resp.Error == "" {
		t.Error("done merge-failure: expected non-empty error")
	}
}

// TestDone_ProjectCascade verifies that when all tickets in a project are done,
// the project itself is marked done and its branch is merged.
func TestDone_ProjectCascade(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a top-level project with two tickets; one already done, one in-review.
	projDir := ticketsDir + "/my-proj"
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("my-proj", "project")
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}

	ticketA := models.NewTicket("my-proj/ticket-a", "already done")
	ticketA.Status = models.StatusDone
	writeTicketToDir(t, projDir, "ticket-a", ticketA)

	ticketB := models.NewTicket("my-proj/ticket-b", "last ticket")
	ticketB.Status = models.StatusInReview
	writeTicketToDir(t, projDir, "ticket-b", ticketB)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "my-proj/ticket-b"},
	})
	if !resp.Success {
		t.Fatalf("done cascade: expected success, got %q", resp.Error)
	}

	// Project should be marked done.
	projState, ok := d.State().Get("my-proj")
	if !ok {
		t.Fatal("done cascade: project not found in state")
	}
	if projState.Status != models.ProjectDone {
		t.Errorf("done cascade: expected project done, got %q", projState.Status)
	}

	// Project branch should be merged into main (top-level project → main).
	foundProjectMerge := false
	for _, m := range mock.MergedBranches {
		if m == "my-proj->main" {
			foundProjectMerge = true
		}
	}
	if !foundProjectMerge {
		t.Errorf("done cascade: expected merge 'my-proj->main', got %v", mock.MergedBranches)
	}
}

// TestDone_InProgress verifies that done accepts a ticket in in-progress
// status (lenient mode).
func TestDone_InProgress(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("quick-fix", "quick fix")
	ticket.Status = models.StatusInProgress
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "quick-fix"},
	})
	if !resp.Success {
		t.Fatalf("done in-progress: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("quick-fix")
	if !ok {
		t.Fatal("done in-progress: ticket not found")
	}
	if wu.Status != models.StatusDone {
		t.Errorf("done in-progress: expected done, got %q", wu.Status)
	}
}

// TestDone_InvalidStatus verifies that done rejects tickets not in in-review
// or in-progress state.
func TestDone_InvalidStatus(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("fresh-ticket", "not started")
	// StatusOpen is not acceptable for done.
	ticket.Status = models.StatusOpen
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "fresh-ticket"},
	})
	if resp.Success {
		t.Fatal("done invalid-status: expected failure, got success")
	}
	if resp.Error == "" {
		t.Error("done invalid-status: expected non-empty error message")
	}
}

// TestDone_NestedProjectCascade verifies that done cascades through multiple
// levels of project hierarchy when all siblings are complete.
func TestDone_NestedProjectCascade(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Build: grand > child-proj > leaf-ticket (only ticket, in-review)
	grandDir := ticketsDir + "/grand"
	childDir := grandDir + "/child-proj"
	if err := os.MkdirAll(childDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	grand := models.NewProject("grand", "grandparent project")
	if err := writeProjectFile(t, grandDir, grand); err != nil {
		t.Fatalf("writeProjectFile grand: %v", err)
	}

	child := models.NewProject("grand/child-proj", "child project")
	if err := writeProjectFile(t, childDir, child); err != nil {
		t.Fatalf("writeProjectFile child: %v", err)
	}

	leaf := models.NewTicket("grand/child-proj/leaf", "leaf ticket")
	leaf.Status = models.StatusInReview
	writeTicketToDir(t, childDir, "leaf", leaf)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "done",
		Params: map[string]string{"identifier": "grand/child-proj/leaf"},
	})
	if !resp.Success {
		t.Fatalf("done nested cascade: expected success, got %q", resp.Error)
	}

	// Both projects should be done.
	for _, id := range []string{"grand/child-proj", "grand"} {
		wu, ok := d.State().Get(id)
		if !ok {
			t.Fatalf("done nested cascade: %q not found in state", id)
		}
		if wu.Status != models.ProjectDone {
			t.Errorf("done nested cascade: expected %q done, got %q", id, wu.Status)
		}
	}

	// grand/child-proj should have been merged into grand, and grand into main.
	foundChildMerge := false
	foundGrandMerge := false
	for _, m := range mock.MergedBranches {
		if m == "grand/child-proj->grand" {
			foundChildMerge = true
		}
		if m == "grand->main" {
			foundGrandMerge = true
		}
	}
	if !foundChildMerge {
		t.Errorf("done nested cascade: expected 'grand/child-proj->grand', got %v", mock.MergedBranches)
	}
	if !foundGrandMerge {
		t.Errorf("done nested cascade: expected 'grand->main', got %v", mock.MergedBranches)
	}
}
