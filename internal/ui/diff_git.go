package ui

import (
	"fmt"
	"strings"
)

// fetchCommitList returns the most recent commits from the worktree, newest first.
// It runs git log --no-merges to get non-merge commits with full hashes.
func fetchCommitList(worktreePath string, maxCommits int) ([]commitEntry, error) {
	out, err := gitOutput(worktreePath, "log", "--no-merges",
		"--format=%H %s", fmt.Sprintf("-%d", maxCommits))
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}

	var commits []commitEntry
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		hash, message, _ := strings.Cut(line, " ")
		commits = append(commits, commitEntry{Hash: hash, Message: message})
	}
	return commits, nil
}

// fetchForkPoint returns the commit hash where the current branch diverged from
// the given default branch.
func fetchForkPoint(worktreePath, defaultBranch string) (string, error) {
	return gitOutput(worktreePath, "merge-base", "--fork-point", defaultBranch)
}

// fetchShowStat returns the git show --stat output for the given commit,
// suppressing the commit header. For the uncommitted pseudo-commit, it
// returns git diff --stat instead.
func fetchShowStat(worktreePath, commitHash string) (string, error) {
	if commitHash == uncommittedHash {
		return gitOutput(worktreePath, "diff", "--stat")
	}
	return gitOutput(worktreePath, "show", "--stat", "--format=", commitHash)
}

// fetchDiff returns the raw diff between two commits.
// It handles three cases:
//   - Both are uncommitted: git diff (working tree changes)
//   - End is uncommitted: git diff <start> (changes from start to working tree)
//   - Normal range: git diff <start>^..<end> (from parent of start to end)
func fetchDiff(worktreePath string, startCommit, endCommit commitEntry) (string, error) {
	if startCommit.Hash == uncommittedHash && endCommit.Hash == uncommittedHash {
		return gitOutput(worktreePath, "diff")
	}
	if endCommit.Hash == uncommittedHash {
		return gitOutput(worktreePath, "diff", startCommit.Hash)
	}
	return gitOutput(worktreePath, "diff", startCommit.Hash+"^.."+endCommit.Hash)
}

// hasUncommittedChanges returns true if the worktree has any modified, staged,
// or untracked files.
func hasUncommittedChanges(worktreePath string) (bool, error) {
	out, err := gitOutput(worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}
