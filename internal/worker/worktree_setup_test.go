package worker

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a repo with one commit and a linked worktree under
// .code-factory/t/worktree, and returns the repo root and the worktree path.
func setupTestRepo(t *testing.T) (string, string) {
	t.Helper()
	repo := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	writeFile(t, filepath.Join(repo, ".gitignore"), ".env\n.code-factory/\n")
	run("add", ".gitignore")
	run("commit", "-m", "initial commit")

	worktree := filepath.Join(repo, ".code-factory", "t", "worktree")
	run("worktree", "add", worktree, "-b", "t")
	return repo, worktree
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestPrepareWorktree_NothingConfigured(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	logPath := filepath.Join(t.TempDir(), "setup.log")

	if err := prepareWorktree(context.Background(), repo, worktree, "", logPath); err != nil {
		t.Fatalf("prepareWorktree: %v", err)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("setup log exists, want no log when nothing is configured")
	}
}

func TestPrepareWorktree_CopiesIgnoredIncludedFilesOnly(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	writeFile(t, filepath.Join(repo, ".worktreeinclude"), ".env\nnotes.txt\n")
	writeFile(t, filepath.Join(repo, ".env"), "SECRET=1\n")
	writeFile(t, filepath.Join(repo, "notes.txt"), "not ignored\n")
	logPath := filepath.Join(t.TempDir(), "setup.log")

	if err := prepareWorktree(context.Background(), repo, worktree, "", logPath); err != nil {
		t.Fatalf("prepareWorktree: %v", err)
	}
	if got := readFile(t, filepath.Join(worktree, ".env")); got != "SECRET=1\n" {
		t.Errorf(".env in worktree = %q, want %q", got, "SECRET=1\n")
	}
	if _, err := os.Stat(filepath.Join(worktree, "notes.txt")); !os.IsNotExist(err) {
		t.Errorf("notes.txt was copied, but git does not ignore it")
	}
	if log := readFile(t, logPath); !strings.Contains(log, "copied .env") {
		t.Errorf("setup log = %q, want it to name the copied file", log)
	}
}

func TestPrepareWorktree_KeepsExistingWorktreeFiles(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	writeFile(t, filepath.Join(repo, ".worktreeinclude"), ".env\n")
	writeFile(t, filepath.Join(repo, ".env"), "FROM_REPO=1\n")
	writeFile(t, filepath.Join(worktree, ".env"), "FROM_WORKTREE=1\n")

	if err := prepareWorktree(context.Background(), repo, worktree, "", filepath.Join(t.TempDir(), "setup.log")); err != nil {
		t.Fatalf("prepareWorktree: %v", err)
	}
	if got := readFile(t, filepath.Join(worktree, ".env")); got != "FROM_WORKTREE=1\n" {
		t.Errorf(".env in worktree = %q, want the worktree's own copy", got)
	}
}

func TestPrepareWorktree_RunsSetupInWorktreeOnce(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	logDir := t.TempDir()
	setup := "pwd > setup-ran; echo setup output"

	if err := prepareWorktree(context.Background(), repo, worktree, setup, filepath.Join(logDir, "1.log")); err != nil {
		t.Fatalf("prepareWorktree: %v", err)
	}
	ranIn := strings.TrimSpace(readFile(t, filepath.Join(worktree, "setup-ran")))
	wantDir, _ := filepath.EvalSymlinks(worktree)
	if gotDir, _ := filepath.EvalSymlinks(ranIn); gotDir != wantDir {
		t.Errorf("setup ran in %q, want %q", ranIn, worktree)
	}
	if log := readFile(t, filepath.Join(logDir, "1.log")); !strings.Contains(log, "setup output") {
		t.Errorf("setup log = %q, want the command output", log)
	}

	if err := os.Remove(filepath.Join(worktree, "setup-ran")); err != nil {
		t.Fatal(err)
	}
	if err := prepareWorktree(context.Background(), repo, worktree, setup, filepath.Join(logDir, "2.log")); err != nil {
		t.Fatalf("second prepareWorktree: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "setup-ran")); !os.IsNotExist(err) {
		t.Errorf("setup ran a second time with the same command")
	}

	if err := prepareWorktree(context.Background(), repo, worktree, setup+"; true", filepath.Join(logDir, "3.log")); err != nil {
		t.Fatalf("prepareWorktree with changed command: %v", err)
	}
	if _, err := os.Stat(filepath.Join(worktree, "setup-ran")); err != nil {
		t.Errorf("setup did not run again after the command changed: %v", err)
	}
}

func TestPrepareWorktree_FailedSetupReturnsErrorAndRetries(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	logDir := t.TempDir()
	setup := "echo broken >&2; test -f fixed"

	err := prepareWorktree(context.Background(), repo, worktree, setup, filepath.Join(logDir, "1.log"))
	if err == nil {
		t.Fatal("prepareWorktree returned nil, want an error from the failed setup command")
	}
	log := readFile(t, filepath.Join(logDir, "1.log"))
	if !strings.Contains(log, "broken") || !strings.Contains(log, "SETUP FAILED") {
		t.Errorf("setup log = %q, want the stderr output and a failure line", log)
	}

	writeFile(t, filepath.Join(worktree, "fixed"), "")
	if err := prepareWorktree(context.Background(), repo, worktree, setup, filepath.Join(logDir, "2.log")); err != nil {
		t.Errorf("prepareWorktree after fix: %v", err)
	}
}

func TestPrepareWorktree_CancelStopsSetup(t *testing.T) {
	repo, worktree := setupTestRepo(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := prepareWorktree(ctx, repo, worktree, "sleep 30", filepath.Join(t.TempDir(), "setup.log"))
	if err == nil {
		t.Fatal("prepareWorktree returned nil, want an error after cancel")
	}
}
