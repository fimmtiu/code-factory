package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitClient defines the git operations needed by the tickets tool.
type GitClient interface {
	CreateWorktree(repoRoot, worktreePath, branchName string) error
	// MergeBranch merges fromBranch into whichever branch is currently checked
	// out at worktreeDir using --no-ff. worktreeDir must already be on the
	// desired target branch (e.g. the parent project's worktree, or repoRoot).
	MergeBranch(worktreeDir, fromBranch string) error
	RemoveWorktree(repoRoot, worktreePath, branchName string) error
	// GetHeadCommit returns the abbreviated HEAD commit hash for the git
	// repository rooted at path.
	GetHeadCommit(path string) (string, error)
}

// RealGitClient implements GitClient using actual git commands.
type RealGitClient struct{}

// NewRealGitClient returns a GitClient backed by real git.
func NewRealGitClient() GitClient {
	return &RealGitClient{}
}

// runGit runs a git command and returns a descriptive error on failure.
func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runGitOutput runs a git command and returns its trimmed stdout on success.
func runGitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s failed: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateWorktree creates a new git branch named branchName and a linked worktree
// at worktreePath. Intermediate directories are created as needed.
func (g *RealGitClient) CreateWorktree(repoRoot, worktreePath, branchName string) error {
	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory %s: %w", worktreePath, err)
	}

	// git worktree add <path> -b <branch>
	if err := runGit("-C", repoRoot, "worktree", "add", worktreePath, "-b", branchName); err != nil {
		return fmt.Errorf("CreateWorktree(%q, %q): %w", repoRoot, worktreePath, err)
	}
	return nil
}

// MergeBranch merges fromBranch into the branch currently checked out at
// worktreeDir using --no-ff. The caller is responsible for ensuring worktreeDir
// is already on the desired target branch.
func (g *RealGitClient) MergeBranch(worktreeDir, fromBranch string) error {
	intoBranch, err := runGitOutput("-C", worktreeDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("MergeBranch: could not determine current branch: %w", err)
	}
	mergeMsg := fmt.Sprintf("merge %s into %s", fromBranch, intoBranch)
	return runGit("-C", worktreeDir, "merge", "--no-ff", fromBranch, "-m", mergeMsg)
}

// GetHeadCommit returns the abbreviated HEAD commit hash for the git
// repository at path.
func (g *RealGitClient) GetHeadCommit(path string) (string, error) {
	return runGitOutput("-C", path, "rev-parse", "--short", "HEAD")
}

// RemoveWorktree removes the linked worktree at worktreePath and deletes its
// associated branch branchName.
func (g *RealGitClient) RemoveWorktree(repoRoot, worktreePath, branchName string) error {
	if err := runGit("-C", repoRoot, "worktree", "remove", "--force", worktreePath); err != nil {
		return fmt.Errorf("RemoveWorktree(%q, %q): %w", repoRoot, worktreePath, err)
	}

	if err := runGit("-C", repoRoot, "branch", "-d", branchName); err != nil {
		return fmt.Errorf("RemoveWorktree: delete branch %q: %w", branchName, err)
	}

	return nil
}
