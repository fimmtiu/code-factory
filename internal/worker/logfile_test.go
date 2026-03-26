package worker_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fimmtiu/tickets/internal/worker"
)

func TestNextLogfilePath_FirstRun(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	got := worker.NextLogfilePath(ticketsDir, "proj/ticket-1", "implement")
	want := filepath.Join(ticketsDir, "proj", "ticket-1", "implement.log")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNextLogfilePath_SecondRun(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	// Create the base file so it "already exists".
	base := filepath.Join(ticketsDir, "proj", "ticket-1", "implement.log")
	if err := os.WriteFile(base, []byte("previous run"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got := worker.NextLogfilePath(ticketsDir, "proj/ticket-1", "implement")
	want := base + ".1"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNextLogfilePath_ThirdRun(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticketDir := filepath.Join(ticketsDir, "proj", "ticket-1")

	// Create the base and first suffix.
	for _, name := range []string{"implement.log", "implement.log.1"} {
		if err := os.WriteFile(filepath.Join(ticketDir, name), []byte("old run"), 0o644); err != nil {
			t.Fatalf("WriteFile %q: %v", name, err)
		}
	}

	got := worker.NextLogfilePath(ticketsDir, "proj/ticket-1", "implement")
	want := filepath.Join(ticketDir, "implement.log.2")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNextLogfilePath_DifferentPhases(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticketDir := filepath.Join(ticketsDir, "proj", "ticket-1")

	for _, phase := range []string{"implement", "refactor", "review", "respond"} {
		got := worker.NextLogfilePath(ticketsDir, "proj/ticket-1", phase)
		want := filepath.Join(ticketDir, phase+".log")
		if got != want {
			t.Errorf("phase %q: got %q, want %q", phase, got, want)
		}
	}
}
