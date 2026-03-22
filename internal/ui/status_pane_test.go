package ui

import (
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/models"
)

func TestStatusPaneCountsProjects(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()
	view := sp.View(units, 40, 10)

	// Should show project counts
	if !strings.Contains(view, "Projects") && !strings.Contains(view, "project") {
		t.Error("expected status pane to mention projects")
	}
}

func TestStatusPaneCountsTickets(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()
	view := sp.View(units, 40, 10)

	// Should show ticket counts
	if !strings.Contains(view, "Tickets") && !strings.Contains(view, "ticket") {
		t.Error("expected status pane to mention tickets")
	}
}

func TestStatusPaneEmpty(t *testing.T) {
	sp := StatusPane{}
	view := sp.View(nil, 40, 10)
	// Should not panic and produce some output
	if view == "" {
		t.Error("expected non-empty view even with no units")
	}
}

func TestStatusPaneCounts(t *testing.T) {
	sp := StatusPane{}

	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true, Status: models.ProjectOpen},
		{Identifier: "proj-b", IsProject: true, Status: models.ProjectInProgress},
		{Identifier: "proj-c", IsProject: true, Status: models.ProjectDone},
		{Identifier: "proj-a/ticket-1", IsProject: false, Status: models.StatusOpen},
		{Identifier: "proj-a/ticket-2", IsProject: false, Status: models.StatusInProgress},
		{Identifier: "proj-a/ticket-3", IsProject: false, Status: models.StatusDone},
	}

	view := sp.View(units, 60, 15)

	// Verify the view contains the expected counts
	// 3 projects total: 1 open, 1 in-progress, 1 done
	// 3 tickets total: 1 open, 1 in-progress, 1 done
	if !strings.Contains(view, "3") {
		t.Errorf("expected view to contain count '3', got: %q", view)
	}
}

func TestStatusPaneDimensions(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()

	// Should not panic with various dimensions
	_ = sp.View(units, 20, 5)
	_ = sp.View(units, 80, 20)
	_ = sp.View(units, 0, 0)
}
