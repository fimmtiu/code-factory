// Package storage provides path utilities and initialization for the .code-factory/
// directory.
package storage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fimmtiu/code-factory/internal/config"
)

// FindRepoRoot walks up from startDir looking for .git. If .git is a
// directory, it returns the directory that contains it. If .git is a file (a
// linked worktree, which contains "gitdir: <path>"), it returns the root of the
// main repository, so that all worktrees of a repository share one
// .code-factory/ directory. It returns an error if no .git is found before the
// filesystem root is reached.
func FindRepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("FindRepoRoot: %w", err)
	}

	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				return dir, nil
			}
			return mainRootFromGitFile(dir, gitPath)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("FindRepoRoot: no git repository found (no .git in any parent)")
		}
		dir = parent
	}
}

// mainRootFromGitFile reads the .git file at gitPath in worktree dir and
// returns the root of the main repository. If the git directory has no
// commondir file (for example, a submodule), it returns dir.
func mainRootFromGitFile(dir, gitPath string) (string, error) {
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", fmt.Errorf("FindRepoRoot: %w", err)
	}
	line := strings.TrimSpace(string(data))
	gitDir, ok := strings.CutPrefix(line, "gitdir:")
	if !ok {
		return "", fmt.Errorf("FindRepoRoot: unrecognized .git file %s", gitPath)
	}
	gitDir = resolvePath(dir, strings.TrimSpace(gitDir))

	commonData, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if errors.Is(err, fs.ErrNotExist) {
		return dir, nil
	}
	if err != nil {
		return "", fmt.Errorf("FindRepoRoot: %w", err)
	}
	commonDir := resolvePath(gitDir, strings.TrimSpace(string(commonData)))
	return filepath.Dir(commonDir), nil
}

func resolvePath(base, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(base, path)
}

// TicketsDirPath returns the path to the .code-factory/ directory within repoRoot.
func TicketsDirPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".code-factory")
}

// InitTicketsDir creates the .code-factory/ directory under repoRoot (if it does not
// already exist) and writes a default settings.json (if one does not already
// exist). It is safe to call multiple times (idempotent).
func InitTicketsDir(repoRoot string) error {
	ticketsDir := TicketsDirPath(repoRoot)

	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		return fmt.Errorf("InitTicketsDir: create .code-factory/: %w", err)
	}

	if _, err := os.Stat(config.Path(ticketsDir)); errors.Is(err, os.ErrNotExist) {
		if err := config.Save(ticketsDir, config.Default()); err != nil {
			return fmt.Errorf("InitTicketsDir: write settings.json: %w", err)
		}
	}

	return nil
}

// TicketDirPath returns the directory path for a ticket given its identifier.
func TicketDirPath(ticketsDir, identifier string) string {
	return filepath.Join(ticketsDir, filepath.FromSlash(identifier))
}

// TicketWorktreePath returns the path where the worktree should be placed for a ticket.
func TicketWorktreePath(ticketDir string) string {
	return filepath.Join(ticketDir, "worktree")
}

// TicketWorktreePathIn returns the git worktree path for a ticket identifier
// within the given ticketsDir, without resolving the repo root.
func TicketWorktreePathIn(ticketsDir, identifier string) string {
	return TicketWorktreePath(TicketDirPath(ticketsDir, identifier))
}

// WorktreePathForIdentifier returns the git worktree path for the given ticket
// identifier, resolving the repo root from the current directory.
func WorktreePathForIdentifier(identifier string) (string, error) {
	repoRoot, err := FindRepoRoot(".")
	if err != nil {
		return "", err
	}
	return TicketWorktreePathIn(TicketsDirPath(repoRoot), identifier), nil
}
