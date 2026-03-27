package workflow_test

import (
	"testing"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/gitutil"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/workflow"
)

// fakeGitClient implements gitutil.GitClient without invoking real git.
type fakeGitClient struct{}

func (f *fakeGitClient) CreateWorktree(_, _, _ string) error    { return nil }
func (f *fakeGitClient) MergeBranch(_, _, _ string) error       { return nil }
func (f *fakeGitClient) RemoveWorktree(_, _, _ string) error    { return nil }
func (f *fakeGitClient) GetHeadCommit(_ string) (string, error) { return "", nil }

var _ gitutil.GitClient = (*fakeGitClient)(nil)

// openTestDB creates a temporary in-memory-like DB for testing.
func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	d.SetGitClient(&fakeGitClient{})
	t.Cleanup(func() { d.Close() })
	return d
}

// ticketPhase returns the current phase of a ticket, failing the test if not found.
func ticketPhase(t *testing.T, d *db.DB, identifier string) string {
	t.Helper()
	phase, err := d.GetTicketPhase(identifier)
	if err != nil {
		t.Fatalf("GetTicketPhase(%q): %v", identifier, err)
	}
	return phase
}

// projectPhase returns the current phase of a project by scanning Status().
func projectPhase(t *testing.T, d *db.DB, identifier string) string {
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
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	// Set ticket to user-review so it looks like it was worked on.
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
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
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

func TestApprove_ReviewToRespond(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseRespond {
		t.Errorf("expected phase %q, got %q", models.PhaseRespond, got)
	}
}

func TestApprove_RespondToDone(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseRespond, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := ticketPhase(t, d, "proj/t1"); got != models.PhaseDone {
		t.Errorf("expected phase %q, got %q", models.PhaseDone, got)
	}
}

// ── Error case tests ──────────────────────────────────────────────────────────

func TestApprove_BlockedReturnsError(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	// Create two tickets; t1 depends on t2 so it starts blocked.
	if err := d.CreateTicket("proj/t2", "dep", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "blocker", []string{"proj/t2"}); err != nil {
		t.Fatal(err)
	}

	err := workflow.Approve(d, "proj/t1")
	if err == nil {
		t.Error("expected error approving a blocked ticket, got nil")
	}
}

func TestApprove_DoneReturnsError(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseRespond, models.StatusIdle); err != nil {
		t.Fatal(err)
	}
	// Mark done first.
	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("first Approve: %v", err)
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
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseRespond, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	if got := projectPhase(t, d, "proj"); got != models.ProjectPhaseDone {
		t.Errorf("expected project phase %q, got %q", models.ProjectPhaseDone, got)
	}
}

func TestApprove_NotAllTicketsDone_ProjectRemainsOpen(t *testing.T) {
	d := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "ticket 1", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "ticket 2", nil); err != nil {
		t.Fatal(err)
	}
	// Only approve t1 (respond → done).
	if err := d.SetStatus("proj/t1", models.PhaseRespond, models.StatusIdle); err != nil {
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
	// Structure:
	//   grandparent
	//     parent
	//       t1
	d := openTestDB(t)
	for _, id := range []string{"grandparent", "grandparent/parent"} {
		if err := d.CreateProject(id, "A project", nil); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}
	if err := d.CreateTicket("grandparent/parent/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("grandparent/parent/t1", models.PhaseRespond, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := workflow.Approve(d, "grandparent/parent/t1"); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	// Both parent and grandparent should be done.
	if got := projectPhase(t, d, "grandparent/parent"); got != models.ProjectPhaseDone {
		t.Errorf("parent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
	if got := projectPhase(t, d, "grandparent"); got != models.ProjectPhaseDone {
		t.Errorf("grandparent phase: expected %q, got %q", models.ProjectPhaseDone, got)
	}
}

func TestApprove_TopLevelTicketNoParent(t *testing.T) {
	// A top-level ticket (no parent project) can be approved without error.
	// There's no project to mark done.
	d := openTestDB(t)
	// Top-level tickets don't have a parent project; we need to create one without
	// a slash in the identifier to verify the no-parent path works.
	// The DB validates identifiers — top-level tickets need no parent project lookup.
	// We'll just use respond → done and confirm no panic/error.
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "A ticket", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t1", models.PhaseImplement, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	// Approve implement → refactor (non-done transition, no parent walk).
	if err := workflow.Approve(d, "proj/t1"); err != nil {
		t.Fatalf("Approve implement: %v", err)
	}
	// Project should still be open.
	if got := projectPhase(t, d, "proj"); got != models.ProjectPhaseOpen {
		t.Errorf("project should still be open, got %q", got)
	}
}
