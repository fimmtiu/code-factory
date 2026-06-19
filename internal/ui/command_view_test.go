package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/models"
)

// ── 'g' key handler tests ────────────────────────────────────────────────────

func TestCommandView_GKey_EmitsOpenDiffViewMsg(t *testing.T) {
	v := CommandView{
		width:  120,
		height: 40,
		rows: []listRow{
			{wu: &models.WorkUnit{
				Identifier: "proj/ticket",
				Phase:      models.PhaseImplement,
				Status:     models.StatusNeedsAttention,
			}},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from 'g' key")
	}

	msg := cmd()
	diffMsg, ok := msg.(openDiffViewMsg)
	if !ok {
		t.Fatalf("expected openDiffViewMsg, got %T", msg)
	}
	if diffMsg.identifier != "proj/ticket" {
		t.Errorf("identifier = %q, want %q", diffMsg.identifier, "proj/ticket")
	}
	if diffMsg.phase != "implement" {
		t.Errorf("phase = %q, want %q", diffMsg.phase, "implement")
	}
}

func TestCommandView_GKey_NoTicket_ReturnsNilCmd(t *testing.T) {
	v := CommandView{
		width:  120,
		height: 40,
		rows:   nil, // empty list
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd != nil {
		t.Error("expected nil cmd when no ticket is selected")
	}
}

func TestCommandView_GKey_DifferentPhase(t *testing.T) {
	v := CommandView{
		width:  120,
		height: 40,
		rows: []listRow{
			{wu: &models.WorkUnit{
				Identifier: "proj/ticket",
				Phase:      models.PhaseReview,
				Status:     models.StatusUserReview,
			}},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from 'g' key")
	}

	msg := cmd()
	diffMsg, ok := msg.(openDiffViewMsg)
	if !ok {
		t.Fatalf("expected openDiffViewMsg, got %T", msg)
	}
	if diffMsg.phase != "review" {
		t.Errorf("phase = %q, want %q", diffMsg.phase, "review")
	}
}

// ── buildRows grouping tests ─────────────────────────────────────────────────

func na(id string) *models.WorkUnit {
	return &models.WorkUnit{Identifier: id, Phase: models.PhaseImplement, Status: models.StatusNeedsAttention}
}

func ur(id string) *models.WorkUnit {
	return &models.WorkUnit{Identifier: id, Phase: models.PhaseReview, Status: models.StatusUserReview}
}

func TestBuildRows_GroupsUnderHeaders(t *testing.T) {
	rows := buildRows([]*models.WorkUnit{na("a"), na("b"), ur("c")})

	// header, a, b, blank, header, c
	if len(rows) != 6 {
		t.Fatalf("expected 6 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].header != "Needs attention:" {
		t.Errorf("row 0 header = %q, want %q", rows[0].header, "Needs attention:")
	}
	if rows[1].wu == nil || rows[1].wu.Identifier != "a" {
		t.Errorf("row 1 should be ticket a, got %+v", rows[1])
	}
	if !rows[3].blank {
		t.Errorf("row 3 should be the blank divider, got %+v", rows[3])
	}
	if rows[4].header != "Ready for review:" {
		t.Errorf("row 4 header = %q, want %q", rows[4].header, "Ready for review:")
	}
	if rows[5].wu == nil || rows[5].wu.Identifier != "c" {
		t.Errorf("row 5 should be ticket c, got %+v", rows[5])
	}

	// Only the three ticket rows are selectable.
	for i, row := range rows {
		wantSelectable := i == 1 || i == 2 || i == 5
		if row.selectable() != wantSelectable {
			t.Errorf("row %d selectable = %v, want %v", i, row.selectable(), wantSelectable)
		}
	}
}

func TestBuildRows_EmptyGroupsShowNone(t *testing.T) {
	rows := buildRows(nil)

	// header, none, blank, header, none
	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d: %+v", len(rows), rows)
	}
	if rows[0].header != "Needs attention:" || !rows[1].none {
		t.Errorf("expected NA header then (none), got %+v, %+v", rows[0], rows[1])
	}
	if !rows[2].blank {
		t.Errorf("row 2 should be the blank divider, got %+v", rows[2])
	}
	if rows[3].header != "Ready for review:" || !rows[4].none {
		t.Errorf("expected UR header then (none), got %+v, %+v", rows[3], rows[4])
	}
	for i, row := range rows {
		if row.selectable() {
			t.Errorf("row %d (%+v) should not be selectable", i, row)
		}
	}
}

func TestCommandView_NavigationSkipsNonSelectableRows(t *testing.T) {
	v := CommandView{width: 120, height: 40}
	v.rows = buildRows([]*models.WorkUnit{na("a"), ur("b")})
	// rows: 0 header, 1 a, 2 blank, 3 header, 4 b
	v.clampSelected()
	if v.selected != 1 {
		t.Fatalf("clampSelected should land on first ticket (index 1), got %d", v.selected)
	}

	v.moveDown(1)
	if v.selected != 4 {
		t.Errorf("moveDown should skip blank+header and land on ticket b (index 4), got %d", v.selected)
	}

	v.moveUp(1)
	if v.selected != 1 {
		t.Errorf("moveUp should skip header+blank and land on ticket a (index 1), got %d", v.selected)
	}
}

// ── KeyBinding description tests ─────────────────────────────────────────────

func TestCommandView_KeyBindings_GDescription(t *testing.T) {
	v := CommandView{}
	for _, kb := range v.KeyBindings() {
		if kb.Key == "g" {
			if kb.Description != "View diff" {
				t.Errorf("'g' description = %q, want %q", kb.Description, "View diff")
			}
			return
		}
	}
	t.Error("'g' keybinding not found in CommandView.KeyBindings()")
}
