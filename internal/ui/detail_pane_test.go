package ui

import (
	"strings"
	"testing"

	"github.com/fimmtiu/tickets/internal/models"
)

func TestDetailPaneSetUnit(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "A test ticket",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	if dp.unit == nil {
		t.Error("expected unit to be set")
	}
	if dp.scrollY != 0 {
		t.Errorf("expected scrollY to reset to 0 when setting unit, got %d", dp.scrollY)
	}
}

func TestDetailPaneScrollDown(t *testing.T) {
	dp := DetailPane{}

	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "Line1\nLine2\nLine3\nLine4\nLine5\nLine6\nLine7\nLine8\nLine9\nLine10",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	dp.ScrollDown()
	if dp.scrollY != 1 {
		t.Errorf("expected scrollY 1 after ScrollDown, got %d", dp.scrollY)
	}
}

func TestDetailPaneScrollUp(t *testing.T) {
	dp := DetailPane{}

	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "Line1\nLine2\nLine3\nLine4\nLine5\nLine6",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	dp.scrollY = 2
	dp.ScrollUp()
	if dp.scrollY != 1 {
		t.Errorf("expected scrollY 1 after ScrollUp, got %d", dp.scrollY)
	}
}

func TestDetailPaneScrollUpAtTop(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "Some description",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	dp.scrollY = 0
	dp.ScrollUp()
	if dp.scrollY != 0 {
		t.Errorf("expected scrollY to stay at 0, got %d", dp.scrollY)
	}
}

func TestDetailPaneViewShowsStatus(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:   "proj/ticket-1",
		Description:  "A test ticket description",
		Status:       models.StatusInProgress,
		Dependencies: []string{},
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20)
	if !strings.Contains(view, "in-progress") {
		t.Errorf("expected view to contain status 'in-progress', got: %q", view)
	}
}

func TestDetailPaneViewShowsDependencies(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:   "proj/ticket-1",
		Description:  "A test ticket description",
		Status:       models.StatusBlocked,
		Dependencies: []string{"proj/ticket-0", "proj/ticket-x"},
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20)
	if !strings.Contains(view, "proj/ticket-0") {
		t.Errorf("expected view to contain dependency 'proj/ticket-0', got: %q", view)
	}
}

func TestDetailPaneViewShowsDescription(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "This is the unique description text",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20)
	if !strings.Contains(view, "unique description text") {
		t.Errorf("expected view to contain description, got: %q", view)
	}
}

func TestDetailPaneViewNilUnit(t *testing.T) {
	dp := DetailPane{}
	// No unit set — should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("View() panicked with nil unit: %v", r)
		}
	}()
	view := dp.View(80, 20)
	_ = view
}

func TestDetailPaneViewHeightMatchesRequested(t *testing.T) {
	// The detail pane has a BorderTop which costs 1 row. View must render to
	// exactly height rows total (not height+1) so the overall frame fits the
	// terminal without scrolling the first line off-screen.
	for _, height := range []int{10, 12, 20, 24} {
		dp := DetailPane{}
		unit := &models.WorkUnit{
			Identifier:  "proj/ticket-1",
			Description: "some description",
			Status:      models.StatusOpen,
		}
		dp.SetUnit(unit)

		view := dp.View(80, height)
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Errorf("View(80, %d): got %d lines, want %d", height, len(lines), height)
		}
	}
}

func TestDetailPaneNilViewHeightMatchesRequested(t *testing.T) {
	// Same contract for the "no item selected" state.
	dp := DetailPane{}
	view := dp.View(80, 12)
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Errorf("nil View(80, 12): got %d lines, want 12", len(lines))
	}
}

func TestDetailPaneScrollDoesNotGoBeyondContent(t *testing.T) {
	dp := DetailPane{}

	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "Short description",
		Status:      models.StatusOpen,
	}
	dp.SetUnit(unit)

	// Scroll down many times
	for i := 0; i < 100; i++ {
		dp.ScrollDown()
	}

	// scrollY should be clamped; should not be 100
	if dp.scrollY >= 100 {
		t.Errorf("expected scrollY to be clamped, got %d", dp.scrollY)
	}
}
