package gitutil_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/gitutil"
)

// initTestRepo creates an isolated git repo in a temp directory with an initial commit.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("setup command %v failed: %v\n%s", args, err, out)
		}
	}

	// Create a file and make an initial commit so HEAD exists.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}

	addOut, err := exec.Command("git", "-C", dir, "add", "README.md").CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, addOut)
	}

	commitOut, err := exec.Command("git", "-C", dir, "commit", "-m", "initial commit").CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, commitOut)
	}

	return dir
}

// currentBranch returns the current branch name in the given repo.
func currentBranch(t *testing.T, repoRoot string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// branchExists checks whether a named branch exists in the repo.
func branchExists(t *testing.T, repoRoot, branch string) bool {
	t.Helper()
	err := exec.Command("git", "-C", repoRoot, "rev-parse", "--verify", branch).Run()
	return err == nil
}

func TestCreateWorktree(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	branchName := "my-feature"
	worktreePath := filepath.Join(dir, ".tickets", "my-feature", "worktree")
	err := client.CreateWorktree(dir, worktreePath, branchName)
	if err != nil {
		t.Fatalf("CreateWorktree returned error: %v", err)
	}

	// The worktree directory should exist.
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("worktree directory %q was not created", worktreePath)
	}

	// The branch should exist.
	if !branchExists(t, dir, branchName) {
		t.Fatalf("branch %q was not created", branchName)
	}
}

func TestCreateWorktreeWithSlashIdentifier(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	branchName := "project/fix-bug"
	worktreePath := filepath.Join(dir, ".tickets", "project", "fix-bug", "worktree")
	err := client.CreateWorktree(dir, worktreePath, branchName)
	if err != nil {
		t.Fatalf("CreateWorktree returned error for slash identifier: %v", err)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatalf("worktree directory %q was not created", worktreePath)
	}

	if !branchExists(t, dir, branchName) {
		t.Fatalf("branch %q was not created", branchName)
	}
}

func TestMergeBranch(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	// Determine the default branch name (could be "main" or "master").
	baseBranch := currentBranch(t, dir)

	// Create a feature branch from base.
	featureBranch := "feature-branch"
	out, err := exec.Command("git", "-C", dir, "checkout", "-b", featureBranch).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to create feature branch: %v\n%s", err, out)
	}

	// Add a commit on the feature branch.
	filePath := filepath.Join(dir, "feature.txt")
	if err := os.WriteFile(filePath, []byte("feature work\n"), 0644); err != nil {
		t.Fatalf("failed to write feature file: %v", err)
	}
	out, err = exec.Command("git", "-C", dir, "add", "feature.txt").CombinedOutput()
	if err != nil {
		t.Fatalf("git add failed: %v\n%s", err, out)
	}
	out, err = exec.Command("git", "-C", dir, "commit", "-m", "feature commit").CombinedOutput()
	if err != nil {
		t.Fatalf("git commit failed: %v\n%s", err, out)
	}

	// Go back to base branch before merging.
	out, err = exec.Command("git", "-C", dir, "checkout", baseBranch).CombinedOutput()
	if err != nil {
		t.Fatalf("failed to checkout base branch: %v\n%s", err, out)
	}

	// Merge featureBranch into the currently-checked-out baseBranch.
	err = client.MergeBranch(dir, featureBranch)
	if err != nil {
		t.Fatalf("MergeBranch returned error: %v", err)
	}

	// The merged file should now be present on baseBranch.
	mergedFile := filepath.Join(dir, "feature.txt")
	if _, err := os.Stat(mergedFile); os.IsNotExist(err) {
		t.Fatal("feature.txt not present after merge")
	}

	// We should still be on baseBranch.
	branch := currentBranch(t, dir)
	if branch != baseBranch {
		t.Fatalf("expected to be on %q after merge, got %q", baseBranch, branch)
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := initTestRepo(t)
	client := gitutil.NewRealGitClient()

	branchName := "remove-me"
	worktreePath := filepath.Join(dir, ".tickets", "remove-me", "worktree")

	// Create the worktree first.
	if err := client.CreateWorktree(dir, worktreePath, branchName); err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Fatal("worktree was not created before removal test")
	}

	// Now remove it.
	if err := client.RemoveWorktree(dir, worktreePath, branchName); err != nil {
		t.Fatalf("RemoveWorktree returned error: %v", err)
	}

	// The worktree directory should be gone.
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Fatalf("worktree directory %q still exists after removal", worktreePath)
	}
}
