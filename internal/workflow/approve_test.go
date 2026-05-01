package workflow_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/workflow"
)

// openTestDB creates a temporary in-memory-like DB for testing.
func openTestDB(t *testing.T) *db.DB {
	d, _ := openTestDBWithGit(t)
	return d
}

// openTestDBWithGit returns the DB and the FakeGitClient driving it, so tests
// can manipulate the fake's behaviour (e.g. inject rebase failures).
func openTestDBWithGit(t *testing.T) (*db.DB, *gitutil.FakeGitClient) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	git := &gitutil.FakeGitClient{}
	d.SetGitClient(git)
	t.Cleanup(func() { d.Close() })
	return d, git
}

// ticketPhase returns the current phase of a ticket, failing the test if not found.
func ticketPhase(t *testing.T, d *db.DB, identifier string) models.TicketPhase {
	t.Helper()
	phase, err := d.GetTicketPhase(identifier)
	if err != nil {
		t.Fatalf("GetTicketPhase(%q): %v", identifier, err)
	}
	return phase
}

// ticketStatus returns the current status of a ticket, failing the test if not found.
func ticketStatus(t *testing.T, d *db.DB, identifier string) models.TicketStatus {
	t.Helper()
	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, u := range units {
		if u.Identifier == identifier {
			return u.Status
		}
	}
	t.Fatalf("ticket %q not found", identifier)
	return ""
}

// projectPhase returns the current phase of a project by scanning Status().
func projectPhase(t *testing.T, d *db.DB, identifier string) models.TicketPhase {
	t.Helper()
	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, u := range units {
		if u.Identifier == identifier && u.IsProject {
			return u.Phase
		}
	}
	t.Fatalf("project %q not found", identifier)
	return ""
}

// ── Phase transition tests ────────────────────────────────────────────────────

func TestApprove_ImplementToRefactor(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseImplement, models.StatusUserReview); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseRefactor {
		t.Errorf("expected phase %q, got %q", models.PhaseRefactor, got)
	}
}

func TestApprove_RefactorToReview(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseRefactor, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseReview {
		t.Errorf("expected phase %q, got %q", models.PhaseReview, got)
	}
}

func TestApprove_ReviewToMerging(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	// Approving review hands the ticket off to a worker via merging/idle;
	// the actual rebase + done transition is done by the worker calling
	// MergeChain. The tests below exercise MergeChain directly to drive
	// the cascade since there is no real worker pool here.
	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseMerging {
		t.Errorf("expected phase %q, got %q", models.PhaseMerging, got)
	}
	if got := ticketStatus(t, d, "proj/t1"); got != models.StatusIdle {
		t.Errorf("expected status %q, got %q", models.StatusIdle, got)
	}

	if err := d.MarkTicketDoneCascading("proj/t1"); err != nil {
		t.Fatalf("MarkTicketDoneCascading: %v", err)
	}
	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseDone {
		t.Errorf("expected phase %q after merge, got %q", models.PhaseDone, got)
	}
}

// ── Open-change-request tests ────────────────────────────────────────────────

func TestApprove_OpenCRsAtImplementSendsToResponding(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseImplement, models.StatusUserReview); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/t1", "file.go:10", "reviewer", "please fix"); err != nil {
		t.Fatalf("AddChangeRequest: %v", err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseImplement {
		t.Errorf("phase should be unchanged at %q, got %q", models.PhaseImplement, got)
	}
	if got := ticketStatus(t, d, "proj/t1"); got != models.StatusResponding {
		t.Errorf("status should be %q, got %q", models.StatusResponding, got)
	}
}

func TestApprove_OpenCRsAtReviewSendsToResponding(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusUserReview); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/t1", "file.go:42", "reviewer", "please fix"); err != nil {
		t.Fatalf("AddChangeRequest: %v", err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseReview {
		t.Errorf("phase should remain %q, got %q", models.PhaseReview, got)
	}
	if got := ticketStatus(t, d, "proj/t1"); got != models.StatusResponding {
		t.Errorf("status should be %q, got %q", models.StatusResponding, got)
	}
}

