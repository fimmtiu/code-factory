package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
)

// --- helpers ---

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
	// Use a temp dir with no .git anywhere above it (it is the root of a fresh temp tree).
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
	settingsPath := filepath.Join(root, ".tickets", ".settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("expected .settings.json to exist: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v", err)
	}
}

func TestInitTicketsDir_Idempotent(t *testing.T) {
	root := makeTempRepo(t)
	// First call
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("first call: %v", err)
	}
	// Overwrite settings with custom content to ensure second call does not clobber
	settingsPath := filepath.Join(root, ".tickets", ".settings.json")
	custom := []byte(`{"stale_threshold_minutes":99,"exit_after_minutes":120}`)
	if err := os.WriteFile(settingsPath, custom, 0644); err != nil {
		t.Fatalf("failed to write custom settings: %v", err)
	}
	// Second call should not overwrite existing settings
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

// ===== ProjectMetaPath / TicketDirPath / TicketMetaPath / TicketWorktreePath =====

func TestProjectMetaPath(t *testing.T) {
	projectDir := "/repo/.tickets/my-project"
	got := storage.ProjectMetaPath(projectDir)
	want := filepath.Join(projectDir, "project.json")
	if got != want {
		t.Errorf("ProjectMetaPath: got %q, want %q", got, want)
	}
}

func TestTicketDirPath(t *testing.T) {
	ticketsDir := "/repo/.tickets"
	got := storage.TicketDirPath(ticketsDir, "my-project/fix-bug")
	want := filepath.Join(ticketsDir, "my-project", "fix-bug")
	if got != want {
		t.Errorf("TicketDirPath: got %q, want %q", got, want)
	}
}

