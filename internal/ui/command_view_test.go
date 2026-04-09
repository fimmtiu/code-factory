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
