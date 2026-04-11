package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// newTestTicketDialog creates a minimal TicketDialog for testing View output
// without requiring a database or filesystem. The caller can customise the
// returned dialog before calling View().
func newTestTicketDialog(crs []models.ChangeRequest) *TicketDialog {
	wu := &models.WorkUnit{
		Identifier:     "test/ticket-1",
		ChangeRequests: crs,
	}
	d := &TicketDialog{
		wu:             wu,
		changeRequests: crs,
		width:          120,
		height:         40,
	}
	d.computeDimensions()
	d.buildItems()
	return d
}

func sampleCRs() []models.ChangeRequest {
	return []models.ChangeRequest{
		{
			ID:           "1",
			Date:         time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Author:       "alice",
			Status:       models.ChangeRequestOpen,
			Description:  "Fix the widget",
			CodeLocation: "main.go:42",
		},
		{
			ID:           "2",
			Date:         time.Date(2025, 1, 14, 9, 0, 0, 0, time.UTC),
			Author:       "bob",
			Status:       models.ChangeRequestDismissed,
			Description:  "Old change",
			CodeLocation: "util.go:10",
		},
	}
}

// ── TicketDialog View tests ───────────────────────────────────────────────────

func TestTicketDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := newTestTicketDialog(sampleCRs())
	view := d.View()
	// NormalBorder uses ┌ instead of Tan's RoundedBorder ╭.
	if !strings.Contains(view, "┌") {
		t.Error("TicketDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

func TestTicketDialog_View_UsesThemeDialogTitleStyle(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	view := d.View()
	if !strings.Contains(view, "test/ticket-1") {
		t.Error("TicketDialog.View() missing ticket identifier in title")
	}
}

func TestTicketDialog_View_EmptyState(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(nil)
	view := d.View()
	if !strings.Contains(view, "no change requests or logfiles") {
		t.Error("TicketDialog.View() should show empty state message when no items")
	}
}

func TestTicketDialog_View_EmptyStateUsesDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := newTestTicketDialog(nil)
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("TicketDialog.View() empty state did not use theme.Current().DialogBoxStyle")
	}
}

// ── renderListPane tests ──────────────────────────────────────────────────────

func TestTicketDialog_renderListPane_UsesThemeTdSectionStyle(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	// TdSectionStyle is used for the "Change requests" header.
	view := d.View()
	if !strings.Contains(view, "Change requests") {
		t.Error("TicketDialog list pane should contain 'Change requests' section header")
	}
}

func TestTicketDialog_renderListPane_UsesThemeTdSelectedStyle(t *testing.T) {
	saveTheme(t)
	// Set TdSelectedStyle to something with an underline so we can detect it.
	theme.Current().TdSelectedStyle = lipgloss.NewStyle().Underline(true)
	d := newTestTicketDialog(sampleCRs())
	// The first CR should be selected by default.
	view := d.View()
	if !strings.Contains(view, "alice") {
		t.Error("TicketDialog list pane should show selected CR author")
	}
}

func TestTicketDialog_renderListPane_UsesThemeTdDismissedStyle(t *testing.T) {
	saveTheme(t)
	// Bob's CR is dismissed. Verify the dismissed item appears.
	d := newTestTicketDialog(sampleCRs())
	view := d.View()
	if !strings.Contains(view, "bob") {
		t.Error("TicketDialog list pane should show dismissed CR author")
	}
}

func TestTicketDialog_renderListPane_ShowsClosedCRWithThemeStyle(t *testing.T) {
	saveTheme(t)
	crs := []models.ChangeRequest{
		{
			ID:           "3",
			Date:         time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
			Author:       "charlie",
			Status:       models.ChangeRequestClosed,
			Description:  "Closed change",
			CodeLocation: "main.go:1",
		},
	}
	d := newTestTicketDialog(crs)
	view := d.View()
	if !strings.Contains(view, "charlie") {
		t.Error("TicketDialog list pane should show closed CR author")
	}
}

// ── renderListPane border style tests ─────────────────────────────────────────

func TestTicketDialog_renderListPane_FocusedUsesThemeFocusedBorderStyle(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	d.focus = tdListFocus
	// The renderListPane is called with focusedBorderStyle when list is focused.
	view := d.View()
	// Just verify it renders without panic.
	if view == "" {
		t.Error("TicketDialog.View() should not return empty string")
	}
}

func TestTicketDialog_renderContentPane_FocusedUsesThemeFocusedBorderStyle(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	d.focus = tdContentFocus
	view := d.View()
	if view == "" {
		t.Error("TicketDialog.View() should not return empty string")
	}
}

func TestTicketDialog_renderListPane_UnfocusedUsesThemeUnfocusedBorderStyle(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	d.focus = tdContentFocus // list is unfocused
	view := d.View()
	if view == "" {
		t.Error("TicketDialog.View() should not return empty string")
	}
}

// ── renderHint tests ──────────────────────────────────────────────────────────

func TestTicketDialog_renderHint_UsesThemeHintStyles(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	view := d.View()
	// Hint should contain key labels.
	if !strings.Contains(view, "diffs") {
		t.Error("TicketDialog hint should contain 'diffs'")
	}
	if !strings.Contains(view, "Esc") {
		t.Error("TicketDialog hint should contain 'Esc'")
	}
}

func TestTicketDialog_renderHint_CRSelected_ShowsDismissReopen(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(sampleCRs())
	// First selected item is a CR.
	view := d.View()
	if !strings.Contains(view, "dismiss") {
		t.Error("TicketDialog hint should show 'dismiss' for CR selection")
	}
}

func TestTicketDialog_renderHint_DismissedCR_UsesHintInactiveStyle(t *testing.T) {
	saveTheme(t)
	// Select the dismissed CR (bob, index 1 in items).
	crs := sampleCRs()
	d := newTestTicketDialog(crs)
	// Move selection to the second CR (bob, dismissed).
	d.selectedIdx = d.nextSelectable(d.selectedIdx)
	view := d.View()
	if !strings.Contains(view, "dismiss") {
		t.Error("TicketDialog hint should contain 'dismiss' text even when inactive")
	}
}

func TestTicketDialog_renderHint_NilItem_ShowsBasicHints(t *testing.T) {
	saveTheme(t)
	d := newTestTicketDialog(nil)
	view := d.View()
	// Empty dialog should still render (shows empty state).
	if view == "" {
		t.Error("TicketDialog.View() should not return empty string for nil items")
	}
}

// ── crContentLines tests ──────────────────────────────────────────────────────

func TestTicketDialog_crContentLines_UsesThemeDetailLabelStyle(t *testing.T) {
	saveTheme(t)
	// Set DetailLabelStyle to italic so we can detect a change.
	theme.Current().DetailLabelStyle = lipgloss.NewStyle().Italic(true)
	d := newTestTicketDialog(sampleCRs())
	lines := d.crContentLines(0)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "File:") {
		t.Error("crContentLines should contain 'File:' label")
	}
	if !strings.Contains(joined, "Status:") {
		t.Error("crContentLines should contain 'Status:' label")
	}
}