func TestTicketMetaPath(t *testing.T) {
	ticketDir := "/repo/.tickets/my-project/fix-bug"
	got := storage.TicketMetaPath(ticketDir)
	want := filepath.Join(ticketDir, "ticket.json")
	if got != want {
		t.Errorf("TicketMetaPath: got %q, want %q", got, want)
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

// ===== TraverseAll =====

// buildTicketsDir constructs a .tickets/ directory structure for testing.
// It creates projects and tickets as described.
func buildTicketsDir(t *testing.T, ticketsDir string) {
	t.Helper()
	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	writeWU := func(path string, wu *models.WorkUnit) {
		t.Helper()
		data, err := json.MarshalIndent(wu, "", "  ")
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("MkdirAll dir: %v", err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}

	now := time.Now().UTC()

	// Top-level ticket: fix-bug/ticket.json
	writeWU(filepath.Join(ticketsDir, "fix-bug", "ticket.json"), &models.WorkUnit{
		Identifier:   "fix-bug",
		Description:  "Fix the bug",
		Status:       models.StatusOpen,
		Dependencies: []string{},
		LastUpdated:  now,
		IsProject:    false,
	})

	// Top-level project: my-feature/
	projDir := filepath.Join(ticketsDir, "my-feature")
	writeWU(filepath.Join(projDir, "project.json"), &models.WorkUnit{
		Identifier:   "my-feature",
		Description:  "My feature project",
		Status:       models.ProjectOpen,
		Dependencies: []string{},
		LastUpdated:  now,
		IsProject:    true,
	})

	// Ticket inside my-feature: my-feature/implement/ticket.json
	writeWU(filepath.Join(projDir, "implement", "ticket.json"), &models.WorkUnit{
		Identifier:   "my-feature/implement",
		Description:  "Implement the feature",
		Status:       models.StatusOpen,
		Dependencies: []string{},
		LastUpdated:  now,
		IsProject:    false,
	})

	// Nested project: my-feature/sub-task/
	subDir := filepath.Join(projDir, "sub-task")
	writeWU(filepath.Join(subDir, "project.json"), &models.WorkUnit{
		Identifier:   "my-feature/sub-task",
		Description:  "Sub-task project",
		Status:       models.ProjectOpen,
		Dependencies: []string{},
		LastUpdated:  now,
		IsProject:    true,
	})

	// Ticket inside nested project: my-feature/sub-task/do-thing/ticket.json
	writeWU(filepath.Join(subDir, "do-thing", "ticket.json"), &models.WorkUnit{
		Identifier:   "my-feature/sub-task/do-thing",
		Description:  "Do the thing",
		Status:       models.StatusOpen,
		Dependencies: []string{},
		LastUpdated:  now,
		IsProject:    false,
	})

	// Hidden directory should be ignored
	hiddenDir := filepath.Join(ticketsDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "ignored.json"), []byte(`{}`), 0644); err != nil {
		t.Fatalf("hidden file: %v", err)
	}
}

func TestTraverseAll_Basic(t *testing.T) {
	root := makeTempRepo(t)
	ticketsDir := filepath.Join(root, ".tickets")
	buildTicketsDir(t, ticketsDir)

	wus, err := storage.TraverseAll(ticketsDir)
	if err != nil {
		t.Fatalf("TraverseAll: %v", err)
	}

	// Build a map identifier -> *WorkUnit for easy lookup
	byID := make(map[string]*models.WorkUnit, len(wus))
	for _, wu := range wus {
		byID[wu.Identifier] = wu
	}

	// Expect exactly 5 entries
	if len(wus) != 5 {
		t.Errorf("expected 5 work units, got %d: %v", len(wus), identifiers(wus))
	}

	// fix-bug: top-level ticket, Parent=""
	if wu, ok := byID["fix-bug"]; !ok {
		t.Error("missing fix-bug")
	} else {
		if wu.IsProject {
			t.Error("fix-bug should not be a project")
		}
		if wu.Parent != "" {
			t.Errorf("fix-bug.Parent = %q; want \"\"", wu.Parent)
		}
	}

	// my-feature: top-level project, Parent=""
	if wu, ok := byID["my-feature"]; !ok {
		t.Error("missing my-feature")
	} else {
		if !wu.IsProject {
			t.Error("my-feature should be a project")
		}
		if wu.Parent != "" {
			t.Errorf("my-feature.Parent = %q; want \"\"", wu.Parent)
		}
	}

	// my-feature/implement: ticket inside my-feature, Parent="my-feature"
	if wu, ok := byID["my-feature/implement"]; !ok {
		t.Error("missing my-feature/implement")
	} else {
		if wu.IsProject {
			t.Error("my-feature/implement should not be a project")
		}
		if wu.Parent != "my-feature" {
			t.Errorf("my-feature/implement.Parent = %q; want \"my-feature\"", wu.Parent)
		}
	}

	// my-feature/sub-task: nested project, Parent="my-feature"
	if wu, ok := byID["my-feature/sub-task"]; !ok {
		t.Error("missing my-feature/sub-task")
	} else {
		if !wu.IsProject {
			t.Error("my-feature/sub-task should be a project")
		}
		if wu.Parent != "my-feature" {
			t.Errorf("my-feature/sub-task.Parent = %q; want \"my-feature\"", wu.Parent)
		}
	}

	// my-feature/sub-task/do-thing: ticket inside nested project, Parent="my-feature/sub-task"
	if wu, ok := byID["my-feature/sub-task/do-thing"]; !ok {
		t.Error("missing my-feature/sub-task/do-thing")
	} else {
		if wu.IsProject {
			t.Error("my-feature/sub-task/do-thing should not be a project")
		}
		if wu.Parent != "my-feature/sub-task" {
			t.Errorf("my-feature/sub-task/do-thing.Parent = %q; want \"my-feature/sub-task\"", wu.Parent)
		}
	}
}

func TestTraverseAll_EmptyDir(t *testing.T) {
	ticketsDir := t.TempDir()
	wus, err := storage.TraverseAll(ticketsDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wus) != 0 {
		t.Errorf("expected 0 work units, got %d", len(wus))
	}
}

// ===== ReadWorkUnit / WriteWorkUnit =====

func TestReadWriteWorkUnit_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ticket.json")

	original := &models.WorkUnit{
		Identifier:   "my-ticket",
		Description:  "A test ticket",
		Status:       models.StatusOpen,
		Dependencies: []string{"other-ticket"},
		LastUpdated:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		IsProject:    false,
	}

	if err := storage.WriteWorkUnit(path, original); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}

	got, err := storage.ReadWorkUnit(path)
	if err != nil {
		t.Fatalf("ReadWorkUnit: %v", err)
	}

	if got.Identifier != original.Identifier {
		t.Errorf("Identifier: got %q, want %q", got.Identifier, original.Identifier)
	}
	if got.Description != original.Description {
		t.Errorf("Description: got %q, want %q", got.Description, original.Description)
	}
	if got.Status != original.Status {
		t.Errorf("Status: got %q, want %q", got.Status, original.Status)
	}
	if len(got.Dependencies) != 1 || got.Dependencies[0] != "other-ticket" {
		t.Errorf("Dependencies: got %v, want %v", got.Dependencies, original.Dependencies)
	}
	if !got.LastUpdated.Equal(original.LastUpdated) {
		t.Errorf("LastUpdated: got %v, want %v", got.LastUpdated, original.LastUpdated)
	}
}

