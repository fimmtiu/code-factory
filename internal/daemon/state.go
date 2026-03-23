package daemon

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
)

// State holds the in-memory representation of all work units loaded from the
// .tickets directory. It is safe for concurrent use.
type State struct {
	mu         sync.RWMutex
	units      map[string]*models.WorkUnit // keyed by identifier
	ticketsDir string
}

// NewState returns a new, empty State for the given tickets directory.
// Call Load to populate it from disk.
func NewState(ticketsDir string) *State {
	return &State{
		units:      make(map[string]*models.WorkUnit),
		ticketsDir: ticketsDir,
	}
}

// Load calls storage.TraverseAll and rebuilds the in-memory unit map.
// It is safe to call multiple times; each call replaces the previous data.
// If ticketsDir is empty, Load is a no-op and returns nil.
func (s *State) Load() error {
	if s.ticketsDir == "" {
		return nil
	}
	units, err := storage.TraverseAll(s.ticketsDir)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.units = make(map[string]*models.WorkUnit, len(units))
	for _, wu := range units {
		s.units[wu.Identifier] = wu
	}
	return nil
}

// Get returns the WorkUnit with the given identifier and true, or nil and
// false if not found.
func (s *State) Get(identifier string) (*models.WorkUnit, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wu, ok := s.units[identifier]
	return wu, ok
}

// All returns a slice containing all work units. The returned slice is a copy;
// the elements are pointers to the same underlying structs held by State.
func (s *State) All() []*models.WorkUnit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*models.WorkUnit, 0, len(s.units))
	for _, wu := range s.units {
		out = append(out, wu)
	}
	return out
}

// FindOpen returns the first open (non-project) ticket whose dependencies are
// all satisfied (status=done). Returns nil if none found.
func (s *State) FindOpen() *models.WorkUnit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, wu := range s.units {
		if wu.IsProject {
			continue
		}
		if wu.Status != models.StatusOpen {
			continue
		}
		if s.unsatisfiedDepsLocked(wu) == 0 {
			return wu
		}
	}
	return nil
}

// FindReviewReady returns the first ticket with status review-ready.
// Returns nil if none found.
func (s *State) FindReviewReady() *models.WorkUnit {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, wu := range s.units {
		if !wu.IsProject && wu.Status == models.StatusReviewReady {
			return wu
		}
	}
	return nil
}

// Update modifies a work unit in memory, refreshes its LastUpdated timestamp,
// and writes it to disk. The work unit must already exist in the state.
func (s *State) Update(wu *models.WorkUnit) error {
	wu.LastUpdated = time.Now().UTC()

	s.mu.Lock()
	s.units[wu.Identifier] = wu
	s.mu.Unlock()

	return s.writeToDisk(wu)
}

// Add inserts a new work unit into the in-memory map and writes it to disk.
func (s *State) Add(wu *models.WorkUnit) error {
	s.mu.Lock()
	s.units[wu.Identifier] = wu
	s.mu.Unlock()

	return s.writeToDisk(wu)
}

// UnsatisfiedDeps returns the identifiers of wu's dependencies that are not
// yet in the "done" status.
func (s *State) UnsatisfiedDeps(wu *models.WorkUnit) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []string
	for _, dep := range wu.Dependencies {
		depUnit, ok := s.units[dep]
		if !ok || depUnit.Status != models.StatusDone {
			out = append(out, dep)
		}
	}
	return out
}

// Parent returns the parent project of wu and true. If wu is a top-level item
// (Parent field is empty) or the parent is not found, it returns nil, false.
func (s *State) Parent(wu *models.WorkUnit) (*models.WorkUnit, bool) {
	if wu.Parent == "" {
		return nil, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	parent, ok := s.units[wu.Parent]
	return parent, ok
}

// AllDone returns true if all direct children of parentID have status "done".
// Returns true for a parent with no children.
func (s *State) AllDone(parentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, wu := range s.units {
		if wu.Parent == parentID && wu.Status != models.StatusDone {
			return false
		}
	}
	return true
}

// MarkAncestorsInProgress walks up the parent chain from wu and marks every
// ancestor project as in-progress if it is not already.
func (s *State) MarkAncestorsInProgress(wu *models.WorkUnit) {
	parent, ok := s.Parent(wu)
	for ok {
		if parent.Status != models.ProjectInProgress {
			parent.Status = models.ProjectInProgress
			s.Update(parent) //nolint:errcheck
		}
		parent, ok = s.Parent(parent)
	}
}

// unsatisfiedDepsLocked counts unsatisfied dependencies without acquiring the
// lock. Must be called with s.mu held (at least read-locked).
func (s *State) unsatisfiedDepsLocked(wu *models.WorkUnit) int {
	count := 0
	for _, dep := range wu.Dependencies {
		depUnit, ok := s.units[dep]
		if !ok || depUnit.Status != models.StatusDone {
			count++
		}
	}
	return count
}

// writeToDisk serialises wu and writes it to the appropriate path under
// ticketsDir, creating subdirectories as needed.
func (s *State) writeToDisk(wu *models.WorkUnit) error {
	var path string
	if wu.IsProject {
		// Projects are stored as ticketsDir/<identifier>/.project.json
		relPath := filepath.FromSlash(wu.Identifier)
		path = filepath.Join(s.ticketsDir, relPath, ".project.json")
	} else {
		// Tickets are stored as ticketsDir/<identifier>/ticket.json
		relPath := filepath.FromSlash(wu.Identifier)
		path = filepath.Join(s.ticketsDir, relPath, "ticket.json")
	}

	// Ensure the parent directory exists.
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return storage.WriteWorkUnit(path, wu)
}
