package daemon_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
)

// makeTempTicketsDir creates a temp directory initialised as a tickets dir.
func makeTempTicketsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	ticketsDir := filepath.Join(dir, ".tickets")
	if err := storage.InitTicketsDir(dir); err != nil {
		t.Fatalf("InitTicketsDir: %v", err)
	}
	return ticketsDir
}

// writeTicket writes a ticket JSON file directly into ticketsDir using the
// new directory layout: ticketsDir/<identifier>/ticket.json.
func writeTicket(t *testing.T, ticketsDir string, wu *models.WorkUnit) {
	t.Helper()
	ticketDir := filepath.Join(ticketsDir, filepath.FromSlash(wu.Identifier))
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(ticketDir, "ticket.json")
	if err := storage.WriteWorkUnit(path, wu); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}
}

// TestStateLoad verifies that Load populates the in-memory unit map.
func TestStateLoad(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu := models.NewTicket("my-ticket", "a test ticket")
	writeTicket(t, ticketsDir, wu)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, ok := s.Get("my-ticket")
	if !ok {
		t.Fatal("expected to find 'my-ticket' after Load")
	}
	if got.Description != "a test ticket" {
		t.Errorf("expected description 'a test ticket', got %q", got.Description)
	}
}

// TestStateAll verifies that All returns all units.
func TestStateAll(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	for _, id := range []string{"alpha", "beta", "gamma"} {
		wu := models.NewTicket(id, id+" description")
		writeTicket(t, ticketsDir, wu)
	}

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	all := s.All()
	if len(all) != 3 {
		t.Errorf("expected 3 units, got %d", len(all))
	}
}

// TestStateFindClaimable_Open verifies that FindClaimable returns an open ticket.
func TestStateFindClaimable_Open(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	a := models.NewTicket("ticket-a", "open ticket")
	writeTicket(t, ticketsDir, a)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := s.FindClaimable()
	if found == nil {
		t.Fatal("expected FindClaimable to return a ticket, got nil")
	}
	if found.Identifier != "ticket-a" {
		t.Errorf("expected 'ticket-a', got %q", found.Identifier)
	}
}

// TestStateFindClaimable_InProgress verifies that FindClaimable returns an
// unclaimed in-progress ticket.
func TestStateFindClaimable_InProgress(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	a := models.NewTicket("ticket-a", "in-progress ticket")
	a.Status = models.StatusInProgress
	writeTicket(t, ticketsDir, a)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	found := s.FindClaimable()
	if found == nil {
		t.Fatal("expected FindClaimable to return a ticket, got nil")
	}
}

// TestStateFindClaimable_SkipsBlocked verifies that FindClaimable skips
// blocked tickets.
func TestStateFindClaimable_SkipsBlocked(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	a := models.NewTicket("ticket-a", "blocked")
	a.Status = models.StatusBlocked
	writeTicket(t, ticketsDir, a)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if found := s.FindClaimable(); found != nil {
		t.Errorf("expected nil, got %q", found.Identifier)
	}
}

// TestStateFindClaimable_SkipsDone verifies that FindClaimable skips done tickets.
func TestStateFindClaimable_SkipsDone(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	a := models.NewTicket("ticket-a", "done")
	a.Status = models.StatusDone
	writeTicket(t, ticketsDir, a)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if found := s.FindClaimable(); found != nil {
		t.Errorf("expected nil, got %q", found.Identifier)
	}
}

// TestStateFindClaimable_SkipsClaimed verifies that FindClaimable skips tickets
// already claimed by another process.
func TestStateFindClaimable_SkipsClaimed(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	a := models.NewTicket("ticket-a", "claimed")
	a.ClaimedBy = "99"
	writeTicket(t, ticketsDir, a)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if found := s.FindClaimable(); found != nil {
		t.Errorf("expected nil, got %q", found.Identifier)
	}
}

// TestStateUpdate verifies that Update modifies the in-memory state and writes
// to disk.
func TestStateUpdate(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu := models.NewTicket("upd-ticket", "update me")
	writeTicket(t, ticketsDir, wu)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, _ := s.Get("upd-ticket")
	got.Status = models.StatusInProgress
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	// In-memory check.
	got2, ok := s.Get("upd-ticket")
	if !ok {
		t.Fatal("expected unit to still exist after Update")
	}
	if got2.Status != models.StatusInProgress {
		t.Errorf("expected in-progress, got %q", got2.Status)
	}

	// Disk check: reload from storage.
	s2 := daemon.NewState(ticketsDir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load2: %v", err)
	}
	got3, ok := s2.Get("upd-ticket")
	if !ok {
		t.Fatal("expected unit to exist after reload")
	}
	if got3.Status != models.StatusInProgress {
		t.Errorf("expected in-progress on disk, got %q", got3.Status)
	}
}

// TestStateAdd verifies that Add inserts a new unit into memory and disk.
func TestStateAdd(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	wu := models.NewTicket("new-ticket", "brand new")
	if err := s.Add(wu); err != nil {
		t.Fatalf("Add: %v", err)
	}

	got, ok := s.Get("new-ticket")
	if !ok {
		t.Fatal("expected to find new-ticket after Add")
	}
	if got.Description != "brand new" {
		t.Errorf("expected 'brand new', got %q", got.Description)
	}

	// Disk check.
	s2 := daemon.NewState(ticketsDir)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load2: %v", err)
	}
	_, ok = s2.Get("new-ticket")
	if !ok {
		t.Fatal("expected new-ticket on disk after Add")
	}
}

