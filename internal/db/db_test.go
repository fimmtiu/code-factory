package db_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/gitutil"
	"github.com/fimmtiu/code-factory/internal/models"
)

// openTestDB creates a temporary directory, opens a fresh DB in it, and
// injects a fake git client so tests don't require a real git repository.
// It returns the DB handle, the ticketsDir path, and the fake client.
func openTestDB(t *testing.T) (*db.DB, string, *gitutil.FakeGitClient) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	git := &gitutil.FakeGitClient{}
	d.SetGitClient(git)
	t.Cleanup(func() { d.Close() })
	return d, dir, git
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// findUnit calls Status() and returns the work unit matching identifier.
func findUnit(t *testing.T, d *db.DB, identifier string) *models.WorkUnit {
	t.Helper()
	units, err := d.Status()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range units {
		if u.Identifier == identifier {
			return u
		}
	}
	t.Fatalf("work unit %q not found", identifier)
	return nil
}

// ===== SetStatus last_updated trigger =====

func TestSetStatus_UpdatesLastUpdated(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	// Sleep so the status-change timestamp is distinguishable from the
	// creation timestamp, which has one-second resolution.
	time.Sleep(time.Second)
	before := time.Now().Unix()

	if err := d.SetStatus("proj/ticket", models.PhaseReview, models.StatusIdle); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, u := range units {
		if u.Identifier == "proj/ticket" {
			if u.LastUpdated.Unix() < before {
				t.Errorf("expected last_updated >= %d after SetStatus, got %d",
					before, u.LastUpdated.Unix())
			}
			return
		}
	}
	t.Error("ticket not found")
}

// ===== CreateProject directory and worktree creation =====

func TestCreateProject_CreatesWorktree(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if len(git.WorktreesCreated) != 1 {
		t.Fatalf("expected 1 worktree created, got %d", len(git.WorktreesCreated))
	}
	want := filepath.Join(ticketsDir, "my-proj", "worktree")
	if git.WorktreesCreated[0] != want {
		t.Errorf("worktree path: got %q, want %q", git.WorktreesCreated[0], want)
	}
}

func TestCreateProject_EachSubprojectGetsOwnWorktree(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)
	for _, id := range []string{"proj", "proj/sub"} {
		if err := d.CreateProject(id, "A project", nil, ""); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}

	if len(git.WorktreesCreated) != 2 {
		t.Fatalf("expected 2 worktrees created, got %d", len(git.WorktreesCreated))
	}
	wantPaths := []string{
		filepath.Join(ticketsDir, "proj", "worktree"),
		filepath.Join(ticketsDir, "proj", "sub", "worktree"),
	}
	for i, want := range wantPaths {
		if git.WorktreesCreated[i] != want {
			t.Errorf("worktree[%d]: got %q, want %q", i, git.WorktreesCreated[i], want)
		}
	}
}

func TestCreateProject_CreatesDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj")) {
		t.Error("expected .code-factory/my-proj/ to be created")
	}
}

func TestCreateProject_CreatesNestedDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("my-proj/sub-proj", "A subproject", nil, ""); err != nil {
		t.Fatalf("CreateProject nested: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "sub-proj")) {
		t.Error("expected .code-factory/my-proj/sub-proj/ to be created")
	}
}

// ===== CreateProject phase =====

func TestCreateProject_DefaultPhaseIsOpen(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, u := range units {
		if u.Identifier == "my-proj" {
			if u.Phase != "open" {
				t.Errorf("expected phase %q, got %q", "open", u.Phase)
			}
			return
		}
	}
	t.Error("project not found in Status output")
}

// ===== CreateProject parent FK =====

func TestCreateProject_SubprojectRecordsParent(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("parent", "A parent project", nil, ""); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("parent/child", "A child project", nil, ""); err != nil {
		t.Fatalf("CreateProject child: %v", err)
	}

	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	for _, u := range units {
		if u.Identifier == "parent/child" {
			if u.Parent != "parent" {
				t.Errorf("expected Parent = %q, got %q", "parent", u.Parent)
			}
			return
		}
	}
	t.Error("child project not found in Status output")
}

func TestCreateProject_DeeplyNestedParents(t *testing.T) {
	d, _, _ := openTestDB(t)
	for _, id := range []string{"foo", "foo/bar", "foo/bar/baz"} {
		if err := d.CreateProject(id, "A project", nil, ""); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}

	units, err := d.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	parents := map[string]string{}
	for _, u := range units {
		parents[u.Identifier] = u.Parent
	}

	if parents["foo"] != "" {
		t.Errorf("foo: expected no parent, got %q", parents["foo"])
	}
	if parents["foo/bar"] != "foo" {
		t.Errorf("foo/bar: expected parent %q, got %q", "foo", parents["foo/bar"])
	}
	if parents["foo/bar/baz"] != "foo/bar" {
		t.Errorf("foo/bar/baz: expected parent %q, got %q", "foo/bar", parents["foo/bar/baz"])
	}
}

