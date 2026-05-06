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
	// MergeBranch fast-forwards the branch currently checked out at
	// worktreeDir to the tip of fromBranch. The caller must ensure fromBranch
	// is already a descendant of the target (typically by rebasing it first);
	// otherwise the fast-forward will fail.
	MergeBranch(worktreeDir, fromBranch string) error
	RemoveWorktree(repoRoot, worktreePath, branchName string) error
	// GetHeadCommit returns the abbreviated HEAD commit hash for the git
	// repository rooted at path.
	GetHeadCommit(path string) (string, error)
	// GetCurrentBranch returns the name of the branch currently checked out
	// at the given path.
	GetCurrentBranch(path string) (string, error)
	// RebaseOnto rebases the branch checked out at worktreeDir onto
	// ontoBranch. On failure, the rebase is left in progress with conflict
	// markers in place so conflicts can be resolved manually. Callers that
	// want a clean worktree on failure should call AbortRebase.
	RebaseOnto(worktreeDir, ontoBranch string) error
	// AbortRebase aborts an in-progress rebase in worktreeDir, restoring the
	// worktree to its pre-rebase state.
	AbortRebase(worktreeDir string) error
	// SquashSinceMergeBase rewrites the branch checked out at worktreeDir into
	// a single commit covering everything not on targetBranch. summary is the
	// subject line; the original commits' subjects become the body. If the
	// branch already has at most one commit since the merge base with
	// targetBranch the call is a no-op, so it is safe to invoke repeatedly.
	SquashSinceMergeBase(worktreeDir, targetBranch, summary string) error
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

	// Copy .claude/settings.json and .claude/settings.local.json into the
	// worktree so that ACP agents inherit the project's permission
	// allow-list. Without this, agents running in the worktree won't find
	// the files (they live on main but the worktree branch may predate
	// the commit).
	for _, name := range []string{"settings.json", "settings.local.json"} {
		src := filepath.Join(repoRoot, ".claude", name)
		data, err := os.ReadFile(src)
		if err != nil {
			continue // file doesn't exist — nothing to copy
		}
		dstDir := filepath.Join(worktreePath, ".claude")
		if err := os.MkdirAll(dstDir, 0755); err != nil {
			return fmt.Errorf("CreateWorktree: create .claude dir in worktree: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dstDir, name), data, 0644); err != nil {
			return fmt.Errorf("CreateWorktree: copy %s to worktree: %w", name, err)
		}
	}

	return nil
}

// MergeBranch fast-forwards the branch currently checked out at worktreeDir
// to the tip of fromBranch. The caller is responsible for ensuring fromBranch
// is a descendant of the target branch (typically by rebasing it first);
// otherwise `--ff-only` will fail.
func (g *RealGitClient) MergeBranch(worktreeDir, fromBranch string) error {
	safeBranchName := strings.ReplaceAll(fromBranch, "/", "_")
	return runGit("-C", worktreeDir, "merge", "--ff-only", safeBranchName)
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
// On failure (e.g. conflicts), the rebase is left in progress with conflict
// markers in place so the caller or user can resolve them. Callers that want
// a clean worktree on failure must call AbortRebase.
func (g *RealGitClient) RebaseOnto(worktreeDir, ontoBranch string) error {
	return runGit("-C", worktreeDir, "rebase", ontoBranch)
}

// AbortRebase aborts an in-progress rebase in worktreeDir, restoring the
// worktree to its pre-rebase state. The underlying `git rebase --abort`
// error (if any) is returned so callers can decide whether to surface it.
func (g *RealGitClient) AbortRebase(worktreeDir string) error {
	return runGit("-C", worktreeDir, "rebase", "--abort")
}

// SquashSinceMergeBase squashes every commit on the branch checked out at
// worktreeDir that is not reachable from targetBranch into a single commit
// whose subject is summary and whose body lists the squashed commits' subjects.
// If there are 0 or 1 such commits, the branch is left untouched, making the
// call idempotent for retry paths.
func (g *RealGitClient) SquashSinceMergeBase(worktreeDir, targetBranch, summary string) error {
	safeTarget := strings.ReplaceAll(targetBranch, "/", "_")

	base, err := runGitOutput("-C", worktreeDir, "merge-base", "HEAD", safeTarget)
	if err != nil {
		return fmt.Errorf("SquashSinceMergeBase: merge-base: %w", err)
	}

	count, err := runGitOutput("-C", worktreeDir, "rev-list", "--count", base+"..HEAD")
	if err != nil {
		return fmt.Errorf("SquashSinceMergeBase: count: %w", err)
	}
	if count == "0" || count == "1" {
		return nil
	}

	subjects, err := runGitOutput("-C", worktreeDir, "log", "--reverse", "--format=%s", base+"..HEAD")
	if err != nil {
		return fmt.Errorf("SquashSinceMergeBase: log: %w", err)
	}

	var body strings.Builder
	for _, line := range strings.Split(subjects, "\n") {
		if line == "" {
			continue
		}
		body.WriteString("* ")
		body.WriteString(line)
		body.WriteByte('\n')
	}

	msg := summary + "\n\n" + body.String()

	if err := runGit("-C", worktreeDir, "reset", "--soft", base); err != nil {
		return fmt.Errorf("SquashSinceMergeBase: reset --soft: %w", err)
	}
	if err := runGit("-C", worktreeDir, "commit", "--allow-empty", "-m", msg); err != nil {
		return fmt.Errorf("SquashSinceMergeBase: commit: %w", err)
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
