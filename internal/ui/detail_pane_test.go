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
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
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
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
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
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
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
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
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
		Phase:        models.PhaseImplement,
		Status:       models.StatusInProgress,
		Dependencies: []string{},
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20, false)
	if !strings.Contains(view, "implement") {
		t.Errorf("expected view to contain phase 'implement', got: %q", view)
	}
}

func TestDetailPaneViewShowsDependencies(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:   "proj/ticket-1",
		Description:  "A test ticket description",
		Phase:        models.PhaseBlocked,
		Status:       models.StatusIdle,
		Dependencies: []string{"proj/ticket-0", "proj/ticket-x"},
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20, false)
	if !strings.Contains(view, "proj/ticket-0") {
		t.Errorf("expected view to contain dependency 'proj/ticket-0', got: %q", view)
	}
}

func TestDetailPaneViewShowsDescription(t *testing.T) {
	dp := DetailPane{}
	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "This is the unique description text",
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
	}
	dp.SetUnit(unit)

	view := dp.View(80, 20, false)
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
	view := dp.View(80, 20, false)
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
			Phase:       models.PhasePlan,
			Status:      models.StatusIdle,
		}
		dp.SetUnit(unit)

		view := dp.View(80, height, false)
		lines := strings.Split(view, "\n")
		if len(lines) != height {
			t.Errorf("View(80, %d): got %d lines, want %d", height, len(lines), height)
		}
	}
}

func TestDetailPaneNilViewHeightMatchesRequested(t *testing.T) {
	// Same contract for the "no item selected" state.
	dp := DetailPane{}
	view := dp.View(80, 12, false)
	lines := strings.Split(view, "\n")
	if len(lines) != 12 {
		t.Errorf("nil View(80, 12): got %d lines, want 12", len(lines))
	}
}

func TestDetailPanePageDown(t *testing.T) {
	dp := DetailPane{}
	dp.SetUnit(&models.WorkUnit{
		Identifier:  "proj/t",
		Description: strings.Repeat("line\n", 30),
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
	})

	dp.PageDown(5)
	if dp.scrollY != 5 {
		t.Errorf("expected scrollY 5 after PageDown(5), got %d", dp.scrollY)
	}
}

func TestDetailPanePageDownClampsAtEnd(t *testing.T) {
	dp := DetailPane{}
	dp.SetUnit(&models.WorkUnit{
		Identifier:  "proj/t",
		Description: "short",
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
	})

	dp.PageDown(1000)
	lines := dp.buildLines()
	maxScroll := len(lines) - 1
	if dp.scrollY != maxScroll {
		t.Errorf("expected scrollY clamped to %d, got %d", maxScroll, dp.scrollY)
	}
}

func TestDetailPanePageUp(t *testing.T) {
	dp := DetailPane{}
	dp.SetUnit(&models.WorkUnit{
		Identifier:  "proj/t",
		Description: strings.Repeat("line\n", 30),
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
	})
	dp.scrollY = 20

	dp.PageUp(8)
	if dp.scrollY != 12 {
		t.Errorf("expected scrollY 12 after PageUp(8) from 20, got %d", dp.scrollY)
	}
}

func TestDetailPanePageUpClampsAtZero(t *testing.T) {
	dp := DetailPane{}
	dp.SetUnit(&models.WorkUnit{
		Identifier:  "proj/t",
		Description: strings.Repeat("line\n", 30),
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
	})
	dp.scrollY = 3

	dp.PageUp(1000)
	if dp.scrollY != 0 {
		t.Errorf("expected scrollY clamped to 0, got %d", dp.scrollY)
	}
}

func TestDetailPaneScrollDoesNotGoBeyondContent(t *testing.T) {
	dp := DetailPane{}

	unit := &models.WorkUnit{
		Identifier:  "proj/ticket-1",
		Description: "Short description",
		Phase:       models.PhasePlan,
		Status:      models.StatusIdle,
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