func TestCreateProject_MissingParentFails(t *testing.T) {
	d, _, _ := openTestDB(t)
	err := d.CreateProject("nonexistent/child", "A child project", nil, "")
	if err == nil {
		t.Error("expected error when parent project does not exist, got nil")
	}
}

// ===== CreateTicket directory creation =====

func TestCreateTicket_CreatesDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("my-proj/my-ticket", "A ticket", nil, ""); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "my-ticket")) {
		t.Error("expected .code-factory/my-proj/my-ticket/ to be created")
	}
}

func TestCreateTicket_CreatesDirectoryDeeplyNested(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateProject("proj/sub", "A subproject", nil, ""); err != nil {
		t.Fatalf("CreateProject sub: %v", err)
	}
	if err := d.CreateTicket("proj/sub/ticket", "A ticket", nil, ""); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "proj", "sub", "ticket")) {
		t.Error("expected .code-factory/proj/sub/ticket/ to be created")
	}
}

func TestCreateTicket_NoDirectoryOnFailure(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	// Attempt to create a ticket whose parent project doesn't exist — should fail.
	err := d.CreateTicket("nonexistent-proj/ticket", "A ticket", nil, "")
	if err == nil {
		t.Fatal("expected error for ticket with missing parent project, got nil")
	}
	if dirExists(filepath.Join(ticketsDir, "nonexistent-proj", "ticket")) {
		t.Error("directory should not be created when the transaction fails")
	}
}

// ===== ActionableTickets =====

func TestActionableTickets_ReturnsOnlyActionable(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		if err := d.CreateTicket("proj/"+id, "desc", nil, ""); err != nil {
			t.Fatalf("CreateTicket %s: %v", id, err)
		}
	}
	// Set different statuses
	if err := d.SetStatus("proj/t1", "implement", "needs-attention"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t2", "implement", "user-review"); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/t3", "implement", "working"); err != nil {
		t.Fatal(err)
	}
	// t4 stays idle

	tickets, err := d.ActionableTickets()
	if err != nil {
		t.Fatalf("ActionableTickets: %v", err)
	}
	if len(tickets) != 2 {
		t.Fatalf("expected 2 actionable tickets, got %d", len(tickets))
	}
	// needs-attention must come first
	if tickets[0].Status != "needs-attention" {
		t.Errorf("expected first ticket status needs-attention, got %q", tickets[0].Status)
	}
	if tickets[1].Status != "user-review" {
		t.Errorf("expected second ticket status user-review, got %q", tickets[1].Status)
	}
}

func TestGetTicket_ReturnsSingleTicket(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/dep", "A dependency", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", []string{"proj/dep"}, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/ticket", "implement", "working"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/ticket", "main.go:42", "alice", "fix this"); err != nil {
		t.Fatal(err)
	}

	wu, err := d.GetTicket("proj/ticket")
	if err != nil {
		t.Fatalf("GetTicket: %v", err)
	}
	if wu.Identifier != "proj/ticket" {
		t.Errorf("expected identifier proj/ticket, got %q", wu.Identifier)
	}
	if wu.Description != "A ticket" {
		t.Errorf("expected description 'A ticket', got %q", wu.Description)
	}
	if wu.Status != "working" {
		t.Errorf("expected status in-progress, got %q", wu.Status)
	}
	if wu.IsProject {
		t.Error("expected IsProject to be false")
	}
	if wu.Parent != "proj" {
		t.Errorf("expected parent proj, got %q", wu.Parent)
	}
	if len(wu.Dependencies) != 1 || wu.Dependencies[0] != "proj/dep" {
		t.Errorf("expected dependencies [proj/dep], got %v", wu.Dependencies)
	}
	if len(wu.ChangeRequests) != 1 {
		t.Fatalf("expected 1 change request, got %d", len(wu.ChangeRequests))
	}
	if wu.ChangeRequests[0].Author != "alice" {
		t.Errorf("expected change request author alice, got %q", wu.ChangeRequests[0].Author)
	}
}

func TestGetTicket_NotFound(t *testing.T) {
	d, _, _ := openTestDB(t)
	_, err := d.GetTicket("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent ticket")
	}
}

// ===== DeleteChangeRequestsForTicket =====

func TestDeleteChangeRequestsForTicket(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}
	for _, author := range []string{"alice", "bob", "charlie"} {
		if err := d.AddChangeRequest("proj/ticket", "main.go:1", author, "fix"); err != nil {
			t.Fatalf("AddChangeRequest(%s): %v", author, err)
		}
	}

	// Verify CRs exist.
	crs, err := d.OpenChangeRequests("proj/ticket")
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 3 {
		t.Fatalf("expected 3 change requests before delete, got %d", len(crs))
	}

	if err := d.DeleteChangeRequestsForTicket("proj/ticket"); err != nil {
		t.Fatalf("DeleteChangeRequestsForTicket: %v", err)
	}

	crs, err = d.OpenChangeRequests("proj/ticket")
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 0 {
		t.Errorf("expected 0 change requests after delete, got %d", len(crs))
	}
}