func TestApprove_DismissedCRsDoNotBlockAdvancement(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseImplement, models.StatusUserReview); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/t1", "file.go:10", "reviewer", "please fix"); err != nil {
		t.Fatalf("AddChangeRequest: %v", err)
	}
	crs, err := d.OpenChangeRequests("proj/t1")
	if err != nil || len(crs) != 1 {
		t.Fatalf("expected 1 open CR, got %d, err %v", len(crs), err)
	}
	id, _ := strconvAtoi(t, crs[0].ID)
	if err := d.DismissChangeRequest(id); err != nil {
		t.Fatalf("DismissChangeRequest: %v", err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseRefactor {
		t.Errorf("expected phase %q, got %q", models.PhaseRefactor, got)
	}
}

// ── Error case tests ──────────────────────────────────────────────────────────

func TestApprove_BlockedReturnsError(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	// Create two tickets; t1 depends on t2 so it starts blocked.
	if err := d.CreateTicket("proj/t2", "dep", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "blocker", []string{"proj/t2"}, ""); err != nil {
		t.Fatal(err)
	}

	err := workflow.Approve(d, "proj/t1")
	if err == nil {
		t.Error("expected error approving a blocked ticket, got nil")
	}
}

func TestApprove_DoneReturnsError(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}
	// Approve hands off to merging; drive the merge to done as a worker would.
	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("first Approve: %v", err)
	}
	if err := d.MarkTicketDoneCascading("proj/t1"); err != nil {
		t.Fatalf("MarkTicketDoneCascading: %v", err)
	}

	// Try to approve again.
	err := workflow.Approve(d, "proj/t1")
	if err == nil {
		t.Error("expected error approving a done ticket, got nil")
	}
}

func TestApprove_NotFoundReturnsError(t *testing.T) {
	d := openTestDB(t)
	err := workflow.Approve(d, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ticket, got nil")
	}
}

// ── Recursive project completion tests ───────────────────────────────────────

func TestApprove_SingleTicketMarkesProjectDone(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := d.MarkTicketDoneCascading("proj/t1"); err != nil {
		t.Fatalf("MarkTicketDoneCascading: %v", err)
	}

	if got := projectPhase(t, d, "proj"); got != models.ProjectPhaseDone {
		t.Errorf("expected project phase %q, got %q", models.ProjectPhaseDone, got)
	}
}

func TestApprove_NotAllTicketsDone_ProjectRemainsOpen(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "ticket 1", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "ticket 2", nil, ""); err != nil {
		t.Fatal(err)
	}
	// Only approve t1 (review → done).
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// t2 is still implement/idle, so project stays open.
	if got := projectPhase(t, d, "proj"); got != models.ProjectPhaseOpen {
		t.Errorf("expected project phase %q, got %q", models.ProjectPhaseOpen, got)
	}
}

