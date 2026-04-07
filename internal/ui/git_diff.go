package ui

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fimmtiu/code-factory/internal/storage"
)

// openGitDiff opens a GitHub compare page if the repo's origin remote is on
// github.com, otherwise opens a terminal in the worktree and runs git diff
// against the fork point.
func openGitDiff(identifier string) {
	worktreePath, err := storage.WorktreePathForIdentifier(identifier)
	if err != nil {
		return
	}

	// Determine the default branch (main or master).
	defaultBranch := detectDefaultBranch(worktreePath)

	// Get the current branch name in the worktree.
	branchName, err := gitOutput(worktreePath, "branch", "--show-current")
	if err != nil || branchName == "" {
		return
	}

	// Check if origin is a GitHub remote.
	originURL, err := gitOutput(worktreePath, "remote", "get-url", "origin")
	if err == nil && strings.Contains(originURL, "github.com") {
		repo := extractGitHubRepo(originURL)
		if repo != "" {
			url := "https://github.com/" + repo + "/compare/" + defaultBranch + "..." + branchName
			_ = exec.Command("open", url).Start()
			return
		}
	}

	// Fallback: open a terminal in the worktree and run git diff against the fork point.
	diffCmd := "git diff $(git merge-base --fork-point '" + defaultBranch + "')"
	_ = openTerminalWithCommand(worktreePath, diffCmd)
}

// openTerminalWithCommand opens iTerm2 in dir and runs cmd.
func openTerminalWithCommand(dir, cmd string) error {
	script := `tell application "iTerm2"
	tell current window
		set myNewTab to create tab with default profile
		tell current session of myNewTab
			write text "cd ` + dir + ` && ` + cmd + `"
		end tell
	end tell
end tell`
	c := exec.Command("osascript")
	c.Stdin = strings.NewReader(script)
	return c.Start()
}

// detectDefaultBranch returns "main" or "master" depending on which branch
// exists in the worktree's repository.
func detectDefaultBranch(worktreePath string) string {
	if out, err := gitOutput(worktreePath, "rev-parse", "--verify", "main"); err == nil && out != "" {
		return "main"
	}
	return "master"
}

// gitOutput runs a git command in the given directory and returns trimmed stdout.
func gitOutput(dir string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", dir}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// extractGitHubRepo extracts "owner/repo" from a GitHub remote URL.
// Handles both SSH (git@github.com:owner/repo.git) and HTTPS
// (https://github.com/owner/repo.git) formats.
func extractGitHubRepo(url string) string {
	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@github.com:") {
		repo := strings.TrimPrefix(url, "git@github.com:")
		repo = strings.TrimSuffix(repo, ".git")
		return repo
	}
	// HTTPS format: https://github.com/owner/repo.git
	if idx := strings.Index(url, "github.com/"); idx >= 0 {
		repo := url[idx+len("github.com/"):]
		repo = strings.TrimSuffix(repo, ".git")
		// Strip trailing path segments beyond owner/repo.
		parts := strings.SplitN(repo, "/", 3)
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
	}
	return ""
}

// identifierFromLogfile extracts the ticket identifier from a logfile path.
// Logfiles live at .tickets/<project>/<ticket>/<phase>.log, so the identifier
// is the two path segments between .tickets/ and the filename.
func identifierFromLogfile(logfile string) string {
	// Normalise and split.
	parts := strings.Split(filepath.ToSlash(logfile), "/")
	for i, p := range parts {
		if p == ".tickets" && i+3 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
	}
	return ""
}
