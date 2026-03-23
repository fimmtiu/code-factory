// Package storage implements file system I/O for the .tickets/ directory.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fimmtiu/tickets/internal/config"
	"github.com/fimmtiu/tickets/internal/models"
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
			// Reached the filesystem root without finding .git.
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

// ProjectMetaPath returns the path to the project.json file inside a project directory.
func ProjectMetaPath(projectDir string) string {
	return filepath.Join(projectDir, "project.json")
}

// TicketMetaPath returns the path to the ticket.json file inside a ticket directory.
func TicketMetaPath(ticketDir string) string {
	return filepath.Join(ticketDir, "ticket.json")
}

// TicketWorktreePath returns the path where the worktree should be placed for a ticket.
func TicketWorktreePath(ticketDir string) string {
	return filepath.Join(ticketDir, "worktree")
}

// TicketDirPath returns the directory path for a ticket given its identifier.
func TicketDirPath(ticketsDir, identifier string) string {
	return filepath.Join(ticketsDir, filepath.FromSlash(identifier))
}

// TraverseAll recursively walks ticketsDir and returns all work units found.
// Only directories are examined. A directory containing ticket.json is a ticket
// (leaf); one containing project.json is a project (recurse into it).
// The Parent field of each WorkUnit is set to the identifier of its containing
// project, or "" for top-level items.
func TraverseAll(ticketsDir string) ([]*models.WorkUnit, error) {
	return traverseDir(ticketsDir, "")
}

// traverseDir is the recursive helper for TraverseAll.
// dir is the current directory being scanned.
// parentIdentifier is the identifier of the enclosing project ("" at top level).
func traverseDir(dir, parentIdentifier string) ([]*models.WorkUnit, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("TraverseAll: read dir %s: %w", dir, err)
	}

	var results []*models.WorkUnit
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		subDir := filepath.Join(dir, name)

		var id string
		if parentIdentifier == "" {
			id = name
		} else {
			id = parentIdentifier + "/" + name
		}

		if _, err := os.Stat(TicketMetaPath(subDir)); err == nil {
			wu, err := ReadWorkUnit(TicketMetaPath(subDir))
			if err != nil {
				return nil, fmt.Errorf("TraverseAll: read ticket %s: %w", subDir, err)
			}
			wu.Identifier = id
			wu.IsProject = false
			wu.Parent = parentIdentifier
			results = append(results, wu)
		} else if _, err := os.Stat(ProjectMetaPath(subDir)); err == nil {
			wu, err := ReadWorkUnit(ProjectMetaPath(subDir))
			if err != nil {
				return nil, fmt.Errorf("TraverseAll: read project %s: %w", subDir, err)
			}
			wu.Identifier = id
			wu.IsProject = true
			wu.Parent = parentIdentifier
			results = append(results, wu)

			children, err := traverseDir(subDir, id)
			if err != nil {
				return nil, err
			}
			results = append(results, children...)
		}
	}

	return results, nil
}

// ReadWorkUnit reads a JSON-encoded WorkUnit from path.
func ReadWorkUnit(path string) (*models.WorkUnit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wu models.WorkUnit
	if err := json.Unmarshal(data, &wu); err != nil {
		return nil, fmt.Errorf("ReadWorkUnit %s: %w", path, err)
	}
	return &wu, nil
}

// WriteWorkUnit writes wu to path as indented JSON.
func WriteWorkUnit(path string, wu *models.WorkUnit) error {
	data, err := json.MarshalIndent(wu, "", "  ")
	if err != nil {
		return fmt.Errorf("WriteWorkUnit marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("WriteWorkUnit write: %w", err)
	}
	return nil
}

// CreateTicketDir creates the ticket directory and writes an initial ticket.json.
func CreateTicketDir(ticketsDir, identifier string) error {
	ticketDir := filepath.Join(ticketsDir, filepath.FromSlash(identifier))
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		return fmt.Errorf("CreateTicketDir: mkdir %s: %w", ticketDir, err)
	}
	wu := &models.WorkUnit{
		Identifier:   identifier,
		Status:       models.StatusOpen,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    false,
	}
	return WriteWorkUnit(TicketMetaPath(ticketDir), wu)
}

// CreateProjectDir creates a project directory for the given identifier under
// ticketsDir and writes an initial project.json file. The identifier may
// contain "/" to represent nested projects (e.g., "my-feature/sub-task"),
// in which case intermediate directories are created as needed.
func CreateProjectDir(ticketsDir, identifier string) error {
	projDir := filepath.Join(ticketsDir, filepath.FromSlash(identifier))

	if err := os.MkdirAll(projDir, 0755); err != nil {
		return fmt.Errorf("CreateProjectDir: mkdir %s: %w", projDir, err)
	}

	wu := &models.WorkUnit{
		Identifier:   identifier,
		Description:  "",
		Status:       models.ProjectOpen,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    true,
	}

	if err := WriteWorkUnit(ProjectMetaPath(projDir), wu); err != nil {
		return fmt.Errorf("CreateProjectDir: write project.json: %w", err)
	}

	return nil
}
