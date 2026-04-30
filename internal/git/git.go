package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// UncommittedHash is the sentinel hash for the "uncommitted changes" pseudo-commit.
const UncommittedHash = "????"

// CommitEntry represents one commit in a log listing.
type CommitEntry struct {
	Hash    string
	Message string
}

// Output runs a git command in the given directory and returns trimmed stdout.
func Output(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DetectDefaultBranch returns "main" or "master" depending on which branch
// exists in the worktree's repository.
func DetectDefaultBranch(worktreePath string) string {
	if out, err := Output(worktreePath, "rev-parse", "--verify", "main"); err == nil && out != "" {
		return "main"
	}
	return "master"
}

// FetchCommitList returns the most recent commits from the worktree, newest first.
// It runs git log --no-merges to get non-merge commits with full hashes.
func FetchCommitList(worktreePath string, maxCommits int) ([]CommitEntry, error) {
	out, err := Output(worktreePath, "log", "--no-merges",
		"--format=%H %s", fmt.Sprintf("-%d", maxCommits))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var commits []CommitEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		hash, message, _ := strings.Cut(line, " ")
		commits = append(commits, CommitEntry{Hash: hash, Message: message})
	}
	return commits, nil
}

// FetchForkPoint returns the commit hash where the current branch diverged from
// the given default branch. Uses merge-base (commit graph) rather than
// --fork-point (reflog), since reflogs are unreliable in worktrees.
func FetchForkPoint(worktreePath, defaultBranch string) (string, error) {
	return Output(worktreePath, "merge-base", defaultBranch, "HEAD")
}

// FirstNonMergeAncestor returns the hash of the most recent non-merge commit
// reachable from the given commit (inclusive). This is useful when the fork
// point is itself a merge commit and won't appear in --no-merges log output.
func FirstNonMergeAncestor(worktreePath, commitHash string) (string, error) {
	return Output(worktreePath, "log", "--no-merges", "--format=%H", "-1", commitHash)
}

// FetchShowStat returns the commit message followed by the --stat file list
// for the given commit. For the uncommitted pseudo-commit, it returns
// git diff --stat prefixed with a blank line so the preview pane treats its
// first line the same as a commit subject (which it bolds).
func FetchShowStat(worktreePath, commitHash string) (string, error) {
	if commitHash == UncommittedHash {
		out, err := Output(worktreePath, "diff", "--stat")
		if err != nil {
			return "", err
		}
		return "\n" + out, nil
	}
	return Output(worktreePath, "show", "--stat", "--format=%B", commitHash)
}

// FetchDiff returns the raw diff between two commits.
// It handles three cases:
//   - Both are uncommitted: git diff (working tree changes)
//   - End is uncommitted: git diff <start> (changes from start to working tree)
//   - Normal range: git diff <start>^..<end> (from parent of start to end)
func FetchDiff(worktreePath string, startCommit, endCommit CommitEntry) (string, error) {
	if startCommit.Hash == UncommittedHash && endCommit.Hash == UncommittedHash {
		return Output(worktreePath, "diff")
	}
	if endCommit.Hash == UncommittedHash {
		return Output(worktreePath, "diff", startCommit.Hash)
	}
	return Output(worktreePath, "diff", startCommit.Hash+"^.."+endCommit.Hash)
}

// HasUncommittedChanges reports whether `git diff` would produce output —
// i.e. there are unstaged modifications to tracked files. This intentionally
// matches FetchDiff/FetchShowStat (both of which call plain `git diff`), so
// the "Uncommitted changes" pseudo-commit is only offered when selecting it
// would actually render content. Untracked files and staged-only changes do
// not count.
func HasUncommittedChanges(worktreePath string) (bool, error) {
	cmd := exec.Command("git", "-C", worktreePath, "diff", "--quiet")
	err := cmd.Run()
	if err == nil {
		return false, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return true, nil
	}
	return false, err
}
