package ui

import (
	"fmt"
	"strings"
)

// UncommittedRef is the sentinel value passed as fromCommit to fetchDiff
// to request a diff of uncommitted changes against HEAD.
const UncommittedRef = "uncommitted"

// fetchCommitList returns the most recent commits from the worktree, newest first.
// It runs git log --oneline --no-merges to get non-merge commits with full hashes.
func fetchCommitList(worktreePath string, maxCommits int) ([]commitEntry, error) {
	out, err := gitOutput(worktreePath, "log", "--oneline", "--no-merges",
		fmt.Sprintf("-%d", maxCommits), "--format=%H %s")
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

// fetchShowStat returns the output of git show --stat for the given commit.
func fetchShowStat(worktreePath, commitHash string) (string, error) {
	return gitOutput(worktreePath, "show", "--stat", commitHash)
}

// fetchDiff returns the raw diff between two commits. If fromCommit is
// UncommittedRef, it returns the diff of uncommitted changes against HEAD
// instead.
func fetchDiff(worktreePath, fromCommit, toCommit string) (string, error) {
	if fromCommit == UncommittedRef {
		return gitOutput(worktreePath, "diff", "HEAD")
	}
	return gitOutput(worktreePath, "diff", fromCommit+".."+toCommit)
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
