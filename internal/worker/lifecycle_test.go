package worker_test

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// --- Test DB helpers ---

func openTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	d.SetGitClient(&gitutil.FakeGitClient{})
	t.Cleanup(func() { d.Close() })
	return d, dir
}

func createProject(t *testing.T, d *db.DB, id string) {
	t.Helper()
	if err := d.CreateProject(id, "test project", nil, ""); err != nil {
		t.Fatalf("CreateProject %q: %v", id, err)
	}
}

func createTicket(t *testing.T, d *db.DB, id string) {
	t.Helper()
	if err := d.CreateTicket(id, "test ticket", nil, ""); err != nil {
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
	pool.WorkFn = worker.NoopWorkFn
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
	pool.WorkFn = worker.NoopWorkFn
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

func TestFindInProgressTickets_ReturnsInProgressTickets(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-a")
	createTicket(t, d, "proj/ticket-b")

	// Claim both and transition to in-progress (as a real worker would).
	ta, err := d.Claim(1)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := d.SetStatus(ta.Identifier, ta.Phase, models.StatusWorking); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	tb, err := d.Claim(2)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := d.SetStatus(tb.Identifier, tb.Phase, models.StatusWorking); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	tickets, err := d.FindInProgressTickets()
	if err != nil {
		t.Fatalf("FindInProgressTickets: %v", err)
	}
	if len(tickets) != 2 {
		t.Errorf("expected 2 in-progress tickets, got %d", len(tickets))
	}
}

func TestFindInProgressTickets_ExcludesIdleTickets(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/idle-ticket")
	// idle-ticket is not claimed, status = idle — NOT in-progress.

	tickets, err := d.FindInProgressTickets()
	if err != nil {
		t.Fatalf("FindInProgressTickets: %v", err)
	}
	if len(tickets) != 0 {
		t.Errorf("expected 0 in-progress tickets for idle ticket, got %d", len(tickets))
	}
}

func TestIsLogfileStale_MissingFile(t *testing.T) {
	if !worker.IsLogfileStale("/nonexistent/path", time.Now()) {
		t.Error("expected missing logfile to be considered stale")
	}
}

func TestIsLogfileStale_EmptyPath(t *testing.T) {
	if !worker.IsLogfileStale("", time.Now()) {
		t.Error("expected empty logfile path to be considered stale")
	}
}

func TestIsLogfileStale_RecentFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "logfile")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if worker.IsLogfileStale(f.Name(), time.Now()) {
		t.Error("expected recently created logfile to NOT be considered stale")
	}
}

func TestIsLogfileStale_OldFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "logfile")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	// Set mod time to 15 minutes ago.
	old := time.Now().Add(-15 * time.Minute)
	if err := os.Chtimes(f.Name(), old, old); err != nil {
		t.Fatal(err)
	}

	if !worker.IsLogfileStale(f.Name(), time.Now()) {
		t.Error("expected logfile with 15-minute-old mod time to be considered stale")
	}
}

