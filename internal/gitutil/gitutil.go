package gitutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	// EnableRerere enables git's rerere ("reuse recorded resolution") feature
	// in the repository that contains worktreeDir. Since worktrees share the
	// parent repo's rr-cache, enabling rerere in any worktree makes recorded
	// resolutions available to all sibling worktrees. The call is idempotent.
	EnableRerere(worktreeDir string) error
	// FindForbiddenMarkers returns lines added on the branch checked out at
	// worktreeDir but not present on targetBranch that contain incomplete-work
	// markers (TODO, FIXME, XXX, panic("unimplemented"), panic("not implemented")).
	// Hits in test files (paths matching *_test.* or under test/, tests/, spec/
	// directories) are filtered out. Each returned hit is formatted as
	// "path:line: text" so callers can surface a jump-to-source list.
	FindForbiddenMarkers(worktreeDir, targetBranch string) ([]string, error)
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
	base, err := runGitOutput("-C", worktreeDir, "merge-base", "HEAD", targetBranch)
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

// EnableRerere sets rerere.enabled=true and rerere.autoUpdate=true in the git
// config for the repository that contains worktreeDir. Since linked worktrees
// share the parent repo's rr-cache directory, enabling rerere in any worktree
// causes all conflict resolutions recorded during rebases to be cached and
// automatically replayed when the same conflict appears at a different level
// of the cascade.
//
// rerere.autoUpdate tells git to automatically stage files that rerere has
// resolved, which is necessary for non-interactive rebases to continue
// without manual intervention.
func (g *RealGitClient) EnableRerere(worktreeDir string) error {
	if err := runGit("-C", worktreeDir, "config", "rerere.enabled", "true"); err != nil {
		return err
	}
	return runGit("-C", worktreeDir, "config", "rerere.autoUpdate", "true")
}

// forbiddenMarkerPattern matches incomplete-work markers we want to keep out
// of merged code. The first alternative is the universal-comment family
// (TODO/FIXME/XXX) found in every language. The remaining alternatives are
// stub-throw idioms in the languages code-factory is most often used with:
// Go's panic("unimplemented"), Rust's unimplemented!()/todo!(), and
// Python/Ruby's NotImplementedError. Adding more languages is just a matter
// of extending this regex — no other code in the scanner is language-specific.
var forbiddenMarkerPattern = regexp.MustCompile(
	`\b(?:TODO|FIXME|XXX)\b` +
		`|panic\(\s*"(?:[Uu]nimplemented|not implemented)"\s*\)` +
		`|\b(?:unimplemented|todo)!\s*\(` +
		`|\braise\s+NotImplementedError\b`,
)

// isTestPath reports whether path looks like a test file by language convention.
// Hits in test files are treated as legitimate scaffolding (e.g. TODO markers
// listing test cases not yet covered) and are excluded from FindForbiddenMarkers.
func isTestPath(path string) bool {
	slashed := filepath.ToSlash(path)
	for _, dir := range []string{"test", "tests", "spec", "specs", "__tests__"} {
		if strings.Contains(slashed, "/"+dir+"/") || strings.HasPrefix(slashed, dir+"/") {
			return true
		}
	}
	base := filepath.Base(slashed)
	switch {
	case strings.HasSuffix(base, "_test.go"),
		strings.HasSuffix(base, "_test.py"),
		strings.HasPrefix(base, "test_"),
		strings.HasSuffix(base, "_spec.rb"),
		strings.HasSuffix(base, "_test.rb"):
		return true
	}
	for _, suffix := range []string{".test.js", ".test.ts", ".test.tsx", ".test.jsx", ".spec.js", ".spec.ts", ".spec.tsx", ".spec.jsx"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

// FindForbiddenMarkers diffs HEAD against targetBranch (using merge-base
// semantics) and returns one "path:line: text" entry per added line that
// matches forbiddenMarkerPattern in a non-test file. The result is empty
// when the diff is clean.
func (g *RealGitClient) FindForbiddenMarkers(worktreeDir, targetBranch string) ([]string, error) {
	out, err := runGitOutput(
		"-C", worktreeDir,
		"diff", "--no-color", "-U0",
		targetBranch+"...HEAD",
	)
	if err != nil {
		return nil, fmt.Errorf("FindForbiddenMarkers: diff: %w", err)
	}
	return scanDiffForMarkers(out), nil
}

// scanDiffForMarkers walks unified-diff output and returns hits in non-test
// files.
func scanDiffForMarkers(diff string) []string {
	if diff == "" {
		return nil
	}
	var (
		hits        []string
		currentPath string
		skipFile    bool
		lineNo      int
	)
	hunkHeader := regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++ "):
			// "+++ b/path/to/file" or "+++ /dev/null" for deletions.
			rest := strings.TrimPrefix(line, "+++ ")
			if rest == "/dev/null" {
				currentPath = ""
				skipFile = true
				continue
			}
			currentPath = strings.TrimPrefix(rest, "b/")
			skipFile = isTestPath(currentPath)
		case strings.HasPrefix(line, "--- "):
			// Reset state — a new file pair is starting; +++ will follow.
			currentPath = ""
			skipFile = false
			lineNo = 0
		case strings.HasPrefix(line, "@@"):
			if skipFile || currentPath == "" {
				continue
			}
			m := hunkHeader.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			start, perr := strconv.Atoi(m[1])
			if perr != nil {
				continue
			}
			lineNo = start
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			if skipFile || currentPath == "" {
				continue
			}
			added := strings.TrimPrefix(line, "+")
			if forbiddenMarkerPattern.MatchString(added) {
				hits = append(hits, fmt.Sprintf("%s:%d: %s", currentPath, lineNo, strings.TrimSpace(added)))
			}
			lineNo++
		case strings.HasPrefix(line, "-"):
			// removed line: position counter only advances on '+' / ' ' lines
		case strings.HasPrefix(line, " "):
			if !skipFile && currentPath != "" {
				lineNo++
			}
		}
	}
	return hits
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
