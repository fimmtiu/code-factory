package worker_test

import (
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// --- Test DB helpers ---

type fakeGitClient struct{}

func (f *fakeGitClient) CreateWorktree(_, worktreePath, _ string) error { return nil }
func (f *fakeGitClient) MergeBranch(_, _ string) error                  { return nil }
func (f *fakeGitClient) RemoveWorktree(_, _, _ string) error            { return nil }
func (f *fakeGitClient) GetHeadCommit(_ string) (string, error)         { return "", nil }

var _ gitutil.GitClient = (*fakeGitClient)(nil)

func openTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	d.SetGitClient(&fakeGitClient{})
	t.Cleanup(func() { d.Close() })
	return d, dir
}

func createProject(t *testing.T, d *db.DB, id string) {
	t.Helper()
	if err := d.CreateProject(id, "test project", nil); err != nil {
		t.Fatalf("CreateProject %q: %v", id, err)
	}
}

func createTicket(t *testing.T, d *db.DB, id string) {
	t.Helper()
	if err := d.CreateTicket(id, "test ticket", nil); err != nil {
		t.Fatalf("CreateTicket %q: %v", id, err)
	}
}

// drainLogs collects log messages from the channel for up to the given
// duration and returns all messages received.
func drainLogs(ch <-chan worker.LogMessage, dur time.Duration) []worker.LogMessage {
	var msgs []worker.LogMessage
	deadline := time.After(dur)
	for {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		case <-deadline:
			return msgs
		}
	}
}

// waitForLog blocks until a log message whose Message field equals want appears
// on ch, or until timeout elapses. Returns true if found.
func waitForLog(ch <-chan worker.LogMessage, want string, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			if msg.Message == want {
				return true
			}
		case <-deadline:
			return false
		}
	}
}

// --- US-001: Start / Stop ---

func TestPool_StartStop(t *testing.T) {
	d, ticketsDir := openTestDB(t)

	pool := worker.NewPool(3, 1)
	pool.Start(d, ticketsDir)

	// Give goroutines a moment to start.
	time.Sleep(20 * time.Millisecond)

	// Stop must return without hanging.
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("Pool.Stop() did not return within 3 seconds")
	}
}

func TestPool_StartStop_NoTickets(t *testing.T) {
	// Stop works even when there are no tickets to process.
	d, ticketsDir := openTestDB(t)

	pool := worker.NewPool(5, 1)
	pool.Start(d, ticketsDir)

	time.Sleep(10 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Pool.Stop() hung with no tickets")
	}
}

// --- US-002: Worker main loop ---

func TestWorker_ClaimProcessRelease(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)

	if !waitForLog(pool.LogChannel, "released ticket proj/ticket-1", 5*time.Second) {
		pool.Stop()
		t.Fatal("ticket was not released within 5 seconds")
	}

	pool.Stop()
}

func TestWorker_LogsClaimAndCompletion(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)

	msgs := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for !(msgs["claimed"] && msgs["completed"] && msgs["released"]) {
		select {
		case msg := <-pool.LogChannel:
			switch msg.Message {
			case "claimed ticket proj/ticket-1":
				msgs["claimed"] = true
			case "completed processing ticket proj/ticket-1":
				msgs["completed"] = true
			case "released ticket proj/ticket-1":
				msgs["released"] = true
			}
		case <-deadline:
			pool.Stop()
			t.Fatalf("did not see expected log messages; got: %v", msgs)
		}
	}

	pool.Stop()
}

func TestWorker_MultipleTickets(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/t1")
	createTicket(t, d, "proj/t2")

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)

	// Both tickets should be released.
	released := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for len(released) < 2 {
		select {
		case msg := <-pool.LogChannel:
			if msg.Message == "released ticket proj/t1" {
				released["t1"] = true
			}
			if msg.Message == "released ticket proj/t2" {
				released["t2"] = true
			}
		case <-deadline:
			pool.Stop()
			t.Fatalf("only %d/2 tickets were released within 5s: %v", len(released), released)
		}
	}

	pool.Stop()
}

// --- US-003: Pause / Unpause ---

func TestPool_PauseWorker_DoesNotClaimNewTickets(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/t1")
	createTicket(t, d, "proj/t2")

	// Pre-pause the worker before starting so it never claims.
	pool := worker.NewPool(1, 1)
	pool.PauseWorker(1)
	pool.Start(d, ticketsDir)

	// Collect all log messages for 500 ms — no claim messages must appear.
	logs := drainLogs(pool.LogChannel, 500*time.Millisecond)
	pool.Stop()

	for _, msg := range logs {
		if msg.Message == "claimed ticket proj/t1" || msg.Message == "claimed ticket proj/t2" {
			t.Errorf("paused worker unexpectedly claimed a ticket: %q", msg.Message)
		}
	}
}

func TestPool_UnpauseWorker_ResumesWork(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	pool := worker.NewPool(1, 1)
	pool.PauseWorker(1)
	pool.Start(d, ticketsDir)

	// Ensure no claim during pause window.
	time.Sleep(300 * time.Millisecond)

	select {
	case msg := <-pool.LogChannel:
		if msg.Message == "claimed ticket proj/ticket-1" {
			pool.Stop()
			t.Fatal("paused worker claimed a ticket before unpause")
		}
	default:
	}

	// Unpause and wait for a claim.
	pool.UnpauseWorker(1)

	if !waitForLog(pool.LogChannel, "claimed ticket proj/ticket-1", 5*time.Second) {
		pool.Stop()
		t.Fatal("worker did not claim ticket after unpause")
	}

	pool.Stop()
}