func TestPool_StartHousekeeping_StopsCleanly(t *testing.T) {
	d, ticketsDir := openTestDB(t)

	pool := worker.NewPool(1, 1)
	pool.Start(d, ticketsDir)
	pool.StartHousekeeping(d, ticketsDir)

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

// --- US-006: Worker returns to idle after work completes ---

func TestWorker_PicksUpNewWorkAfterCompletion(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/t1")
	createTicket(t, d, "proj/t2")

	pool := worker.NewPool(1, 1)
	pool.WorkFn = worker.NoopWorkFn
	pool.Start(d, ticketsDir)

	// Both tickets must be released; if the worker deadlocks after the
	// first ticket, the second is never claimed and the test times out.
	released := 0
	deadline := time.After(5 * time.Second)
	for released < 2 {
		select {
		case msg := <-pool.LogChannel:
			if len(msg.Message) > 8 && msg.Message[:8] == "released" {
				released++
			}
		case <-deadline:
			pool.Stop()
			t.Fatalf("only %d/2 tickets released within 5s — worker likely stuck in busy state after first ticket", released)
		}
	}

	pool.Stop()
}

func TestWorker_ShutdownUnblocksHangingWorkFn(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/t1")

	pool := worker.NewPool(1, 1)
	// WorkFn that blocks until the context is cancelled, simulating
	// a subprocess that never exits.
	pool.WorkFn = worker.HangingWorkFn
	pool.Start(d, ticketsDir)

	// Wait for the ticket to be claimed.
	if !waitForLog(pool.LogChannel, "claimed ticket proj/t1", 5*time.Second) {
		pool.Stop()
		t.Fatal("ticket was not claimed within 5 seconds")
	}

	// Stop the pool — this cancels the context and must unblock the
	// hanging WorkFn. If it doesn't, the test times out.
	done := make(chan struct{})
	go func() {
		pool.Stop()
		close(done)
	}()

	select {
	case <-done:
		// ok — shutdown completed
	case <-time.After(5 * time.Second):
		t.Fatal("Pool.Stop() did not return within 5s — hanging WorkFn was not unblocked by context cancellation")
	}
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

// --- Startup recovery ---

func TestResetTicket_ResetsStatusAndClaim(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/stuck")

	// Simulate a worker claiming and starting work.
	ticket, err := d.Claim(1)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := d.SetStatus(ticket.Identifier, ticket.Phase, models.StatusWorking); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	// ResetTicket should clear both status and claim.
	if err := d.ResetTicket("proj/stuck"); err != nil {
		t.Fatalf("ResetTicket: %v", err)
	}

	// The ticket should now be claimable again.
	reclaimed, err := d.Claim(2)
	if err != nil {
		t.Fatalf("Claim after reset: %v", err)
	}
	if reclaimed.Identifier != "proj/stuck" {
		t.Errorf("expected to reclaim proj/stuck, got %q", reclaimed.Identifier)
	}
}

func TestRecoverOrphanedTickets_ResetsRunningTickets(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/in-prog")
	createTicket(t, d, "proj/needs-attn")
	createTicket(t, d, "proj/user-rev")
	createTicket(t, d, "proj/idle")

	// Set up various states to simulate a hard kill mid-run.
	t1, _ := d.Claim(1)
	_ = d.SetStatus(t1.Identifier, t1.Phase, models.StatusWorking)

	t2, _ := d.Claim(2)
	_ = d.SetStatus(t2.Identifier, t2.Phase, models.StatusNeedsAttention)

	t3, _ := d.Claim(3)
	_ = d.SetStatus(t3.Identifier, t3.Phase, models.StatusUserReview)
	_ = d.Release(t3.Identifier)

	// proj/idle is never claimed — stays idle.

	// Recover should reset in-progress and needs-attention, but not user-review or idle.
	count, err := d.RecoverOrphanedTickets()
	if err != nil {
		t.Fatalf("RecoverOrphanedTickets: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 recovered tickets, got %d", count)
	}

	// All 4 tickets should now be in a valid state. The two recovered ones
	// plus the idle one should be claimable (3 total). user-review stays put.
	var claimed []string
	for i := 0; i < 3; i++ {
		wu, err := d.Claim(10 + i)
		if err != nil {
			break
		}
		claimed = append(claimed, wu.Identifier)
	}
	if len(claimed) != 3 {
		t.Errorf("expected 3 claimable tickets after recovery, got %d: %v", len(claimed), claimed)
	}
}

func TestRecoverOrphanedTickets_NothingToRecover(t *testing.T) {
	d, _ := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/healthy")

	count, err := d.RecoverOrphanedTickets()
	if err != nil {
		t.Fatalf("RecoverOrphanedTickets: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 recovered tickets, got %d", count)
	}
}

// --- WorkFn error handling ---

func TestWorker_AcpError_DoesNotAdvanceToUserReview(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/fail-ticket")

	pool := worker.NewPool(1, 1)
	pool.WorkFn = worker.ErrorWorkFn
	pool.Start(d, ticketsDir)

	// Wait for the error to be processed and the ticket released.
	if !waitForLog(pool.LogChannel, "released ticket proj/fail-ticket", 5*time.Second) {
		pool.Stop()
		t.Fatal("ticket was not released within 5 seconds")
	}
	pool.Stop()

	// The ticket must NOT have been advanced to user-review.
	units, err := d.Status()
	if err != nil {
		t.Fatalf("d.Status(): %v", err)
	}
	for _, u := range units {
		if u.Identifier == "proj/fail-ticket" {
			if u.Status == models.StatusUserReview {
				t.Errorf("ticket status = %q after ACP error; must not advance to user-review", u.Status)
			}
			return
		}
	}
	t.Fatal("ticket proj/fail-ticket not found in status output")
}

// --- WorkAvailable notification ---

func TestPool_WorkAvailable_WakesIdleWorker(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	// No tickets yet — workers will idle.

	pool := worker.NewPool(1, 60) // 60-second poll interval
	pool.WorkFn = worker.NoopWorkFn
	pool.Start(d, ticketsDir)

	// Give the worker time to enter waitForNextPoll.
	time.Sleep(100 * time.Millisecond)

	// Create a ticket and signal that work is available.
	createTicket(t, d, "proj/wake-test")
	select {
	case pool.WorkAvailable <- struct{}{}:
	default:
	}

	// The worker should claim the ticket well within the 60s poll interval.
	if !waitForLog(pool.LogChannel, "claimed ticket proj/wake-test", 3*time.Second) {
		pool.Stop()
		t.Fatal("worker did not claim ticket within 3s after WorkAvailable signal (poll interval is 60s)")
	}

	pool.Stop()
}
