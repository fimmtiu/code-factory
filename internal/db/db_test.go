package db_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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
	if err := d.CreateProject("my-proj", "A project", nil, "", nil); err != nil {
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
		if err := d.CreateProject(id, "A project", nil, "", nil); err != nil {
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
	if err := d.CreateProject("my-proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj")) {
		t.Error("expected .code-factory/my-proj/ to be created")
	}
}

func TestCreateProject_CreatesNestedDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("my-proj/sub-proj", "A subproject", nil, "", nil); err != nil {
		t.Fatalf("CreateProject nested: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "sub-proj")) {
		t.Error("expected .code-factory/my-proj/sub-proj/ to be created")
	}
}

// ===== CreateProject phase =====

func TestCreateProject_DefaultPhaseIsOpen(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, "", nil); err != nil {
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
	if err := d.CreateProject("parent", "A parent project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("parent/child", "A child project", nil, "", nil); err != nil {
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
		if err := d.CreateProject(id, "A project", nil, "", nil); err != nil {
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
	err := d.CreateProject("nonexistent/child", "A child project", nil, "", nil)
	if err == nil {
		t.Error("expected error when parent project does not exist, got nil")
	}
}

// ===== CreateTicket directory creation =====

func TestCreateTicket_CreatesDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("my-proj/my-ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "my-ticket")) {
		t.Error("expected .code-factory/my-proj/my-ticket/ to be created")
	}
}

func TestCreateTicket_CreatesDirectoryDeeplyNested(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateProject("proj/sub", "A subproject", nil, "", nil); err != nil {
		t.Fatalf("CreateProject sub: %v", err)
	}
	if err := d.CreateTicket("proj/sub/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "proj", "sub", "ticket")) {
		t.Error("expected .code-factory/proj/sub/ticket/ to be created")
	}
}

func TestCreateTicket_NoDirectoryOnFailure(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	// Attempt to create a ticket whose parent project doesn't exist — should fail.
	err := d.CreateTicket("nonexistent-proj/ticket", "A ticket", nil, "", nil)
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"t1", "t2", "t3", "t4"} {
		if err := d.CreateTicket("proj/"+id, "desc", nil, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/dep", "A dependency", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", []string{"proj/dep"}, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "Ticket 1", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Ticket 2", nil, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "custom-branch", nil); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj")
	if u.ParentBranch != "custom-branch" {
		t.Errorf("expected ParentBranch %q, got %q", "custom-branch", u.ParentBranch)
	}
}

func TestCreateProject_DefaultParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
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
		err := d.CreateProject("proj", "A project", nil, c.branch, nil)
		if err == nil {
			t.Errorf("expected error for parent_branch %q, got nil", c.branch)
		} else if !strings.Contains(err.Error(), c.bad) {
			t.Errorf("expected error mentioning %q for parent_branch %q, got: %v", c.bad, c.branch, err)
		}
	}
}

func TestCreateTicket_RejectsInvalidParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	err := d.CreateTicket("proj/ticket", "A ticket", nil, "HEAD@{5}", nil)
	if err == nil {
		t.Error("expected error for parent_branch with @{, got nil")
	}
}

func TestCreateTicket_StoresParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "custom-branch", nil); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj/ticket")
	if u.ParentBranch != "custom-branch" {
		t.Errorf("expected ParentBranch %q, got %q", "custom-branch", u.ParentBranch)
	}
}

func TestCreateTicket_DefaultParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	u := findUnit(t, d, "proj/ticket")
	if u.ParentBranch != "" {
		t.Errorf("expected empty ParentBranch, got %q", u.ParentBranch)
	}
}

