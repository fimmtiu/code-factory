package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary bare repo and a worktree clone with several
// commits on a feature branch forked from "main". Returns the worktree path.
func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	bare := filepath.Join(dir, "bare.git")
	wt := filepath.Join(dir, "worktree")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}

	// Init bare repo.
	run(dir, "init", "--bare", bare)

	// Clone and set up main branch with an initial commit.
	run(dir, "clone", bare, wt)
	run(wt, "checkout", "-b", "main")
	os.WriteFile(filepath.Join(wt, "README.md"), []byte("# Hello\n"), 0644)
	run(wt, "add", "README.md")
	run(wt, "commit", "-m", "initial commit")
	run(wt, "push", "-u", "origin", "main")

	// Create a feature branch with three commits.
	run(wt, "checkout", "-b", "feature")
	for i, msg := range []string{"first feature commit", "second feature commit", "third feature commit"} {
		name := filepath.Join(wt, "file"+string(rune('0'+i))+".txt")
		os.WriteFile(name, []byte(msg+"\n"), 0644)
		run(wt, "add", ".")
		run(wt, "commit", "-m", msg)
	}

	return wt
}

func TestFetchCommitList(t *testing.T) {
	wt := setupTestRepo(t)

	commits, err := fetchCommitList(wt, 10)
	if err != nil {
		t.Fatalf("fetchCommitList: %v", err)
	}

	// Should have 4 commits: 3 feature + 1 initial (no merges).
	if len(commits) != 4 {
		t.Fatalf("expected 4 commits, got %d: %+v", len(commits), commits)
	}

	// Newest first.
	if commits[0].Message != "third feature commit" {
		t.Errorf("first commit message: got %q, want %q", commits[0].Message, "third feature commit")
	}
	if commits[3].Message != "initial commit" {
		t.Errorf("last commit message: got %q, want %q", commits[3].Message, "initial commit")
	}

	// Hashes should be full-length (40 hex chars).
	for _, c := range commits {
		if len(c.Hash) != 40 {
			t.Errorf("expected 40-char hash, got %d chars: %q", len(c.Hash), c.Hash)
		}
	}
}

func TestFetchCommitList_MaxCommits(t *testing.T) {
	wt := setupTestRepo(t)

	commits, err := fetchCommitList(wt, 2)
	if err != nil {
		t.Fatalf("fetchCommitList: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(commits))
	}
	if commits[0].Message != "third feature commit" {
		t.Errorf("first commit: got %q, want %q", commits[0].Message, "third feature commit")
	}
}

func TestFetchForkPoint(t *testing.T) {
	wt := setupTestRepo(t)

	hash, err := fetchForkPoint(wt, "main")
	if err != nil {
		t.Fatalf("fetchForkPoint: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %q", len(hash), hash)
	}

	// The fork point should be the "initial commit" hash.
	commits, _ := fetchCommitList(wt, 10)
	initialHash := commits[len(commits)-1].Hash
	if hash != initialHash {
		t.Errorf("fork point %q should equal initial commit %q", hash, initialHash)
	}
}

func TestFetchShowStat(t *testing.T) {
	wt := setupTestRepo(t)

	commits, _ := fetchCommitList(wt, 10)
	stat, err := fetchShowStat(wt, commits[0].Hash)
	if err != nil {
		t.Fatalf("fetchShowStat: %v", err)
	}
	if stat == "" {
		t.Error("expected non-empty stat output")
	}
	// Should contain the commit message and file change summary.
	if !strings.Contains(stat, "third feature commit") {
		t.Errorf("stat should contain commit message, got:\n%s", stat)
	}
}

func TestFetchDiff(t *testing.T) {
	wt := setupTestRepo(t)

	commits, _ := fetchCommitList(wt, 10)
	forkPoint := commits[len(commits)-1].Hash
	head := commits[0].Hash

	diff, err := fetchDiff(wt, forkPoint, head)
	if err != nil {
		t.Fatalf("fetchDiff: %v", err)
	}
	if diff == "" {
		t.Error("expected non-empty diff")
	}
	// The diff should mention the files added in the feature commits.
	if !strings.Contains(diff, "file0.txt") {
		t.Errorf("diff should contain file0.txt, got:\n%s", diff)
	}
}

func TestFetchDiff_Uncommitted(t *testing.T) {
	wt := setupTestRepo(t)

	// Create an uncommitted change.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("modified\n"), 0644)

	diff, err := fetchDiff(wt, UncommittedRef, "")
	if err != nil {
		t.Fatalf("fetchDiff uncommitted: %v", err)
	}
	if !strings.Contains(diff, "modified") {
		t.Errorf("uncommitted diff should contain 'modified', got:\n%s", diff)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	wt := setupTestRepo(t)

	// Clean state: no uncommitted changes.
	has, err := hasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("hasUncommittedChanges: %v", err)
	}
	if has {
		t.Error("expected no uncommitted changes in clean repo")
	}

	// Modify a file.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("dirty\n"), 0644)

	has, err = hasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("hasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes after modifying a file")
	}
}

func TestHasUncommittedChanges_UntrackedFile(t *testing.T) {
	wt := setupTestRepo(t)

	// Add an untracked file.
	os.WriteFile(filepath.Join(wt, "untracked.txt"), []byte("new\n"), 0644)

	has, err := hasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("hasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes with untracked file")
	}
}
