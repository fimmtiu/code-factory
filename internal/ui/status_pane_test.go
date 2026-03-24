package ui

import (
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/models"
)

func TestStatusPaneShowsRepoName(t *testing.T) {
	sp := StatusPane{repoName: "my-cool-project"}
	view := sp.View(nil, 40, 15)
	if !strings.Contains(view, "my-cool-project") {
		t.Errorf("expected repo name in title, got: %q", view)
	}
}

func TestStatusPaneFallsBackToStatusWhenNoRepoName(t *testing.T) {
	sp := StatusPane{}
	view := sp.View(nil, 40, 15)
	if !strings.Contains(view, "Status") {
		t.Errorf("expected fallback title 'Status', got: %q", view)
	}
}

func TestStatusPaneShowsTickets(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()
	view := sp.View(units, 40, 15)

	if !strings.Contains(view, "Tickets") {
		t.Error("expected status pane to mention tickets")
	}
}

func TestStatusPaneDoesNotShowProjects(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()
	view := sp.View(units, 40, 15)

	if strings.Contains(view, "Projects") {
		t.Error("expected status pane NOT to show project statistics")
	}
}

func TestStatusPaneEmpty(t *testing.T) {
	sp := StatusPane{}
	view := sp.View(nil, 40, 10)
	if view == "" {
		t.Error("expected non-empty view even with no units")
	}
}

func TestStatusPaneCounts(t *testing.T) {
	sp := StatusPane{}

	units := []*models.WorkUnit{
		{Identifier: "proj-a", IsProject: true},
		{Identifier: "proj-a/ticket-1", IsProject: false, Phase: models.PhaseImplement, Status: models.StatusIdle},
		{Identifier: "proj-a/ticket-2", IsProject: false, Phase: models.PhaseImplement, Status: models.StatusInProgress},
		{Identifier: "proj-a/ticket-3", IsProject: false, Phase: models.PhaseDone, Status: models.StatusIdle},
	}

	view := sp.View(units, 60, 15)

	// 3 tickets total: 1 idle, 1 in-progress, 1 done
	if !strings.Contains(view, "3") {
		t.Errorf("expected view to contain count '3', got: %q", view)
	}
}

func TestStatusPaneDimensions(t *testing.T) {
	sp := StatusPane{}
	units := sampleUnits()

	_ = sp.View(units, 20, 5)
	_ = sp.View(units, 80, 20)
	_ = sp.View(units, 0, 0)
}
