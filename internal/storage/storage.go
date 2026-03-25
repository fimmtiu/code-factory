// Package storage provides path utilities and initialization for the .tickets/
// directory.
package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fimmtiu/tickets/internal/config"
)

// FindRepoRoot walks up from startDir looking for a .git directory.
// It returns the directory that contains .git, or an error if no such
// directory is found before the filesystem root is reached.
func FindRepoRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("FindRepoRoot: %w", err)
	}

	for {
		info, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("FindRepoRoot: no git repository found (no .git directory in any parent)")
		}
		dir = parent
	}
}

// TicketsDirPath returns the path to the .tickets/ directory within repoRoot.
func TicketsDirPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".tickets")
}

// InitTicketsDir creates the .tickets/ directory under repoRoot (if it does not
// already exist) and writes a default .settings.json (if one does not already
// exist). It is safe to call multiple times (idempotent).
func InitTicketsDir(repoRoot string) error {
	ticketsDir := TicketsDirPath(repoRoot)

	if err := os.MkdirAll(ticketsDir, 0755); err != nil {
		return fmt.Errorf("InitTicketsDir: create .tickets/: %w", err)
	}

	settingsPath := filepath.Join(ticketsDir, ".settings.json")
	if _, err := os.Stat(settingsPath); errors.Is(err, os.ErrNotExist) {
		if err := config.Save(ticketsDir, config.Default()); err != nil {
			return fmt.Errorf("InitTicketsDir: write .settings.json: %w", err)
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