func TestDeleteChangeRequestsForTicket_NoChangeRequests(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	// Should succeed even with no CRs to delete.
	if err := d.DeleteChangeRequestsForTicket("proj/ticket"); err != nil {
		t.Fatalf("DeleteChangeRequestsForTicket: %v", err)
	}
}

func TestDeleteChangeRequestsForTicket_NotFound(t *testing.T) {
	d, _, _ := openTestDB(t)
	err := d.DeleteChangeRequestsForTicket("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent ticket, got nil")
	}
}

func TestDeleteChangeRequestsForTicket_LeavesOtherTickets(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "Ticket 1", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Ticket 2", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/t1", "a.go:1", "alice", "fix t1"); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/t2", "b.go:1", "bob", "fix t2"); err != nil {
		t.Fatal(err)
	}

	if err := d.DeleteChangeRequestsForTicket("proj/t1"); err != nil {
		t.Fatalf("DeleteChangeRequestsForTicket: %v", err)
	}

	// t1's CRs should be gone.
	crs, err := d.OpenChangeRequests("proj/t1")
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 0 {
		t.Errorf("expected 0 CRs on t1, got %d", len(crs))
	}

	// t2's CRs should be untouched.
	crs, err = d.OpenChangeRequests("proj/t2")
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 1 {
		t.Errorf("expected 1 CR on t2, got %d", len(crs))
	}
}

// ===== ParentBranch =====

func TestCreateProject_StoresParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "custom-branch"); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj")
	if u.ParentBranch != "custom-branch" {
		t.Errorf("expected ParentBranch %q, got %q", "custom-branch", u.ParentBranch)
	}
}

func TestCreateProject_DefaultParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj")
	if u.ParentBranch != "" {
		t.Errorf("expected empty ParentBranch, got %q", u.ParentBranch)
	}
}

func TestCreateProject_RejectsInvalidParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	cases := []struct {
		branch string
		bad    string
	}{
		{"@{-1}", "@{"},
		{"main..dev", ".."},
		{"branch~1", "~"},
		{"branch^2", "^"},
		{"a:b", ":"},
		{"-flag", "-"},
		{"a b", " "},
	}
	for _, c := range cases {
		err := d.CreateProject("proj", "A project", nil, c.branch)
		if err == nil {
			t.Errorf("expected error for parent_branch %q, got nil", c.branch)
		} else if !strings.Contains(err.Error(), c.bad) {
			t.Errorf("expected error mentioning %q for parent_branch %q, got: %v", c.bad, c.branch, err)
		}
	}
}

func TestCreateTicket_RejectsInvalidParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	err := d.CreateTicket("proj/ticket", "A ticket", nil, "HEAD@{5}")
	if err == nil {
		t.Error("expected error for parent_branch with @{, got nil")
	}
}

func TestCreateTicket_StoresParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "custom-branch"); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj/ticket")
	if u.ParentBranch != "custom-branch" {
		t.Errorf("expected ParentBranch %q, got %q", "custom-branch", u.ParentBranch)
	}
}

func TestCreateTicket_DefaultParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj/ticket")
	if u.ParentBranch != "" {
		t.Errorf("expected empty ParentBranch, got %q", u.ParentBranch)
	}
}

func TestGetTicket_IncludesParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "release-branch"); err != nil {
		t.Fatal(err)
	}

	wu, err := d.GetTicket("proj/ticket")
	if err != nil {
		t.Fatal(err)
	}
	if wu.ParentBranch != "release-branch" {
		t.Errorf("expected ParentBranch %q, got %q", "release-branch", wu.ParentBranch)
	}
}