func TestCreateTicket_ImplementWhenAllDependenciesDone(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// Another project used as a completed dependency.
	if err := d.CreateProject("donedep", "Done dep", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "a", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// Mark both dependencies as done.
	if err := d.SetProjectPhase("donedep", string(models.ProjectPhaseDone)); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := d.CreateTicket("proj/b", "b", []string{"proj/a", "donedep"}, "", nil); err != nil {
		t.Fatal(err)
	}
	u := findUnit(t, d, "proj/b")
	if u.Phase != models.PhaseImplement {
		t.Errorf("phase = %s, want implement (all deps done)", u.Phase)
	}
}

func TestCreateTicket_BlockedWhenSomeDependenciesOpen(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "a", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "b", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// Only one dep is done.
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if err := d.CreateTicket("proj/c", "c", []string{"proj/a", "proj/b"}, "", nil); err != nil {
		t.Fatal(err)
	}
	u := findUnit(t, d, "proj/c")
	if u.Phase != models.PhaseBlocked {
		t.Errorf("phase = %s, want blocked (one dep still open)", u.Phase)
	}
}

func TestGetTicket_IncludesParentBranch(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "release-branch", nil); err != nil {
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

	if err := d.CreateProject("main-proj", "Main project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("other-proj", "Other project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("main-proj/ticket", "A ticket", nil, "other-proj", nil); err != nil {
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

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// Set parent_branch to "main" — should merge into repoRoot, not parent project.
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "main", nil); err != nil {
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

	if err := d.CreateProject("parent-proj", "Parent", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("other-proj", "Other", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("parent-proj/child", "Child", nil, "other-proj", nil); err != nil {
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

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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

func TestMarkTicketDone_SquashesTicketBranch(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	git.Squashes = nil

	if err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if len(git.Squashes) != 1 {
		t.Fatalf("expected one squash on the ticket branch, got %v", git.Squashes)
	}
	wantWorktree := filepath.Join(ticketsDir, "proj", "ticket", "worktree")
	if git.Squashes[0] != wantWorktree {
		t.Errorf("expected squash on %q, got %q", wantWorktree, git.Squashes[0])
	}
}

func TestMarkTicketDone_RejectsForbiddenMarkers(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	// Inject a marker hit; the fake returns this verbatim from
	// FindForbiddenMarkers regardless of the diff.
	git.ForbiddenMarkers = []string{"internal/refresh/refresh.go:24: // TODO: implement periodic refresh"}
	git.Squashes = nil
	git.MergeTargets = nil

	err := d.SetStatus("proj/ticket", models.PhaseDone, models.StatusIdle)
	var fmErr *db.ForbiddenMarkersError
	if !errors.As(err, &fmErr) {
		t.Fatalf("expected *db.ForbiddenMarkersError, got %T: %v", err, err)
	}
	if fmErr.Identifier != "proj/ticket" {
		t.Errorf("Identifier = %q, want %q", fmErr.Identifier, "proj/ticket")
	}
	wantWorktree := filepath.Join(ticketsDir, "proj", "ticket", "worktree")
	if fmErr.WorktreePath != wantWorktree {
		t.Errorf("WorktreePath = %q, want %q", fmErr.WorktreePath, wantWorktree)
	}
	if len(fmErr.Markers) != 1 || !strings.Contains(fmErr.Markers[0], "TODO") {
		t.Errorf("Markers = %v, want a single TODO entry", fmErr.Markers)
	}
	// We must abort before any history-rewriting or fast-forward step.
	if len(git.Squashes) != 0 {
		t.Errorf("expected no squash when markers present, got %v", git.Squashes)
	}
	if len(git.MergeTargets) != 0 {
		t.Errorf("expected no fast-forward merge when markers present, got %v", git.MergeTargets)
	}
}

func TestMarkTicketDoneCascading_SquashesTicketsButNotProjects(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("grand", "Grandparent", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("grand/parent", "Parent", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("grand/parent/t1", "Only ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	git.Squashes = nil

	if err := d.MarkTicketDoneCascading("grand/parent/t1"); err != nil {
		t.Fatalf("MarkTicketDoneCascading: %v", err)
	}

	// Cascade rebases ticket → parent project → grandparent project (3 steps).
	// Only the ticket step should squash.
	if len(git.Squashes) != 1 {
		t.Fatalf("expected exactly one squash (ticket only), got %d: %v", len(git.Squashes), git.Squashes)
	}
	wantWorktree := filepath.Join(ticketsDir, "grand", "parent", "t1", "worktree")
	if git.Squashes[0] != wantWorktree {
		t.Errorf("expected squash on ticket worktree %q, got %q", wantWorktree, git.Squashes[0])
	}
}

func TestRebaseTicketOnParent_AbortsOnFailure(t *testing.T) {
	d, _, git := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "release-v2", nil); err != nil {
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
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
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

// ===== UpdateChangeRequestDescription =====

// ===== Rerere across cascade =====

// assertRerereEnabled checks that EnableRerere was called with the given
// worktree path. It fails the test with a descriptive message if not found.
func assertRerereEnabled(t *testing.T, git *gitutil.FakeGitClient, want string) {
	t.Helper()
	for _, dir := range git.RereresEnabled {
		if dir == want {
			return
		}
	}
	t.Errorf("expected EnableRerere on %q, got %v", want, git.RereresEnabled)
}

func TestMergeChain_EnablesRerereOnWorktrees(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	git.RereresEnabled = nil

	if err := d.MarkTicketDoneCascading("proj/ticket"); err != nil {
		t.Fatal(err)
	}

	if len(git.RereresEnabled) == 0 {
		t.Fatal("expected EnableRerere to be called")
	}
	assertRerereEnabled(t, git, filepath.Join(ticketsDir, "proj", "ticket", "worktree"))
}

func TestMergeChain_EnablesRerereOnEachCascadeStep(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("grand", "Grandparent", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("grand/parent", "Parent", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("grand/parent/t1", "Only ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	git.RereresEnabled = nil

	if err := d.MarkTicketDoneCascading("grand/parent/t1"); err != nil {
		t.Fatal(err)
	}

	// Cascade has 3 steps: ticket → parent → grandparent.
	// EnableRerere should be called on the worktree for each step.
	if len(git.RereresEnabled) < 3 {
		t.Fatalf("expected at least 3 EnableRerere calls for cascade, got %d: %v",
			len(git.RereresEnabled), git.RereresEnabled)
	}

	assertRerereEnabled(t, git, filepath.Join(ticketsDir, "grand", "parent", "t1", "worktree"))
	assertRerereEnabled(t, git, filepath.Join(ticketsDir, "grand", "parent", "worktree"))
	assertRerereEnabled(t, git, filepath.Join(ticketsDir, "grand", "worktree"))
}

func TestMergeChain_EnablesRerereOnConflictWorktree(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)

	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	// MergeChain calls git.IsWorktreeClean (a real git command) on the
	// conflict worktree after onConflict returns. Initialise a bare git
	// repo at the worktree path so `git status` succeeds.
	worktreeDir := filepath.Join(ticketsDir, "proj", "ticket", "worktree")
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", worktreeDir).CombinedOutput(); err != nil {
		t.Fatalf("git init worktree: %v\n%s", err, out)
	}

	// Make the first rebase fail, then succeed on retry.
	callCount := 0
	git.RebaseErrFunc = func(worktreeDir, ontoBranch string) error {
		callCount++
		if callCount == 1 {
			return errors.New("conflict")
		}
		return nil
	}
	git.RereresEnabled = nil

	conflictSeen := false
	onConflict := func(stepIdentifier, worktreePath string) error {
		conflictSeen = true
		return nil
	}

	if err := d.MergeChain(context.Background(), "proj/ticket", onConflict); err != nil {
		t.Fatalf("MergeChain: %v", err)
	}

	if !conflictSeen {
		t.Fatal("expected onConflict to be called")
	}

	// EnableRerere should have been called on the child worktree
	// (before the rebase, so its resolution is recorded for later).
	assertRerereEnabled(t, git, filepath.Join(ticketsDir, "proj", "ticket", "worktree"))
}

func TestUpdateChangeRequestDescription_AcceptsStringID(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.AddChangeRequest("proj/ticket", "main.go:10", "alice", "original"); err != nil {
		t.Fatal(err)
	}

	crs, err := d.OpenChangeRequests("proj/ticket")
	if err != nil {
		t.Fatal(err)
	}
	if len(crs) != 1 {
		t.Fatalf("expected 1 CR, got %d", len(crs))
	}

	// Use the string ID from the model directly.
	if err := d.UpdateChangeRequestDescription(crs[0].ID, "updated"); err != nil {
		t.Fatalf("UpdateChangeRequestDescription: %v", err)
	}

	wu, err := d.GetTicket("proj/ticket")
	if err != nil {
		t.Fatal(err)
	}
	if len(wu.ChangeRequests) != 1 {
		t.Fatalf("expected 1 CR, got %d", len(wu.ChangeRequests))
	}
	if wu.ChangeRequests[0].Description != "updated" {
		t.Errorf("expected description %q, got %q", "updated", wu.ChangeRequests[0].Description)
	}
}

func TestUpdateChangeRequestDescription_InvalidID(t *testing.T) {
	d, _, _ := openTestDB(t)
	err := d.UpdateChangeRequestDescription("not-a-number", "desc")
	if err == nil {
		t.Fatal("expected error for non-numeric ID, got nil")
	}
	if !strings.Contains(err.Error(), "invalid change request id") {
		t.Errorf("expected error mentioning invalid ID, got: %v", err)
	}
}

func TestUpdateChangeRequestDescription_NotFound(t *testing.T) {
	d, _, _ := openTestDB(t)
	err := d.UpdateChangeRequestDescription("99999", "desc")
	if err == nil {
		t.Fatal("expected error for nonexistent CR, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

// ===== Stacked (chained) tickets =====
//
// When sibling tickets share write scope, the planner chains them via
// dependencies (A → B → C) instead of running them in parallel. These
// tests verify that the blocking/unblocking and sequential merge
// behaviour works correctly for such chains.

func TestStackedTickets_ChainStartsBlocked(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// Create a chain: A (no deps) → B (depends on A) → C (depends on B).
	if err := d.CreateTicket("proj/a", "first in chain", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second in chain", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/c", "third in chain", []string{"proj/b"}, "", nil); err != nil {
		t.Fatal(err)
	}

	a := findUnit(t, d, "proj/a")
	b := findUnit(t, d, "proj/b")
	c := findUnit(t, d, "proj/c")

	if a.Phase != models.PhaseImplement {
		t.Errorf("proj/a: phase = %s, want implement (head of chain)", a.Phase)
	}
	if b.Phase != models.PhaseBlocked {
		t.Errorf("proj/b: phase = %s, want blocked (depends on a)", b.Phase)
	}
	if c.Phase != models.PhaseBlocked {
		t.Errorf("proj/c: phase = %s, want blocked (depends on b)", c.Phase)
	}
}

func TestStackedTickets_CompletingHeadUnblocksNext(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "first", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/c", "third", []string{"proj/b"}, "", nil); err != nil {
		t.Fatal(err)
	}

	// Complete A.
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	b := findUnit(t, d, "proj/b")
	c := findUnit(t, d, "proj/c")

	if b.Phase != models.PhaseImplement {
		t.Errorf("proj/b: phase = %s, want implement (a is done)", b.Phase)
	}
	// C still blocked because B is not done yet.
	if c.Phase != models.PhaseBlocked {
		t.Errorf("proj/c: phase = %s, want blocked (b not done)", c.Phase)
	}
}

func TestStackedTickets_FullChainUnblock(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "first", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/c", "third", []string{"proj/b"}, "", nil); err != nil {
		t.Fatal(err)
	}

	// Complete A, then B.
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}
	if err := d.SetStatus("proj/b", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	c := findUnit(t, d, "proj/c")
	if c.Phase != models.PhaseImplement {
		t.Errorf("proj/c: phase = %s, want implement (full chain complete)", c.Phase)
	}
}

func TestStackedTickets_MergeSequentially(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "first", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}

	// Complete A — should merge into project worktree.
	git.MergeTargets = nil
	git.Squashes = nil
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	expectedTarget := filepath.Join(ticketsDir, "proj", "worktree")
	if len(git.MergeTargets) == 0 {
		t.Fatal("expected A to merge into project worktree")
	}
	if git.MergeTargets[0] != expectedTarget {
		t.Errorf("A merge target: got %q, want %q", git.MergeTargets[0], expectedTarget)
	}

	// Complete B — should also merge into the same project worktree, which
	// now includes A's changes, preventing conflicts on shared files.
	git.MergeTargets = nil
	if err := d.SetStatus("proj/b", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if len(git.MergeTargets) == 0 {
		t.Fatal("expected B to merge into project worktree")
	}
	if git.MergeTargets[0] != expectedTarget {
		t.Errorf("B merge target: got %q, want %q", git.MergeTargets[0], expectedTarget)
	}
}

func TestStackedTickets_ClaimRespectsOrder(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "first", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}

	// Only A should be claimable; B is blocked.
	wu, err := d.Claim(1)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if wu.Identifier != "proj/a" {
		t.Errorf("expected to claim proj/a, got %q", wu.Identifier)
	}

	// No more tickets should be claimable (B is still blocked).
	_, err = d.Claim(2)
	if err == nil {
		t.Error("expected no claimable ticket while B is blocked, but Claim succeeded")
	}
}

func TestStackedTickets_NotifyWorkAvailableOnUnblock(t *testing.T) {
	d, _, _ := openTestDB(t)

	notified := 0
	d.SetOnWorkAvailable(func() { notified++ })

	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/a", "first", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/b", "second", []string{"proj/a"}, "", nil); err != nil {
		t.Fatal(err)
	}

	notified = 0 // reset after setup
	if err := d.SetStatus("proj/a", models.PhaseDone, models.StatusIdle); err != nil {
		t.Fatal(err)
	}

	if notified == 0 {
		t.Error("expected onWorkAvailable notification when stacked ticket is unblocked")
	}
}

// ===== GetSiblingDescriptions =====

func TestGetSiblingDescriptions_ReturnsSiblingTickets(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "Add rate-limiting middleware", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Refactor request pipeline", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t3", "Add caching layer", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	siblings, err := d.GetSiblingDescriptions("proj/t1")
	if err != nil {
		t.Fatalf("GetSiblingDescriptions: %v", err)
	}

	if len(siblings) != 2 {
		t.Fatalf("expected 2 siblings, got %d", len(siblings))
	}

	// Collect identifiers and descriptions.
	found := map[string]string{}
	for _, s := range siblings {
		found[s.Identifier] = s.Description
	}

	if found["proj/t2"] != "Refactor request pipeline" {
		t.Errorf("expected proj/t2 description, got %q", found["proj/t2"])
	}
	if found["proj/t3"] != "Add caching layer" {
		t.Errorf("expected proj/t3 description, got %q", found["proj/t3"])
	}
	if _, ok := found["proj/t1"]; ok {
		t.Error("expected the queried ticket to be excluded from siblings")
	}
}

func TestGetSiblingDescriptions_IncludesSiblingSubprojects(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("proj/sub", "A subproject that refactors core", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "Add a feature", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	siblings, err := d.GetSiblingDescriptions("proj/t1")
	if err != nil {
		t.Fatalf("GetSiblingDescriptions: %v", err)
	}

	if len(siblings) != 1 {
		t.Fatalf("expected 1 sibling (the subproject), got %d", len(siblings))
	}
	if siblings[0].Identifier != "proj/sub" {
		t.Errorf("expected identifier proj/sub, got %q", siblings[0].Identifier)
	}
	if siblings[0].Description != "A subproject that refactors core" {
		t.Errorf("expected subproject description, got %q", siblings[0].Description)
	}
}

func TestGetSiblingDescriptions_NoParentReturnsEmpty(t *testing.T) {
	d, _, _ := openTestDB(t)
	// A top-level ticket has no parent project; siblings are meaningless.
	if err := d.CreateTicket("standalone", "A standalone ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	siblings, err := d.GetSiblingDescriptions("standalone")
	if err != nil {
		t.Fatalf("GetSiblingDescriptions: %v", err)
	}
	if len(siblings) != 0 {
		t.Errorf("expected 0 siblings for top-level ticket, got %d", len(siblings))
	}
}

func TestGetSiblingDescriptions_MissingParentProjectReturnsEmpty(t *testing.T) {
	d, _, _ := openTestDB(t)
	// Query an identifier whose parent project doesn't exist in the DB.
	// This exercises the sql.ErrNoRows path — should return nil, nil rather
	// than an error.
	siblings, err := d.GetSiblingDescriptions("nonexistent-proj/t1")
	if err != nil {
		t.Fatalf("expected nil error for missing parent project, got: %v", err)
	}
	if len(siblings) != 0 {
		t.Errorf("expected 0 siblings for missing parent, got %d", len(siblings))
	}
}

func TestGetSiblingDescriptions_OnlyChildReturnsEmpty(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/only", "The only ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	siblings, err := d.GetSiblingDescriptions("proj/only")
	if err != nil {
		t.Fatalf("GetSiblingDescriptions: %v", err)
	}
	if len(siblings) != 0 {
		t.Errorf("expected 0 siblings for only child, got %d", len(siblings))
	}
}

// ===== WriteScope storage =====

func TestCreateTicket_StoresWriteScope(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", []string{"internal/db/", "cmd/main.go"}); err != nil {
		t.Fatal(err)
	}

	scope, err := d.GetWriteScope("proj/ticket")
	if err != nil {
		t.Fatalf("GetWriteScope: %v", err)
	}
	if len(scope) != 2 {
		t.Fatalf("expected 2 write_scope entries, got %d: %v", len(scope), scope)
	}
	if scope[0] != "internal/db/" || scope[1] != "cmd/main.go" {
		t.Errorf("unexpected write_scope: %v", scope)
	}
}

func TestCreateTicket_EmptyWriteScope(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	scope, err := d.GetWriteScope("proj/ticket")
	if err != nil {
		t.Fatalf("GetWriteScope: %v", err)
	}
	if len(scope) != 0 {
		t.Errorf("expected empty write_scope, got %v", scope)
	}
}

func TestCreateProject_StoresWriteScope(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", []string{"pkg/api/"}); err != nil {
		t.Fatal(err)
	}

	scope, err := d.GetWriteScope("proj")
	if err != nil {
		t.Fatalf("GetWriteScope: %v", err)
	}
	if len(scope) != 1 || scope[0] != "pkg/api/" {
		t.Errorf("unexpected write_scope: %v", scope)
	}
}

func TestSetWriteScope_OverwritesExisting(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil, "", []string{"old/path/"}); err != nil {
		t.Fatal(err)
	}

	if err := d.SetWriteScope("proj/ticket", []string{"new/path/"}); err != nil {
		t.Fatalf("SetWriteScope: %v", err)
	}

	scope, err := d.GetWriteScope("proj/ticket")
	if err != nil {
		t.Fatalf("GetWriteScope: %v", err)
	}
	if len(scope) != 1 || scope[0] != "new/path/" {
		t.Errorf("expected [new/path/], got %v", scope)
	}
}

func TestSetWriteScope_Project(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", []string{"old/"}); err != nil {
		t.Fatal(err)
	}

	if err := d.SetWriteScope("proj", []string{"new/path/"}); err != nil {
		t.Fatalf("SetWriteScope on project: %v", err)
	}

	scope, err := d.GetWriteScope("proj")
	if err != nil {
		t.Fatalf("GetWriteScope: %v", err)
	}
	if len(scope) != 1 || scope[0] != "new/path/" {
		t.Errorf("expected [new/path/], got %v", scope)
	}
}

func TestGetWriteScope_NotFound(t *testing.T) {
	d, _, _ := openTestDB(t)
	_, err := d.GetWriteScope("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent identifier")
	}
}

// ===== Sibling exclusion at claim time =====

func TestClaim_SkipsTicketWithOverlappingSiblingScope(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// t1 and t2 are siblings with overlapping write_scope.
	if err := d.CreateTicket("proj/t1", "First ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Second ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}

	// Claim t1.
	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim t1: %v", err)
	}
	if wu1.Identifier != "proj/t1" {
		t.Fatalf("expected to claim proj/t1, got %q", wu1.Identifier)
	}

	// Attempting to claim again should fail because t2 overlaps with the
	// currently-claimed t1.
	_, err = d.Claim(200)
	if err == nil {
		t.Fatal("expected error when claiming t2 (overlapping scope with claimed t1)")
	}
	if !strings.Contains(err.Error(), "no claimable ticket") {
		t.Errorf("expected 'no claimable ticket' error, got: %v", err)
	}
}

func TestClaim_AllowsNonOverlappingSiblings(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// t1 and t2 are siblings with disjoint write_scope.
	if err := d.CreateTicket("proj/t1", "First ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Second ticket", nil, "", []string{"cmd/server/"}); err != nil {
		t.Fatal(err)
	}

	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim t1: %v", err)
	}
	if wu1.Identifier != "proj/t1" {
		t.Fatalf("expected to claim proj/t1, got %q", wu1.Identifier)
	}

	// t2 should be claimable because its scope doesn't overlap t1's.
	wu2, err := d.Claim(200)
	if err != nil {
		t.Fatalf("Claim t2: %v", err)
	}
	if wu2.Identifier != "proj/t2" {
		t.Errorf("expected to claim proj/t2, got %q", wu2.Identifier)
	}
}

func TestClaim_AllowsOverlappingNonSiblings(t *testing.T) {
	d, _, _ := openTestDB(t)
	// Two separate projects — tickets are NOT siblings.
	if err := d.CreateProject("proj-a", "Project A", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateProject("proj-b", "Project B", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj-a/t1", "Ticket in A", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj-b/t1", "Ticket in B", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}

	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim first: %v", err)
	}

	// The other ticket should be claimable because they're not siblings.
	wu2, err := d.Claim(200)
	if err != nil {
		t.Fatalf("Claim second: %v", err)
	}
	if wu1.Identifier == wu2.Identifier {
		t.Error("both claims returned the same ticket")
	}
}

func TestClaim_NoScopeTicketsAlwaysClaimable(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// t1 has scope, t2 has no scope — no overlap possible.
	if err := d.CreateTicket("proj/t1", "First ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Second ticket", nil, "", nil); err != nil {
		t.Fatal(err)
	}

	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim t1: %v", err)
	}
	if wu1.Identifier != "proj/t1" {
		t.Fatalf("expected to claim proj/t1, got %q", wu1.Identifier)
	}

	// t2 has no write_scope, so it's always claimable.
	wu2, err := d.Claim(200)
	if err != nil {
		t.Fatalf("Claim t2: %v", err)
	}
	if wu2.Identifier != "proj/t2" {
		t.Errorf("expected to claim proj/t2, got %q", wu2.Identifier)
	}
}

func TestClaim_PrefixOverlapBlocksClaim(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	// t1 declares a directory scope, t2 declares a file inside that directory.
	if err := d.CreateTicket("proj/t1", "First ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Second ticket", nil, "", []string{"internal/db/schema.go"}); err != nil {
		t.Fatal(err)
	}

	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim t1: %v", err)
	}
	if wu1.Identifier != "proj/t1" {
		t.Fatalf("expected to claim proj/t1, got %q", wu1.Identifier)
	}

	// t2's scope "internal/db/schema.go" is inside t1's "internal/db/" — overlap.
	_, err = d.Claim(200)
	if err == nil {
		t.Fatal("expected error: t2 scope overlaps with claimed t1 via prefix")
	}
}

func TestClaim_ReleasedTicketUnblocksScope(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t1", "First ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/t2", "Second ticket", nil, "", []string{"internal/db/"}); err != nil {
		t.Fatal(err)
	}

	// Claim and then release t1.
	wu1, err := d.Claim(100)
	if err != nil {
		t.Fatalf("Claim t1: %v", err)
	}
	if err := d.Release(wu1.Identifier); err != nil {
		t.Fatalf("Release t1: %v", err)
	}

	// Now t2 should be claimable since t1 is no longer claimed.
	wu2, err := d.Claim(200)
	if err != nil {
		t.Fatalf("Claim t2 after release: %v", err)
	}
	// Either t1 or t2 is fine — the point is that a claim succeeds.
	if wu2.Identifier != "proj/t1" && wu2.Identifier != "proj/t2" {
		t.Errorf("expected to claim proj/t1 or proj/t2, got %q", wu2.Identifier)
	}
}
