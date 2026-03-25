package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fimmtiu/tickets/internal/storage"
)

// makeTempRepo creates a temp dir with a .git subdirectory to simulate a git repo root.
func makeTempRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	return root
}

// ===== FindRepoRoot =====

func TestFindRepoRoot_FromRoot(t *testing.T) {
	root := makeTempRepo(t)
	got, err := storage.FindRepoRoot(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("expected %q, got %q", root, got)
	}
}

func TestFindRepoRoot_FromSubdir(t *testing.T) {
	root := makeTempRepo(t)
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}
	got, err := storage.FindRepoRoot(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("expected %q, got %q", root, got)
	}
}

func TestFindRepoRoot_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := storage.FindRepoRoot(dir)
	if err == nil {
		t.Fatal("expected error when no .git found, got nil")
	}
}

// ===== TicketsDirPath =====

func TestTicketsDirPath(t *testing.T) {
	root := "/some/repo"
	got := storage.TicketsDirPath(root)
	want := filepath.Join(root, ".tickets")
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

// ===== InitTicketsDir =====

func TestInitTicketsDir_CreatesDir(t *testing.T) {
	root := makeTempRepo(t)
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ticketsDir := filepath.Join(root, ".tickets")
	info, err := os.Stat(ticketsDir)
	if err != nil {
		t.Fatalf("expected .tickets to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected .tickets to be a directory")
	}
}

func TestInitTicketsDir_CreatesSettings(t *testing.T) {
	root := makeTempRepo(t)
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	settingsPath := filepath.Join(root, ".tickets", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("expected settings.json to exist: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}
}

func TestInitTicketsDir_Idempotent(t *testing.T) {
	root := makeTempRepo(t)
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("first call: %v", err)
	}
	settingsPath := filepath.Join(root, ".tickets", "settings.json")
	custom := []byte(`{"stale_threshold_minutes":99,"exit_after_minutes":120}`)
	if err := os.WriteFile(settingsPath, custom, 0644); err != nil {
		t.Fatalf("failed to write custom settings: %v", err)
	}
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("second call: %v", err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("could not read settings after second call: %v", err)
	}
	if string(data) != string(custom) {
		t.Errorf("second InitTicketsDir call overwrote existing settings; got %q", data)
	}
}

// ===== TicketDirPath / TicketWorktreePath =====

func TestTicketDirPath(t *testing.T) {
	ticketsDir := "/repo/.tickets"
	got := storage.TicketDirPath(ticketsDir, "my-project/fix-bug")
	want := filepath.Join(ticketsDir, "my-project", "fix-bug")
	if got != want {
		t.Errorf("TicketDirPath: got %q, want %q", got, want)
	}
}

func TestTicketWorktreePath(t *testing.T) {
	ticketDir := "/repo/.tickets/my-project/fix-bug"
	got := storage.TicketWorktreePath(ticketDir)
	want := filepath.Join(ticketDir, "worktree")
	if got != want {
		t.Errorf("TicketWorktreePath: got %q, want %q", got, want)
	}
}