func TestWriteWorkUnit_Atomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ticket.json")

	wu := &models.WorkUnit{
		Identifier:  "atomic-test",
		Description: "Testing atomic write",
		Status:      models.StatusOpen,
		LastUpdated: time.Now().UTC(),
	}

	if err := storage.WriteWorkUnit(path, wu); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}

	// Verify no temp files were left behind
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected exactly 1 file in dir, got %d: %v", len(entries), names)
	}
}

func TestReadWorkUnit_NotFound(t *testing.T) {
	_, err := storage.ReadWorkUnit("/nonexistent/path/ticket.json")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// ===== CreateProjectDir =====

func TestCreateProjectDir_TopLevel(t *testing.T) {
	ticketsDir := t.TempDir()
	if err := storage.CreateProjectDir(ticketsDir, "my-feature"); err != nil {
		t.Fatalf("CreateProjectDir: %v", err)
	}

	projDir := filepath.Join(ticketsDir, "my-feature")
	info, err := os.Stat(projDir)
	if err != nil {
		t.Fatalf("project dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory at %s", projDir)
	}

	projectFile := filepath.Join(projDir, "project.json")
	_, err = os.Stat(projectFile)
	if err != nil {
		t.Fatalf("project.json does not exist: %v", err)
	}

	wu, err := storage.ReadWorkUnit(projectFile)
	if err != nil {
		t.Fatalf("ReadWorkUnit: %v", err)
	}
	if wu.Identifier != "my-feature" {
		t.Errorf("Identifier: got %q, want %q", wu.Identifier, "my-feature")
	}
	// Note: IsProject is json:"-" so it is not persisted; it is set by
	// TraverseAll based on whether the file is project.json. The raw
	// ReadWorkUnit call will return IsProject=false, which is expected.
}

func TestCreateProjectDir_Nested(t *testing.T) {
	ticketsDir := t.TempDir()
	if err := storage.CreateProjectDir(ticketsDir, "my-feature/sub-task"); err != nil {
		t.Fatalf("CreateProjectDir: %v", err)
	}

	projDir := filepath.Join(ticketsDir, "my-feature", "sub-task")
	info, err := os.Stat(projDir)
	if err != nil {
		t.Fatalf("nested project dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected directory at %s", projDir)
	}

	wu, err := storage.ReadWorkUnit(filepath.Join(projDir, "project.json"))
	if err != nil {
		t.Fatalf("ReadWorkUnit: %v", err)
	}
	if wu.Identifier != "my-feature/sub-task" {
		t.Errorf("Identifier: got %q, want %q", wu.Identifier, "my-feature/sub-task")
	}
}

// ===== helpers =====

func identifiers(wus []*models.WorkUnit) []string {
	ids := make([]string, len(wus))
	for i, wu := range wus {
		ids[i] = wu.Identifier
	}
	return ids
}