func TestApprove_NestedProjectCompletion(t *testing.T) {
	// Structure: grandparent / parent / t1.
	d := openTestDB(t)
	for _, id := range []string{"grandparent", "grandparent/parent"} {
		if err := d.CreateProject(id, "A project", nil, ""); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}
	if err := d.CreateTicket("grandparent/parent/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("grandparent/parent/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "grandparent/parent/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := d.MarkTicketDoneCascading("grandparent/parent/t1"); err != nil {
		t.Fatalf("MarkTicketDoneCascading: %v", err)
	}

	if got := projectPhase(t, d, "grandparent/parent"); got != models.ProjectPhaseDone {
		t.Errorf("parent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
	if got := projectPhase(t, d, "grandparent"); got != models.ProjectPhaseDone {
		t.Errorf("grandparent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
}

func TestApprove_TopLevelTicketNoParent(t *testing.T) {
	// Approve implement → refactor (non-done transition, no parent walk).
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseImplement, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve implement: %v", err)
	}
	if got := projectPhase(t, d, "proj"); got != models.ProjectPhaseOpen {
		t.Errorf("project should still be open, got %q", got)
	}
}

// strconvAtoi is a tiny helper to keep test call sites terse when converting
// the string-formatted CR IDs returned by OpenChangeRequests back to int64.
func strconvAtoi(t *testing.T, s string) (int64, error) {
	t.Helper()
	var n int64
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		t.Fatalf("parse id %q: %v", s, err)
	}
	return n, nil
}

// ── Cascading-failure tests ──────────────────────────────────────────────────

// TestMergeChain_GrandparentRebaseFailureLeavesNoneDone verifies the fix
// for the cascading-completion bug: when the ticket-level rebase succeeds
// but a parent rebase further up the tree fails, neither the ticket nor
// any project may be marked done. With the merging phase the ticket sits
// at merging/idle until the user (or agent) fixes the conflict and the
// cascade is retried.
func TestMergeChain_GrandparentRebaseFailureLeavesNoneDone(t *testing.T) {
	d, git := openTestDBWithGit(t)

	// grandparent / parent / t1 — completing t1 would cascade both projects.
	for _, id := range []string{"grandparent", "grandparent/parent"} {
		if err := d.CreateProject(id, "A project", nil, ""); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}
	if err := d.CreateTicket("grandparent/parent/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("grandparent/parent/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	// Fail only the second rebase in the cascade (parent → grandparent),
	// letting the ticket's own rebase succeed.
	conflictErr := errors.New("simulated parent->grandparent conflict")
	calls := 0
	git.RebaseErrFunc = func(_, _ string) error {
		calls++
		if calls == 2 {
			return conflictErr
		}
		return nil
	}

	// Approve hands off to merging.
	if err := workflow.Approve(d, "grandparent/parent/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	// MergeChain (simulating the worker run) should fail at the parent step.
	if err := d.MarkTicketDoneCascading("grandparent/parent/t1"); err == nil {
		t.Fatal("expected MergeChain to fail when parent rebase conflicts")
	}

	// Ticket must remain at merging/idle (no further transition by the
	// chain itself; the worker is what would set it to user-review).
	if got := ticketPhase(t, d, "grandparent/parent/t1"); got != models.PhaseMerging {
		t.Errorf("ticket phase: expected %q, got %q", models.PhaseMerging, got)
	}
	if got := projectPhase(t, d, "grandparent/parent"); got == models.ProjectPhaseDone {
		t.Errorf("parent project should not be done after partial failure")
	}
	if got := projectPhase(t, d, "grandparent"); got == models.ProjectPhaseDone {
		t.Errorf("grandparent project should not be done after partial failure")
	}
}

// TestMergeChain_RetryAfterParentConflictSucceeds verifies that, once the
// conflict that caused the cascade to fail is gone, re-running the chain
// completes everything — i.e. earlier rebases are safely retried without
// breaking anything.
func TestMergeChain_RetryAfterParentConflictSucceeds(t *testing.T) {
	d, git := openTestDBWithGit(t)

	for _, id := range []string{"grandparent", "grandparent/parent"} {
		if err := d.CreateProject(id, "A project", nil, ""); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}
	if err := d.CreateTicket("grandparent/parent/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("grandparent/parent/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	conflictErr := errors.New("simulated parent->grandparent conflict")
	calls := 0
	git.RebaseErrFunc = func(_, _ string) error {
		calls++
		if calls == 2 {
			return conflictErr
		}
		return nil
	}
	if err := workflow.Approve(d, "grandparent/parent/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := d.MarkTicketDoneCascading("grandparent/parent/t1"); err == nil {
		t.Fatal("setup: expected first MergeChain to fail")
	}

	// User "fixes" the conflict — clear the failure injection.
	git.RebaseErrFunc = nil

	if err := d.MarkTicketDoneCascading("grandparent/parent/t1"); err != nil {
		t.Fatalf("retry MergeChain: %v", err)
	}

	if got := ticketPhase(t, d, "grandparent/parent/t1"); got != models.PhaseDone {
		t.Errorf("ticket phase: expected %q, got %q", models.PhaseDone, got)
	}
	if got := projectPhase(t, d, "grandparent/parent"); got != models.ProjectPhaseDone {
		t.Errorf("parent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
	if got := projectPhase(t, d, "grandparent"); got != models.ProjectPhaseDone {
		t.Errorf("grandparent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
}

// TestMergeChain_TicketRebaseFailureLeavesTicketAtMerging verifies the
// simpler case: when the ticket's own rebase onto its parent fails,
// MergeChain returns an error and no work unit is finalized.
func TestMergeChain_TicketRebaseFailureLeavesTicketAtMerging(t *testing.T) {
	d, git := openTestDBWithGit(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	git.RebaseErr = errors.New("simulated ticket conflict")

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := d.MarkTicketDoneCascading("proj/t1"); err == nil {
		t.Fatal("expected MergeChain to fail when ticket rebase conflicts")
	}
	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseMerging {
		t.Errorf("ticket phase: expected %q, got %q", models.PhaseMerging, got)
	}
	if got := projectPhase(t, d, "proj"); got == models.ProjectPhaseDone {
		t.Errorf("project should not be done after ticket rebase failure")
	}
}
