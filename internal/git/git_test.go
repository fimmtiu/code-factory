package git

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

	commits, err := FetchCommitList(wt, 10)
	if err != nil {
		t.Fatalf("FetchCommitList: %v", err)
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

	commits, err := FetchCommitList(wt, 2)
	if err != nil {
		t.Fatalf("FetchCommitList: %v", err)
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

	hash, err := FetchForkPoint(wt, "main")
	if err != nil {
		t.Fatalf("FetchForkPoint: %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("expected 40-char hash, got %d chars: %q", len(hash), hash)
	}

	// The fork point should be the "initial commit" hash.
	commits, _ := FetchCommitList(wt, 10)
	initialHash := commits[len(commits)-1].Hash
	if hash != initialHash {
		t.Errorf("fork point %q should equal initial commit %q", hash, initialHash)
	}
}

func TestFetchShowStat(t *testing.T) {
	wt := setupTestRepo(t)

	commits, _ := FetchCommitList(wt, 10)
	stat, err := FetchShowStat(wt, commits[0].Hash)
	if err != nil {
		t.Fatalf("FetchShowStat: %v", err)
	}
	if stat == "" {
		t.Error("expected non-empty stat output")
	}
	// With --format=, the commit message header is suppressed, but file
	// change summary should still be present.
	if !strings.Contains(stat, "file2.txt") {
		t.Errorf("stat should contain file change info, got:\n%s", stat)
	}
}

func TestFetchShowStat_Uncommitted(t *testing.T) {
	wt := setupTestRepo(t)

	// Create an uncommitted change.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("modified\n"), 0644)

	stat, err := FetchShowStat(wt, UncommittedHash)
	if err != nil {
		t.Fatalf("FetchShowStat uncommitted: %v", err)
	}
	if !strings.Contains(stat, "file0.txt") {
		t.Errorf("uncommitted stat should mention file0.txt, got:\n%s", stat)
	}
}

func TestFetchDiff(t *testing.T) {
	wt := setupTestRepo(t)

	commits, _ := FetchCommitList(wt, 10)
	// Use the second feature commit as start and the newest as end.
	// Avoid using the root commit, since <root>^ has no parent.
	start := commits[1] // "second feature commit"
	end := commits[0]   // "third feature commit"

	d, err := FetchDiff(wt, start, end)
	if err != nil {
		t.Fatalf("FetchDiff: %v", err)
	}
	if d == "" {
		t.Error("expected non-empty diff")
	}
	// The diff from second^ to third should include file2.txt (added in third commit).
	if !strings.Contains(d, "file2.txt") {
		t.Errorf("diff should contain file2.txt, got:\n%s", d)
	}
}

func TestFetchDiff_Uncommitted(t *testing.T) {
	wt := setupTestRepo(t)

	// Create an uncommitted change.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("modified\n"), 0644)

	uncommitted := CommitEntry{Hash: UncommittedHash, Message: "Uncommitted changes"}
	d, err := FetchDiff(wt, uncommitted, uncommitted)
	if err != nil {
		t.Fatalf("FetchDiff uncommitted: %v", err)
	}
	if !strings.Contains(d, "modified") {
		t.Errorf("uncommitted diff should contain 'modified', got:\n%s", d)
	}
}

func TestFetchDiff_RangeToUncommitted(t *testing.T) {
	wt := setupTestRepo(t)

	// Create an uncommitted change.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("modified\n"), 0644)

	commits, _ := FetchCommitList(wt, 10)
	start := commits[0]
	uncommitted := CommitEntry{Hash: UncommittedHash, Message: "Uncommitted changes"}

	d, err := FetchDiff(wt, start, uncommitted)
	if err != nil {
		t.Fatalf("FetchDiff range to uncommitted: %v", err)
	}
	if !strings.Contains(d, "modified") {
		t.Errorf("diff should contain 'modified', got:\n%s", d)
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	wt := setupTestRepo(t)

	// Clean state: no uncommitted changes.
	has, err := HasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if has {
		t.Error("expected no uncommitted changes in clean repo")
	}

	// Modify a file.
	os.WriteFile(filepath.Join(wt, "file0.txt"), []byte("dirty\n"), 0644)

	has, err = HasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if !has {
		t.Error("expected uncommitted changes after modifying a file")
	}
}

func TestHasUncommittedChanges_UntrackedFile(t *testing.T) {
	wt := setupTestRepo(t)

	// Add an untracked file. Untracked files don't count: `git diff`
	// won't include them, so the "Uncommitted changes" pseudo-commit
	// would render as empty if it were offered.
	os.WriteFile(filepath.Join(wt, "untracked.txt"), []byte("new\n"), 0644)

	has, err := HasUncommittedChanges(wt)
	if err != nil {
		t.Fatalf("HasUncommittedChanges: %v", err)
	}
	if has {
		t.Error("untracked-only worktree should not report uncommitted changes")
	}
}

func TestIsWorktreeCleanForScope(t *testing.T) {
	mkdirAll := func(t *testing.T, wt, rel string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(wt, rel), 0755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
	}
	write := func(t *testing.T, wt, rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(wt, rel), []byte(body), 0644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	t.Run("empty scope falls back to whole-tree check", func(t *testing.T) {
		wt := setupTestRepo(t)
		write(t, wt, "sibling.txt", "untracked\n")

		clean, err := IsWorktreeCleanForScope(wt, nil)
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if clean {
			t.Error("empty scope should be dirty when any untracked file exists")
		}
	})

	t.Run("untracked file outside scope is ignored", func(t *testing.T) {
		wt := setupTestRepo(t)
		mkdirAll(t, wt, "src/mine")
		mkdirAll(t, wt, "src/sibling-gem/lib")
		write(t, wt, "src/sibling-gem/Gemfile", "source 'rubygems'\n")
		write(t, wt, "src/sibling-gem/lib/x.rb", "module X; end\n")

		clean, err := IsWorktreeCleanForScope(wt, []string{"src/mine/"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if !clean {
			t.Error("expected clean: untracked files are outside scope")
		}
	})

	t.Run("untracked file inside scope is dirty", func(t *testing.T) {
		wt := setupTestRepo(t)
		mkdirAll(t, wt, "src/mine")
		write(t, wt, "src/mine/forgot.rb", "module Forgot; end\n")

		clean, err := IsWorktreeCleanForScope(wt, []string{"src/mine/"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if clean {
			t.Error("expected dirty: untracked file is inside scope")
		}
	})

	t.Run("modified tracked file outside scope is ignored", func(t *testing.T) {
		wt := setupTestRepo(t)
		// file0.txt was committed on the feature branch by setupTestRepo.
		write(t, wt, "file0.txt", "modified by sibling staging\n")

		clean, err := IsWorktreeCleanForScope(wt, []string{"src/mine/"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if !clean {
			t.Error("expected clean: modified tracked file is outside scope")
		}
	})

	t.Run("modified tracked file inside scope is dirty", func(t *testing.T) {
		wt := setupTestRepo(t)
		write(t, wt, "file0.txt", "modified content\n")

		clean, err := IsWorktreeCleanForScope(wt, []string{"file0.txt"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if clean {
			t.Error("expected dirty: modified tracked file is in scope")
		}
	})

	t.Run("exact-file scope entry matches the file but not a sibling with shared prefix", func(t *testing.T) {
		wt := setupTestRepo(t)
		// Create one tracked file matching the scope path exactly, and an
		// untracked file whose path is a prefix-extension of the scope path
		// but a different file ("file0.txt" vs "file0.txt.bak").
		write(t, wt, "file0.txt.bak", "backup\n")

		clean, err := IsWorktreeCleanForScope(wt, []string{"file0.txt"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if !clean {
			t.Error("expected clean: 'file0.txt' scope must not match 'file0.txt.bak'")
		}
	})

	t.Run("unmerged entries are always dirty regardless of scope", func(t *testing.T) {
		wt := setupTestRepo(t)
		// Create a conflict on file0.txt (which is outside this unit's scope)
		// by making the feature branch and main diverge on it, then merging.
		run := func(dir string, expectFail bool, args ...string) {
			t.Helper()
			cmd := exec.Command("git", args...)
			cmd.Dir = dir
			cmd.Env = append(os.Environ(),
				"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@test.com",
				"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@test.com",
			)
			out, err := cmd.CombinedOutput()
			if err != nil && !expectFail {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		// On feature: modify file0.txt and commit.
		write(t, wt, "file0.txt", "feature edit\n")
		run(wt, false, "commit", "-am", "feature edit")
		// On main: modify file0.txt differently and commit.
		run(wt, false, "checkout", "main")
		write(t, wt, "file0.txt", "main edit\n")
		run(wt, false, "add", "file0.txt")
		run(wt, false, "commit", "-m", "main edit")
		// Merge feature into main → conflict (git exits non-zero on conflict).
		run(wt, true, "merge", "feature")

		// Scope is something totally unrelated to file0.txt, but the
		// unmerged state must still surface as dirty.
		clean, err := IsWorktreeCleanForScope(wt, []string{"src/mine/"})
		if err != nil {
			t.Fatalf("IsWorktreeCleanForScope: %v", err)
		}
		if clean {
			t.Error("expected dirty: unmerged entries must always count, even outside scope")
		}
	})
}

func TestPathInScope(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		scope []string
		want  bool
	}{
		{"exact match", "lib/foo.rb", []string{"lib/foo.rb"}, true},
		{"dir prefix with trailing slash", "lib/foo/bar.rb", []string{"lib/foo/"}, true},
		{"dir prefix without trailing slash", "lib/foo/bar.rb", []string{"lib/foo"}, true},
		{"prefix-extension is not a match", "lib/foo_bar.rb", []string{"lib/foo"}, false},
		{"unrelated path", "src/other.rb", []string{"lib/foo/"}, false},
		{"multiple entries, second matches", "spec/x_spec.rb", []string{"lib/foo/", "spec/"}, true},
		{"empty scope entry is ignored", "anything.rb", []string{""}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathInScope(tc.path, tc.scope); got != tc.want {
				t.Errorf("pathInScope(%q, %v) = %v, want %v", tc.path, tc.scope, got, tc.want)
			}
		})
	}
}
