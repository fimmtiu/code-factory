package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fimmtiu/tickets/internal/db"
)

// openTestDB creates a temporary directory and opens a fresh DB in it.
// It returns both the DB handle and the ticketsDir path.
func openTestDB(t *testing.T) (*db.DB, string) {
	t.Helper()
	dir := t.TempDir()
	d, err := db.Open(dir, dir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d, dir
}

// dirExists reports whether path is an existing directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// ===== CreateProject directory creation =====

func TestCreateProject_CreatesDirectory(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("my-proj", "A project", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if !dirExists(filepath.Join(ticketsDir, "my-proj")) {
		t.Error("expected .tickets/my-proj/ to be created")
	}
}

func TestCreateProject_CreatesNestedDirectory(t *testing.T) {
	d, ticketsDir := openTestDB(t)
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

// ===== CreateProject parent FK =====

func TestCreateProject_SubprojectRecordsParent(t *testing.T) {
	d, _ := openTestDB(t)
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
	d, _ := openTestDB(t)
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
	d, _ := openTestDB(t)
	err := d.CreateProject("nonexistent/child", "A child project", nil)
	if err == nil {
		t.Error("expected error when parent project does not exist, got nil")
	}
}

// ===== CreateTicket directory creation =====

func TestCreateTicket_CreatesDirectory(t *testing.T) {
	d, ticketsDir := openTestDB(t)
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
	d, ticketsDir := openTestDB(t)
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
	d, ticketsDir := openTestDB(t)
	// Attempt to create a ticket whose parent project doesn't exist — should fail.
	err := d.CreateTicket("nonexistent-proj/ticket", "A ticket", nil)
	if err == nil {
		t.Fatal("expected error for ticket with missing parent project, got nil")
	}
	if dirExists(filepath.Join(ticketsDir, "nonexistent-proj", "ticket")) {
		t.Error("directory should not be created when the transaction fails")
	}
}