func TestPool_PauseUnpause_ViaMessages(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	// No tickets initially — worker will idle.

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)

	// Pause the worker.
	pool.PauseWorker(1)
	time.Sleep(20 * time.Millisecond)
	if w := pool.GetWorker(1); w == nil {
		t.Fatal("GetWorker(1) returned nil")
	}

	// Unpause it.
	pool.UnpauseWorker(1)
	time.Sleep(20 * time.Millisecond)

	pool.Stop()
}

// --- US-004: Message handling during idle ---

func TestWorker_ProcessesMessagesWhileIdle(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	// No tickets — worker stays in the sleep/poll loop.

	pool := worker.NewPool(1, 60) // long poll interval
	pool.Start(d, ticketsDir)

	// Give the worker a moment to enter the poll sleep.
	time.Sleep(50 * time.Millisecond)

	// Send pause and then unpause — these must be processed promptly even
	// with a 60-second poll interval, because the select responds to channel
	// messages immediately.
	pool.PauseWorker(1)
	time.Sleep(20 * time.Millisecond)

	pool.UnpauseWorker(1)
	time.Sleep(20 * time.Millisecond)

	// Stop must complete within a few seconds, proving the worker woke up
	// for the ctx.Done() signal rather than waiting 60 seconds.
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()
	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("Pool.Stop() did not return promptly with a 60-second poll interval")
	}
}

// --- US-005: Housekeeping ---

func TestFindStaleTickets_WithThreshold(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/stale")
	createTicket(t, d, "proj/fresh")

	// Claim both so they are in-progress.
	if _, err := d.Claim(1); err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if _, err := d.Claim(1); err != nil {
		t.Fatalf("Claim: %v", err)
	}

	// Freshly claimed tickets are NOT stale with a 10-minute threshold.
	stale10, err := d.FindStaleTickets(10)
	if err != nil {
		t.Fatalf("FindStaleTickets(10): %v", err)
	}
	if len(stale10) != 0 {
		t.Errorf("expected 0 stale tickets with 10-min threshold, got %d", len(stale10))
	}
}

func TestFindStaleTickets_FindsOldTickets(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/old-ticket")

	// Claim the ticket and then set it to in-progress (as a real worker would).
	ticket, err := d.Claim(99)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := d.SetStatus(ticket.Identifier, string(ticket.Phase), "in-progress"); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	// With threshold=-1 (cutoff = now + 1 minute), the freshly-claimed ticket
	// should appear as stale because its last_updated < cutoff.
	stale, err := d.FindStaleTickets(-1)
	if err != nil {
		t.Fatalf("FindStaleTickets(-1): %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale ticket with threshold=-1, got %d", len(stale))
	}
	if stale[0].Identifier != "proj/old-ticket" {
		t.Errorf("expected stale ticket 'proj/old-ticket', got %q", stale[0].Identifier)
	}
}

func TestFindStaleTickets_OnlyInProgress(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/idle-ticket")
	// idle-ticket is not claimed, status = idle — NOT in-progress.

	// FindStaleTickets should not return idle tickets even with a very wide threshold.
	stale, err := d.FindStaleTickets(-1)
	if err != nil {
		t.Fatalf("FindStaleTickets: %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("expected 0 stale tickets for idle ticket, got %d", len(stale))
	}
}

func TestPool_StartHousekeeping_StopsCleanly(t *testing.T) {
	d, ticketsDir := openTestDB(t)

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)
	pool.StartHousekeeping(d)

	// Stop must complete within a few seconds even with housekeeping running.
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("Pool.Stop() did not return within 3 seconds with housekeeping running")
	}
}

// --- US-003: PauseWorker / UnpauseWorker on pool ---

func TestPool_PauseUnpauseWorker_OutOfRange(t *testing.T) {
	// Calling Pause/Unpause with out-of-range numbers must not panic.
	pool := worker.NewPool(2, 1)
	pool.PauseWorker(0)
	pool.PauseWorker(3)
	pool.UnpauseWorker(-1)
	pool.UnpauseWorker(99)
}

// --- concurrent workers test ---

func TestPool_MultipleWorkers_ClaimIndependently(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	// Create 4 tickets for 2 workers.
	for i := 1; i <= 4; i++ {
		createTicket(t, d, filepath.Join("proj", "ticket-"+string(rune('0'+i))))
	}

	pool := worker.NewPool(2, 1)
	pool.Start(d, ticketsDir)

	// Wait until all 4 tickets are released.
	released := new(atomic.Int32)
	deadline := time.After(10 * time.Second)
	for released.Load() < 4 {
		select {
		case msg := <-pool.LogChannel:
			if len(msg.Message) > 8 && msg.Message[:8] == "released" {
				released.Add(1)
			}
		case <-deadline:
			pool.Stop()
			t.Fatalf("only %d/4 tickets released within 10s", released.Load())
		}
	}

	pool.Stop()
}
