package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/gitutil"
)

// fakeGitClient implements gitutil.GitClient without invoking real git.
// It records the worktree paths passed to CreateWorktree.
type fakeGitClient struct {
	worktreesCreated []string
}

func (f *fakeGitClient) CreateWorktree(_, worktreePath, _ string) error {
	f.worktreesCreated = append(f.worktreesCreated, worktreePath)
	return nil
}
func (f *fakeGitClient) MergeBranch(_, _, _ string) error        { return nil }
func (f *fakeGitClient) RemoveWorktree(_, _, _ string) error     { return nil }
func (f *fakeGitClient) GetRepoRoot(path string) (string, error) { return path, nil }
func (f *fakeGitClient) GetHeadCommit(_ string) (string, error)  { return "", nil }

var _ gitutil.GitClient = (*fakeGitClient)(nil) // compile-time interface check

// openTestDB creates a temporary directory, opens a fresh DB in it, and
// injects a fake git client so tests don't require a real git repository.
// It returns the DB handle, the ticketsDir path, and the fake client.
func openTestDB(t *testing.T) (*db.DB, string, *fakeGitClient) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	git := &fakeGitClient{}
	d.SetGitClient(git)
	t.Cleanup(func() { d.Close() })
	return d, dir, git
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ===== SetStatus last_updated trigger =====

func TestSetStatus_UpdatesLastUpdated(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatal(err)
	}
	if err := d.CreateTicket("proj/ticket", "A ticket", nil); err != nil {
		t.Fatal(err)
	}

	// Sleep so the status-change timestamp is distinguishable from the
	// creation timestamp, which has one-second resolution.
	time.Sleep(time.Second)
	before := time.Now().Unix()

	if err := d.SetStatus("proj/ticket", "review", "idle"); err != nil {
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
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	if len(git.worktreesCreated) != 1 {
		t.Fatalf("expected 1 worktree created, got %d", len(git.worktreesCreated))
	}
	want := filepath.Join(ticketsDir, "my-proj", "worktree")
	if git.worktreesCreated[0] != want {
		t.Errorf("worktree path: got %q, want %q", git.worktreesCreated[0], want)
	}
}

func TestCreateProject_EachSubprojectGetsOwnWorktree(t *testing.T) {
	d, ticketsDir, git := openTestDB(t)
	for _, id := range []string{"proj", "proj/sub"} {
		if err := d.CreateProject(id, "A project", nil); err != nil {
			t.Fatalf("CreateProject %q: %v", id, err)
		}
	}

	if len(git.worktreesCreated) != 2 {
		t.Fatalf("expected 2 worktrees created, got %d", len(git.worktreesCreated))
	}
	wantPaths := []string{
		filepath.Join(ticketsDir, "proj", "worktree"),
		filepath.Join(ticketsDir, "proj", "sub", "worktree"),
	}
	for i, want := range wantPaths {
		if git.worktreesCreated[i] != want {
			t.Errorf("worktree[%d]: got %q, want %q", i, git.worktreesCreated[i], want)
		}
	}
}

func TestCreateProject_CreatesDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj")) {
		t.Error("expected .tickets/my-proj/ to be created")
	}
}

func TestCreateProject_CreatesNestedDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("my-proj/sub-proj", "A subproject", nil); err != nil {
		t.Fatalf("CreateProject nested: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "sub-proj")) {
		t.Error("expected .tickets/my-proj/sub-proj/ to be created")
	}
}

// ===== CreateProject phase =====

func TestCreateProject_DefaultPhaseIsOpen(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
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
	if err := d.CreateProject("parent", "A parent project", nil); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateProject("parent/child", "A child project", nil); err != nil {
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
		if err := d.CreateProject(id, "A project", nil); err != nil {
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
	err := d.CreateProject("nonexistent/child", "A child project", nil)
	if err == nil {
		t.Error("expected error when parent project does not exist, got nil")
	}
}

// ===== CreateTicket directory creation =====

func TestCreateTicket_CreatesDirectory(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("my-proj/my-ticket", "A ticket", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj", "my-ticket")) {
		t.Error("expected .tickets/my-proj/my-ticket/ to be created")
	}
}

func TestCreateTicket_CreatesDirectoryDeeplyNested(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateProject("proj/sub", "A subproject", nil); err != nil {
		t.Fatalf("CreateProject sub: %v", err)
	}
	if err := d.CreateTicket("proj/sub/ticket", "A ticket", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "proj", "sub", "ticket")) {
		t.Error("expected .tickets/proj/sub/ticket/ to be created")
	}
}

func TestCreateTicket_NoDirectoryOnFailure(t *testing.T) {
	d, ticketsDir, _ := openTestDB(t)
	// Attempt to create a ticket whose parent project doesn't exist — should fail.
	err := d.CreateTicket("nonexistent-proj/ticket", "A ticket", nil)
	if err == nil {
		t.Fatal("expected error for ticket with missing parent project, got nil")
	}
	if dirExists(filepath.Join(ticketsDir, "nonexistent-proj", "ticket")) {
		t.Error("directory should not be created when the transaction fails")
	}
}