// TestStateUnsatisfiedDeps verifies UnsatisfiedDeps returns only not-done deps.
func TestStateUnsatisfiedDeps(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	done := models.NewTicket("done-dep", "done")
	done.Status = models.StatusDone
	writeTicket(t, ticketsDir, done)

	open := models.NewTicket("open-dep", "open")
	open.Status = models.StatusOpen
	writeTicket(t, ticketsDir, open)

	wu := models.NewTicket("my-ticket", "has deps")
	wu.Dependencies = []string{"done-dep", "open-dep"}
	writeTicket(t, ticketsDir, wu)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ticket, _ := s.Get("my-ticket")
	unsatisfied := s.UnsatisfiedDeps(ticket)
	if len(unsatisfied) != 1 {
		t.Errorf("expected 1 unsatisfied dep, got %d: %v", len(unsatisfied), unsatisfied)
	}
	if len(unsatisfied) > 0 && unsatisfied[0] != "open-dep" {
		t.Errorf("expected 'open-dep', got %q", unsatisfied[0])
	}
}

// TestStateParent verifies that Parent finds the parent project of a ticket.
func TestStateParent(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	// Create a project directory with a ticket inside.
	projDir := filepath.Join(ticketsDir, "my-project")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("my-project", "a project")
	if err := storage.WriteWorkUnit(filepath.Join(projDir, "project.json"), proj); err != nil {
		t.Fatalf("WriteWorkUnit project: %v", err)
	}

	ticketDir := filepath.Join(projDir, "sub-ticket")
	if err := os.MkdirAll(ticketDir, 0755); err != nil {
		t.Fatalf("MkdirAll ticket dir: %v", err)
	}
	ticket := models.NewTicket("my-project/sub-ticket", "nested")
	if err := storage.WriteWorkUnit(filepath.Join(ticketDir, "ticket.json"), ticket); err != nil {
		t.Fatalf("WriteWorkUnit ticket: %v", err)
	}

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	child, ok := s.Get("my-project/sub-ticket")
	if !ok {
		t.Fatal("expected to find my-project/sub-ticket")
	}

	parent, ok := s.Parent(child)
	if !ok {
		t.Fatal("expected Parent to find my-project")
	}
	if parent.Identifier != "my-project" {
		t.Errorf("expected parent 'my-project', got %q", parent.Identifier)
	}
}

// TestStateAllDone verifies AllDone returns true only when all children are done.
func TestStateAllDone(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	projDir := filepath.Join(ticketsDir, "proj")
	if err := os.MkdirAll(projDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	proj := models.NewProject("proj", "parent")
	if err := storage.WriteWorkUnit(filepath.Join(projDir, "project.json"), proj); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}

	doneChildDir := filepath.Join(projDir, "done-child")
	if err := os.MkdirAll(doneChildDir, 0755); err != nil {
		t.Fatalf("MkdirAll done-child: %v", err)
	}
	doneChild := models.NewTicket("proj/done-child", "done")
	doneChild.Status = models.StatusDone
	if err := storage.WriteWorkUnit(filepath.Join(doneChildDir, "ticket.json"), doneChild); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}

	openChildDir := filepath.Join(projDir, "open-child")
	if err := os.MkdirAll(openChildDir, 0755); err != nil {
		t.Fatalf("MkdirAll open-child: %v", err)
	}
	openChild := models.NewTicket("proj/open-child", "open")
	if err := storage.WriteWorkUnit(filepath.Join(openChildDir, "ticket.json"), openChild); err != nil {
		t.Fatalf("WriteWorkUnit: %v", err)
	}

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if s.AllDone("proj") {
		t.Error("expected AllDone=false when some children are not done")
	}

	// Mark the open child as done.
	child, _ := s.Get("proj/open-child")
	child.Status = models.StatusDone
	if err := s.Update(child); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if !s.AllDone("proj") {
		t.Error("expected AllDone=true after all children marked done")
	}
}

// TestStateGetEmpty verifies Get returns false when unit not found.
func TestStateGetEmpty(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, ok := s.Get("nonexistent")
	if ok {
		t.Error("expected Get to return false for nonexistent identifier")
	}
}

// TestStateFindClaimableNone verifies FindClaimable returns nil when nothing
// is available.
func TestStateFindClaimableNone(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)
	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	if found := s.FindClaimable(); found != nil {
		t.Errorf("expected nil from FindClaimable on empty state, got %q", found.Identifier)
	}
}

// TestStateParentNone verifies Parent returns false for top-level units.
func TestStateParentNone(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu := models.NewTicket("top-ticket", "top level")
	writeTicket(t, ticketsDir, wu)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	ticket, _ := s.Get("top-ticket")
	_, ok := s.Parent(ticket)
	if ok {
		t.Error("expected Parent to return false for top-level ticket")
	}
}

// TestStateUpdateTimestamp verifies that Update refreshes LastUpdated.
func TestStateUpdateTimestamp(t *testing.T) {
	ticketsDir := makeTempTicketsDir(t)

	wu := models.NewTicket("ts-ticket", "timestamp test")
	wu.LastUpdated = time.Now().UTC().Add(-time.Hour)
	writeTicket(t, ticketsDir, wu)

	s := daemon.NewState(ticketsDir)
	if err := s.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}

	got, _ := s.Get("ts-ticket")
	oldTime := got.LastUpdated
	got.Status = models.StatusInProgress
	if err := s.Update(got); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got2, _ := s.Get("ts-ticket")
	if !got2.LastUpdated.After(oldTime) {
		t.Errorf("expected LastUpdated to be refreshed after Update")
	}
}
