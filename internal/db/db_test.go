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
