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
		// Write default settings only when the file does not yet exist.
		if err := config.Save(ticketsDir, config.Default()); err != nil {
			return fmt.Errorf("InitTicketsDir: write .settings.json: %w", err)
		}
	}

	return nil
}

// IsProjectDir reports whether name (the base name of a directory entry inside
// .tickets/) identifies a project directory. A project directory is any directory
// whose name does NOT begin with a period.
func IsProjectDir(name string) bool {
	return name != "" && name[0] != '.'
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
// Non-dot directories are treated as project directories (their .project.json
// is read). Non-dot .json files are treated as ticket files.
// The Parent field of each WorkUnit is set to the identifier of its containing
// project, or "" for top-level items.
func TraverseAll(ticketsDir string) ([]*models.WorkUnit, error) {
	var results []*models.WorkUnit

	err := traverseDir(ticketsDir, ticketsDir, "", &results)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// traverseDir is the recursive helper for TraverseAll.
// dir is the current directory being scanned.
// ticketsDir is the root .tickets/ path (used to compute relative identifiers).
// parentIdentifier is the identifier of the enclosing project ("" at top level).
func traverseDir(dir, ticketsDir, parentIdentifier string, results *[]*models.WorkUnit) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("TraverseAll: read dir %s: %w", dir, err)
	}

	for _, entry := range entries {
		name := entry.Name()

		if !entry.IsDir() {
			continue
		}
		if !IsProjectDir(name) {
			continue
		}

		// Compute identifier for this directory entry.
		var id string
		if parentIdentifier == "" {
			id = name
		} else {
			id = parentIdentifier + "/" + name
		}

		subDir := filepath.Join(dir, name)

		// Determine whether this is a ticket directory or a project directory.
		ticketMetaPath := filepath.Join(subDir, "ticket.json")
		projectFilePath := filepath.Join(subDir, ".project.json")

		if _, err := os.Stat(ticketMetaPath); err == nil {
			// It's a ticket directory — read ticket.json and do NOT recurse.
			wu, err := ReadWorkUnit(ticketMetaPath)
			if err != nil {
				return fmt.Errorf("TraverseAll: read ticket %s: %w", ticketMetaPath, err)
			}
			wu.Identifier = id
			wu.IsProject = false
			wu.Parent = parentIdentifier
			*results = append(*results, wu)
		} else if _, err := os.Stat(projectFilePath); err == nil {
			// It's a project directory — read .project.json and recurse.
			wu, err := ReadWorkUnit(projectFilePath)
			if err != nil {
				return fmt.Errorf("TraverseAll: read project %s: %w", projectFilePath, err)
			}
			wu.Identifier = id
			wu.IsProject = true
			wu.Parent = parentIdentifier
			*results = append(*results, wu)

			if err := traverseDir(subDir, ticketsDir, id, results); err != nil {
				return err
			}
		}
		// else: neither ticket.json nor .project.json — skip
	}

	return nil
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

// WriteWorkUnit writes wu to path as indented JSON using an atomic
// write-to-temp-then-rename sequence.
func WriteWorkUnit(path string, wu *models.WorkUnit) error {
	data, err := json.MarshalIndent(wu, "", "  ")
	if err != nil {
		return fmt.Errorf("WriteWorkUnit marshal: %w", err)
	}

	// Write to a temp file in the same directory so os.Rename is atomic.
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tickets-tmp-*")
	if err != nil {
		return fmt.Errorf("WriteWorkUnit create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("WriteWorkUnit write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("WriteWorkUnit close temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("WriteWorkUnit rename: %w", err)
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
// ticketsDir and writes an initial .project.json file. The identifier may
// contain "/" to represent nested projects (e.g., "my-feature/sub-task"),
// in which case intermediate directories are created as needed.
func CreateProjectDir(ticketsDir, identifier string) error {
	relPath := filepath.FromSlash(identifier)
	projDir := filepath.Join(ticketsDir, relPath)

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

	projectFile := filepath.Join(projDir, ".project.json")
	if err := WriteWorkUnit(projectFile, wu); err != nil {
		return fmt.Errorf("CreateProjectDir: write .project.json: %w", err)
	}

	return nil
}
