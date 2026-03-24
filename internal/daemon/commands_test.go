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
	GetHeadCommitErr  error
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

func (m *MockGitClient) GetHeadCommit(path string) (string, error) {
	return "abc1234", m.GetHeadCommitErr
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
	if wu.Phase != models.PhasePlan {
		t.Errorf("create-ticket: expected phase plan, got %q", wu.Phase)
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
	if wu.Phase != models.PhaseBlocked {
		t.Errorf("create-ticket blocked: expected status blocked, got %q", wu.Phase)
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
	if wu.Phase != models.PhasePlan {
		t.Errorf("create-ticket top-level: expected status open, got %q", wu.Phase)
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
// set-status tests
// --------------------------------------------------------------------------

// TestSetStatus_Simple verifies that set-status updates a ticket's status.
func TestSetStatus_Simple(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("my-ticket", "do work")
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-ticket", "phase": "review"},
	})
	if !resp.Success {
		t.Fatalf("set-status: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-ticket")
	if !ok {
		t.Fatal("set-status: ticket not found")
	}
	if wu.Phase != models.PhaseReview {
		t.Errorf("set-status: expected review phase, got %q", wu.Phase)
	}
}

// TestSetStatus_InProgress_CreatesWorktree verifies that set-status to
// in-progress creates a git worktree and cascades in-progress to the parent.
func TestSetStatus_InProgress_CreatesWorktree(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

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
	// Make GetHeadCommit fail so the worktree creation path is exercised.
	mock.GetHeadCommitErr = errors.New("no worktree")
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-proj/work-ticket", "phase": "implement", "status": "in-progress"},
	})
	if !resp.Success {
		t.Fatalf("set-status in-progress: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-proj/work-ticket")
	if !ok {
		t.Fatal("set-status: ticket not found in state")
	}
	if wu.Phase != models.PhaseImplement || wu.Status != models.StatusInProgress {
		t.Errorf("set-status: expected implement/in-progress, got phase=%q status=%q", wu.Phase, wu.Status)
	}


	if len(mock.CreatedWorktrees) != 1 || mock.CreatedWorktrees[0] != "my-proj/work-ticket" {
		t.Errorf("set-status: expected CreateWorktree('my-proj/work-ticket'), got %v", mock.CreatedWorktrees)
	}
}

// TestSetStatus_Done_MergesAndCascades verifies that set-status done merges
// the branch, marks the ticket done, removes the worktree, and cascades to
// the parent project when all siblings are done.
func TestSetStatus_Done_MergesAndCascades(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	projDir := ticketsDir + "/my-proj"
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("my-proj", "project")
	if err := writeProjectFile(t, projDir, proj); err != nil {
		t.Fatalf("writeProjectFile: %v", err)
	}
	ticketA := models.NewTicket("my-proj/ticket-a", "already done")
	ticketA.Phase = models.PhaseDone
	writeTicketToDir(t, projDir, "ticket-a", ticketA)

	ticketB := models.NewTicket("my-proj/ticket-b", "last ticket")
	ticketB.Phase = models.PhaseImplement
	ticketB.Status = models.StatusInProgress
	writeTicketToDir(t, projDir, "ticket-b", ticketB)

	d, mock := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-proj/ticket-b", "phase": "done"},
	})
	if !resp.Success {
		t.Fatalf("set-status done: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-proj/ticket-b")
	if !ok {
		t.Fatal("set-status done: ticket not found")
	}
	if wu.Phase != models.PhaseDone {
		t.Errorf("set-status done: expected done, got %q", wu.Phase)
	}

	if len(mock.MergedBranches) == 0 {
		t.Fatal("set-status done: expected MergeBranch to be called")
	}
	if mock.MergedBranches[0] != "my-proj/ticket-b->my-proj" {
		t.Errorf("set-status done: expected merge 'my-proj/ticket-b->my-proj', got %q", mock.MergedBranches[0])
	}

	if len(mock.RemovedWorktrees) == 0 || mock.RemovedWorktrees[0] != "my-proj/ticket-b" {
		t.Errorf("set-status done: expected RemoveWorktree('my-proj/ticket-b'), got %v", mock.RemovedWorktrees)
	}

}

// TestSetStatus_Done_ClearsClaimedBy verifies that marking a ticket done also
// clears its ClaimedBy field.
func TestSetStatus_Done_ClearsClaimedBy(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("my-ticket", "claimed work")
	ticket.Phase = models.PhaseImplement
	ticket.Status = models.StatusInProgress
	ticket.ClaimedBy = "42"
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-ticket", "phase": "done"},
	})
	if !resp.Success {
		t.Fatalf("set-status done: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-ticket")
	if !ok {
		t.Fatal("set-status done: ticket not found")
	}
	if wu.ClaimedBy != "" {
		t.Errorf("set-status done: expected ClaimedBy cleared, got %q", wu.ClaimedBy)
	}
}

// TestSetStatus_NotFound verifies that set-status returns error for an unknown
// identifier.
func TestSetStatus_NotFound(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "nonexistent", "status": "open"},
	})
	if resp.Success {
		t.Fatal("set-status not-found: expected failure, got success")
	}
}

