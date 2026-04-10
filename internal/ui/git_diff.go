package ui

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/storage"
)

// openGitHubCompare opens a GitHub compare page for the ticket's branch
// against the default branch.
func openGitHubCompare(identifier string) {
	worktreePath, err := storage.WorktreePathForIdentifier(identifier)
	if err != nil {
		return
	}
	defaultBranch := git.DetectDefaultBranch(worktreePath)
	branchName, err := git.Output(worktreePath, "branch", "--show-current")
	if err != nil || branchName == "" {
		return
	}
	originURL, _ := git.Output(worktreePath, "remote", "get-url", "origin")
	repo := extractGitHubRepo(originURL)
	if repo == "" {
		return
	}
	url := "https://github.com/" + repo + "/compare/" + defaultBranch + "..." + branchName
	_ = exec.Command("open", url).Start()
}

// isGitHubRepo returns true if the repository's origin remote points to
// github.com. The result is computed once and cached.
var isGitHubRepo = sync.OnceValue(func() bool {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return false
	}
	originURL, err := git.Output(repoRoot, "remote", "get-url", "origin")
	if err != nil {
		return false
	}
	return strings.Contains(originURL, "github.com")
})

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
// Logfiles live at .code-factory/<project>/<ticket>/<phase>.log, so the identifier
// is the two path segments between .code-factory/ and the filename.
func identifierFromLogfile(logfile string) string {
	// Normalise and split.
	parts := strings.Split(filepath.ToSlash(logfile), "/")
	for i, p := range parts {
		if p == ".code-factory" && i+3 < len(parts) {
			return parts[i+1] + "/" + parts[i+2]
		}
	}
	return ""
}

// phaseFromLogfile extracts the phase name from a logfile path.
// Logfiles live at .code-factory/<project>/<ticket>/<phase>.log (optionally
// numbered as <phase>.log.N), so the phase is the filename base before ".log".
func phaseFromLogfile(logfile string) string {
	parts := strings.Split(filepath.ToSlash(logfile), "/")
	for i, p := range parts {
		if p == ".code-factory" && i+3 < len(parts) {
			filename := parts[i+3]
			// Strip ".log" or ".log.N" suffix to get the phase.
			if idx := strings.Index(filename, ".log"); idx > 0 {
				return filename[:idx]
			}
		}
	}
	return ""
}