func TestMarkTicketDone_UsesParentBranch(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("main-proj", "Main project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("other-proj", "Other project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("main-proj/ticket", "A ticket", nil, "other-proj"); err != nil {
		t.Fatal(err)
	}

	if err := d.SetStatus("main-proj/ticket", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	expectedTarget := filepath.Join(ticketsDir, "other-proj", "worktree")
	if len(git.MergeTargets) == 0 {
		t.Fatal("expected MergeBranch to be called")
	}
	if git.MergeTargets[0] != expectedTarget {
		t.Errorf("expected merge target %q, got %q", expectedTarget, git.MergeTargets[0])
	}
}

func TestMarkTicketDone_DefaultMergesIntoParentProject(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	if err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	expectedTarget := filepath.Join(ticketsDir, "proj", "worktree")
	if len(git.MergeTargets) == 0 {
		t.Fatal("expected MergeBranch to be called")
	}
	if git.MergeTargets[0] != expectedTarget {
		t.Errorf("expected merge target %q, got %q", expectedTarget, git.MergeTargets[0])
	}
}

func TestMarkTicketDone_ParentBranchDefaultBranch(t *testing.T) {
	d, _, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	// Set parent_branch to "main" — should merge into repoRoot, not parent project.
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "main"); err != nil {
		t.Fatal(err)
	}

	if err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if len(git.MergeTargets) == 0 {
		t.Fatal("expected MergeBranch to be called")
	}
	// openTestDB uses the same temp dir for both ticketsDir and repoRoot,
	// so just verify the target is NOT a project worktree.
	if strings.Contains(git.MergeTargets[0], "worktree") {
		t.Errorf("expected merge into repoRoot, but got a worktree path: %q", git.MergeTargets[0])
	}
}

func TestSetProjectPhase_UsesParentBranch(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("parent-proj", "Parent", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("other-proj", "Other", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("parent-proj/child", "Child", nil, "other-proj"); err != nil {
		t.Fatal(err)
	}

	if err := d.SetProjectPhase("parent-proj/child", "done"); err != nil {
		t.Fatal(err)
	}

	expectedTarget := filepath.Join(ticketsDir, "other-proj", "worktree")
	if len(git.MergeTargets) == 0 {
		t.Fatal("expected MergeBranch to be called")
	}
	if git.MergeTargets[0] != expectedTarget {
		t.Errorf("expected merge target %q, got %q", expectedTarget, git.MergeTargets[0])
	}
}

func TestMarkTicketDone_RebasesBeforeMerging(t *testing.T) {
	d, _, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	// Clear anything captured while setting up worktrees.
	git.RebaseTargets = nil
	git.MergeTargets = nil

	if err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if len(git.RebaseTargets) == 0 {
		t.Fatal("expected RebaseOnto to be called as part of the rebase strategy")
	}
	if len(git.MergeTargets) == 0 {
		t.Fatal("expected MergeBranch (fast-forward) to be called after rebase")
	}
}

func TestMarkTicketDone_RebaseConflictReturnsChildWorktreePath(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	// Force the rebase to fail so we can verify the error points at the
	// child's worktree (where the user will resolve the conflict), and that
	// no fast-forward merge is attempted.
	git.RebaseErr = errors.New("rebase conflict")
	git.MergeTargets = nil

	err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle)
	if err == nil {
		t.Fatal("expected error from rebase conflict")
	}
	var mergeErr *db.MergeConflictError
	if !errors.As(err, &mergeErr) {
		t.Fatalf("expected *db.MergeConflictError, got %T: %v", err, err)
	}
	expectedWorktree := filepath.Join(ticketsDir, "proj", "ticket", "worktree")
	if mergeErr.WorktreePath != expectedWorktree {
		t.Errorf("expected conflict worktree %q, got %q", expectedWorktree, mergeErr.WorktreePath)
	}
	if len(git.MergeTargets) != 0 {
		t.Errorf("expected no FF merge after a failed rebase, got %v", git.MergeTargets)
	}
}

func TestRebaseTicketOnParent_AbortsOnFailure(t *testing.T) {
	d, _, git := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	git.RebaseErr = errors.New("rebase conflict")
	git.RebasesAborted = nil

	if err := d.RebaseTicketOnParent("proj/ticket", "proj", ""); err == nil {
		t.Fatal("expected error from failing rebase")
	}
	if len(git.RebasesAborted) == 0 {
		t.Fatal("expected AbortRebase to be called so the worktree is left clean")
	}
}

func TestRebaseTicketOnParent_UsesParentBranch(t *testing.T) {
	d, _, git := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "release-v2"); err != nil {
		t.Fatal(err)
	}

	if err := d.RebaseTicketOnParent("proj/ticket", "proj", "release-v2"); err != nil {
		t.Fatal(err)
	}

	if len(git.RebaseTargets) == 0 {
		t.Fatal("expected RebaseOnto to be called")
	}
	if git.RebaseTargets[0] != "release-v2" {
		t.Errorf("expected rebase onto %q, got %q", "release-v2", git.RebaseTargets[0])
	}
}

func TestRebaseTicketOnParent_FallsBackToParentIdentifier(t *testing.T) {
	d, _, git := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, ""); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, ""); err != nil {
		t.Fatal(err)
	}

	if err := d.RebaseTicketOnParent("proj/ticket", "proj", ""); err != nil {
		t.Fatal(err)
	}

	if len(git.RebaseTargets) == 0 {
		t.Fatal("expected RebaseOnto to be called")
	}
	if git.RebaseTargets[0] != "proj" {
		t.Errorf("expected rebase onto %q, got %q", "proj", git.RebaseTargets[0])
	}
}