// TestSetStatus_InvalidStatus verifies that set-status rejects unknown statuses.
func TestSetStatus_InvalidStatus(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("my-ticket", "a ticket")
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-ticket", "status": "flying"},
	})
	if resp.Success {
		t.Fatal("set-status invalid: expected failure, got success")
	}
}

// TestSetStatus_RejectsProject verifies that set-status returns an error when
// the identifier refers to a project.
func TestSetStatus_RejectsProject(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	cancel := startWorker(t, d)
	defer cancel()

	sendCommand(t, d, protocol.Command{
		Name:   "create-project",
		Params: map[string]string{"identifier": "my-proj", "description": "a project"},
	})

	resp := sendCommand(t, d, protocol.Command{
		Name:   "set-status",
		Params: map[string]string{"identifier": "my-proj", "phase": "done"},
	})
	if resp.Success {
		t.Fatal("set-status project: expected failure, got success")
	}
}

// --------------------------------------------------------------------------
// claim tests
// --------------------------------------------------------------------------

// TestClaim_Success verifies that claim assigns an available ticket to the
// given PID and returns it.
func TestClaim_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("open-ticket", "unclaimed work")
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "claim",
		Params: map[string]string{"pid": "1234"},
	})
	if !resp.Success {
		t.Fatalf("claim: expected success, got %q", resp.Error)
	}

	var wu models.WorkUnit
	if err := json.Unmarshal(resp.Data, &wu); err != nil {
		t.Fatalf("claim: unmarshal: %v", err)
	}
	if wu.Identifier != "open-ticket" {
		t.Errorf("claim: expected 'open-ticket', got %q", wu.Identifier)
	}
	if wu.ClaimedBy != "1234" {
		t.Errorf("claim: expected ClaimedBy='1234', got %q", wu.ClaimedBy)
	}

	// Verify the claim is persisted in state.
	state, ok := d.State().Get("open-ticket")
	if !ok {
		t.Fatal("claim: ticket not found in state")
	}
	if state.ClaimedBy != "1234" {
		t.Errorf("claim: state ClaimedBy expected '1234', got %q", state.ClaimedBy)
	}
}

// TestClaim_SkipsClaimedTickets verifies that claim does not return a ticket
// already claimed by another process.
func TestClaim_SkipsClaimedTickets(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	taken := models.NewTicket("taken-ticket", "already claimed")
	taken.ClaimedBy = "999"
	writeTicket(t, ticketsDir, taken)

	free := models.NewTicket("free-ticket", "unclaimed")
	writeTicket(t, ticketsDir, free)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "claim",
		Params: map[string]string{"pid": "42"},
	})
	if !resp.Success {
		t.Fatalf("claim: expected success, got %q", resp.Error)
	}

	var wu models.WorkUnit
	if err := json.Unmarshal(resp.Data, &wu); err != nil {
		t.Fatalf("claim: unmarshal: %v", err)
	}
	if wu.Identifier != "free-ticket" {
		t.Errorf("claim: expected 'free-ticket', got %q", wu.Identifier)
	}
}

// TestClaim_NoneAvailable verifies that claim returns failure when no ticket
// can be claimed.
func TestClaim_NoneAvailable(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	done := models.NewTicket("done-ticket", "finished")
	done.Phase = models.PhaseDone
	writeTicket(t, ticketsDir, done)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "claim",
		Params: map[string]string{"pid": "1"},
	})
	if resp.Success {
		t.Fatal("claim none-available: expected failure, got success")
	}
}

// TestClaim_NoPID verifies that claim requires a pid parameter.
func TestClaim_NoPID(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{Name: "claim"})
	if resp.Success {
		t.Fatal("claim no-pid: expected failure, got success")
	}
}

// --------------------------------------------------------------------------
// release tests
// --------------------------------------------------------------------------

// TestRelease_Success verifies that release clears the ClaimedBy field on a
// claimed ticket.
func TestRelease_Success(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	ticket := models.NewTicket("my-ticket", "claimed work")
	ticket.ClaimedBy = "42"
	writeTicket(t, ticketsDir, ticket)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "release",
		Params: map[string]string{"identifier": "my-ticket"},
	})
	if !resp.Success {
		t.Fatalf("release: expected success, got %q", resp.Error)
	}

	wu, ok := d.State().Get("my-ticket")
	if !ok {
		t.Fatal("release: ticket not found")
	}
	if wu.ClaimedBy != "" {
		t.Errorf("release: expected ClaimedBy cleared, got %q", wu.ClaimedBy)
	}
}

// TestRelease_NotFound verifies that release returns an error for an unknown
// identifier.
func TestRelease_NotFound(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	d, _ := newTestDaemonWithMockGit(t, ticketsDir)
	if err := d.State().Load(); err != nil {
		t.Fatalf("State.Load: %v", err)
	}
	cancel := startWorker(t, d)
	defer cancel()

	resp := sendCommand(t, d, protocol.Command{
		Name:   "release",
		Params: map[string]string{"identifier": "nonexistent"},
	})
	if resp.Success {
		t.Fatal("release not-found: expected failure, got success")
	}
}
