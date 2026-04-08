package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	// GetCurrentBranch returns the name of the branch currently checked out
	// at the given path.
	GetCurrentBranch(path string) (string, error)
	// RebaseOnto rebases the branch checked out at worktreeDir onto
	// ontoBranch. If the rebase fails (e.g. conflicts), it is aborted so
	// the worktree is left in its original state.
	RebaseOnto(worktreeDir, ontoBranch string) error
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
	safeBranchName := strings.ReplaceAll(branchName, "/", "_")

	if err := runGit("-C", repoRoot, "worktree", "add", worktreePath, "-b", safeBranchName); err != nil {
		return fmt.Errorf("CreateWorktree(%q, %q): %w", repoRoot, worktreePath, err)
	}

	// Copy .claude/settings.json into the worktree so that ACP agents
	// inherit the project's permission allow-list. Without this, agents
	// running in the worktree won't find the file (it lives on main but
	// the worktree branch may predate the commit).
	srcSettings := filepath.Join(repoRoot, ".claude", "settings.json")
	if data, err := os.ReadFile(srcSettings); err == nil {
		dstDir := filepath.Join(worktreePath, ".claude")
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("CreateWorktree: create .claude dir in worktree: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, "settings.json"), data, 0644); err != nil {
			return fmt.Errorf("CreateWorktree: copy settings.json to worktree: %w", err)
		}
	}

	return nil
}

// MergeBranch merges fromBranch into the branch currently checked out at
// worktreeDir using --no-ff. The caller is responsible for ensuring worktreeDir
// is already on the desired target branch.
func (g *RealGitClient) MergeBranch(worktreeDir, fromBranch string) error {
	safeBranchName := strings.ReplaceAll(fromBranch, "/", "_")
	intoBranch, err := runGitOutput("-C", worktreeDir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return fmt.Errorf("MergeBranch: could not determine current branch: %w", err)
	}
	mergeMsg := fmt.Sprintf("merge %s into %s", safeBranchName, intoBranch)
	return runGit("-C", worktreeDir, "merge", "--no-ff", safeBranchName, "-m", mergeMsg)
}

// GetHeadCommit returns the abbreviated HEAD commit hash for the git
// repository at path.
func (g *RealGitClient) GetHeadCommit(path string) (string, error) {
	return runGitOutput("-C", path, "rev-parse", "--short", "HEAD")
}

// GetCurrentBranch returns the name of the branch currently checked out at path.
func (g *RealGitClient) GetCurrentBranch(path string) (string, error) {
	return runGitOutput("-C", path, "rev-parse", "--abbrev-ref", "HEAD")
}

// RebaseOnto rebases the branch checked out at worktreeDir onto ontoBranch.
// If the rebase fails (e.g. due to conflicts), the rebase is aborted so the
// worktree is left in its original state.
func (g *RealGitClient) RebaseOnto(worktreeDir, ontoBranch string) error {
	if err := runGit("-C", worktreeDir, "rebase", ontoBranch); err != nil {
		_ = runGit("-C", worktreeDir, "rebase", "--abort")
		return err
	}
	return nil
}

// RemoveWorktree removes the linked worktree at worktreePath and deletes its
// associated branch branchName.
func (g *RealGitClient) RemoveWorktree(repoRoot, worktreePath, branchName string) error {
	safeBranchName := strings.ReplaceAll(branchName, "/", "_")
	if err := runGit("-C", repoRoot, "worktree", "remove", "--force", worktreePath); err != nil {
		return fmt.Errorf("RemoveWorktree(%q, %q): %w", repoRoot, worktreePath, err)
	}

	if err := runGit("-C", repoRoot, "branch", "-D", safeBranchName); err != nil {
		return fmt.Errorf("RemoveWorktree: delete branch %q: %w", safeBranchName, err)
	}

	return nil
}
