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

// IsWorktreeClean reports whether the worktree at worktreePath has no
// pending state of any kind: no unstaged changes, no staged changes, no
// untracked files, no unmerged entries, no in-progress rebase or merge
// that would leave files dirty. Implemented as `git status --porcelain`
// returning empty output. Used by the merging phase to decide whether a
// rebase can safely (re)run after an agent has attempted to resolve
// conflicts.
func IsWorktreeClean(worktreePath string) (bool, error) {
	out, err := Output(worktreePath, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// IsWorktreeCleanForScope reports whether the worktree is clean from the
// perspective of a work unit whose owned paths are `scope`. Untracked and
// modified files that fall outside every scope entry are ignored — they
// belong to sibling units whose outputs were staged into this worktree so
// the unit's code could load and run, and they aren't this unit's
// responsibility to commit. Files inside the scope are evaluated normally.
//
// Unmerged entries (any "U" status, plus AA/DD) are always treated as
// dirty regardless of scope, because they mean a rebase or merge is
// physically not in a state to continue.
//
// An empty scope falls back to IsWorktreeClean's whole-tree semantics, so
// units without a declared WriteScope keep the old behavior.
//
// Scope entries are matched as either an exact path or a directory prefix.
// A trailing "/" is optional: "lib/foo" matches "lib/foo" and any path
// under "lib/foo/" but not "lib/foo_bar".
func IsWorktreeCleanForScope(worktreePath string, scope []string) (bool, error) {
	if len(scope) == 0 {
		return IsWorktreeClean(worktreePath)
	}
	// Don't go through Output(): it calls strings.TrimSpace, which would
	// eat the leading space on worktree-only status codes (" M", " D", " A")
	// and break the 2-char XY parse below.
	raw, err := exec.Command("git", "-C", worktreePath, "status",
		"--porcelain=v1", "-z", "--untracked-files=all").Output()
	if err != nil {
		return false, err
	}
	if len(raw) == 0 {
		return true, nil
	}
	entries := strings.Split(string(raw), "\x00")
	for i := 0; i < len(entries); i++ {
		e := entries[i]
		if len(e) < 3 {
			continue
		}
		xy := e[:2]
		path := e[3:]
		// Renames and copies carry the original path as the next NUL-
		// separated entry. Consume it so it isn't reparsed as a fresh
		// status line; the new path (above) is what we evaluate.
		if xy[0] == 'R' || xy[0] == 'C' {
			i++
		}
		if isUnmergedStatus(xy) {
			return false, nil
		}
		if pathInScope(path, scope) {
			return false, nil
		}
	}
	return true, nil
}

// isUnmergedStatus reports whether a porcelain XY status code indicates an
// unmerged path: any 'U' on either side, plus the AA and DD double-modify
// cases. These mean a rebase or merge is physically stuck and can't be
// continued, irrespective of which unit owns the file.
func isUnmergedStatus(xy string) bool {
	if len(xy) < 2 {
		return false
	}
	if xy[0] == 'U' || xy[1] == 'U' {
		return true
	}
	return xy == "AA" || xy == "DD"
}

// pathInScope reports whether `path` falls inside any scope entry. An
// entry matches as either an exact path or as a directory prefix; the
// trailing "/" on a directory entry is optional. The "/" boundary check
// prevents "lib/foo" from spuriously matching "lib/foo_bar".
func pathInScope(path string, scope []string) bool {
	for _, entry := range scope {
		e := strings.TrimSuffix(entry, "/")
		if e == "" {
			continue
		}
		if path == e {
			return true
		}
		if strings.HasPrefix(path, e+"/") {
			return true
		}
	}
	return false
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
