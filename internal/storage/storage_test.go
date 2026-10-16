package storage_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fimmtiu/code-factory/internal/config"
	"github.com/fimmtiu/code-factory/internal/storage"
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

// ===== FindMainRepoRoot =====

// makeTempWorktree creates a main repo and a linked worktree outside it, with
// the same .git file and commondir layout that `git worktree add` creates.
func makeTempWorktree(t *testing.T, gitdirLine string) (mainRoot, worktree string) {
	t.Helper()
	mainRoot = makeTempRepo(t)
	adminDir := filepath.Join(mainRoot, ".git", "worktrees", "wt")
	if err := os.MkdirAll(adminDir, 0755); err != nil {
		t.Fatalf("failed to create worktree admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "commondir"), []byte("../..\n"), 0644); err != nil {
		t.Fatalf("failed to write commondir: %v", err)
	}
	worktree = t.TempDir()
	if gitdirLine == "" {
		gitdirLine = "gitdir: " + adminDir
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte(gitdirLine+"\n"), 0644); err != nil {
		t.Fatalf("failed to write .git file: %v", err)
	}
	return mainRoot, worktree
}

func TestFindMainRepoRoot_FromRepo(t *testing.T) {
	root := makeTempRepo(t)
	got, err := storage.FindMainRepoRoot(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != root {
		t.Errorf("expected %q, got %q", root, got)
	}
}

func TestFindMainRepoRoot_FromWorktreeSubdir(t *testing.T) {
	mainRoot, worktree := makeTempWorktree(t, "")
	sub := filepath.Join(worktree, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("failed to create subdirs: %v", err)
	}
	got, err := storage.FindMainRepoRoot(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != mainRoot {
		t.Errorf("expected %q, got %q", mainRoot, got)
	}
}

func TestFindMainRepoRoot_RelativeGitdir(t *testing.T) {
	mainRoot := makeTempRepo(t)
	adminDir := filepath.Join(mainRoot, ".git", "worktrees", "wt")
	if err := os.MkdirAll(adminDir, 0755); err != nil {
		t.Fatalf("failed to create worktree admin dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminDir, "commondir"), []byte("../..\n"), 0644); err != nil {
		t.Fatalf("failed to write commondir: %v", err)
	}
	worktree := filepath.Join(mainRoot, "nested", "wt")
	if err := os.MkdirAll(worktree, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: ../../.git/worktrees/wt\n"), 0644); err != nil {
		t.Fatalf("failed to write .git file: %v", err)
	}
	got, err := storage.FindMainRepoRoot(worktree)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != mainRoot {
		t.Errorf("expected %q, got %q", mainRoot, got)
	}
}

func TestFindMainRepoRoot_NoCommondir(t *testing.T) {
	gitDir := t.TempDir()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: "+gitDir+"\n"), 0644); err != nil {
		t.Fatalf("failed to write .git file: %v", err)
	}
	got, err := storage.FindMainRepoRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != dir {
		t.Errorf("expected %q, got %q", dir, got)
	}
}

func TestFindMainRepoRoot_MalformedGitFile(t *testing.T) {
	_, worktree := makeTempWorktree(t, "garbage")
	if _, err := storage.FindMainRepoRoot(worktree); err == nil {
		t.Fatal("expected error for malformed .git file, got nil")
	}
}

func TestFindMainRepoRoot_NotFound(t *testing.T) {
	if _, err := storage.FindMainRepoRoot(t.TempDir()); err == nil {
		t.Fatal("expected error when no .git found, got nil")
	}
}

// ===== TicketsDirPath =====

func TestTicketsDirPath(t *testing.T) {
	root := "/some/repo"
	got := storage.TicketsDirPath(root)
	want := filepath.Join(root, ".code-factory")
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
	ticketsDir := filepath.Join(root, ".code-factory")
	info, err := os.Stat(ticketsDir)
	if err != nil {
		t.Fatalf("expected .code-factory to exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("expected .code-factory to be a directory")
	}
}

func TestInitTicketsDir_CreatesSettings(t *testing.T) {
	root := makeTempRepo(t)
	if err := storage.InitTicketsDir(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ticketsDir := storage.TicketsDirPath(root)
	data, err := os.ReadFile(config.Path(ticketsDir))
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
	settingsPath := config.Path(storage.TicketsDirPath(root))
	custom := []byte(`{"stale_threshold_minutes":99}`)
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
	ticketsDir := "/repo/.code-factory"
	got := storage.TicketDirPath(ticketsDir, "my-project/fix-bug")
	want := filepath.Join(ticketsDir, "my-project", "fix-bug")
	if got != want {
		t.Errorf("TicketDirPath: got %q, want %q", got, want)
	}
}

func TestTicketWorktreePath(t *testing.T) {
	ticketDir := "/repo/.code-factory/my-project/fix-bug"
	got := storage.TicketWorktreePath(ticketDir)
	want := filepath.Join(ticketDir, "worktree")
	if got != want {
		t.Errorf("TicketWorktreePath: got %q, want %q", got, want)
	}
}
