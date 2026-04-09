package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/models"
)

// ── 'g' key handler tests ────────────────────────────────────────────────────

func TestLogView_GKey_EmitsOpenDiffViewMsg(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{
				Message: "claimed proj/ticket",
				Logfile: "/repo/.tickets/proj/ticket/implement.log",
			},
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

func TestLogView_GKey_ReviewPhase(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{
				Message: "review output",
				Logfile: "/repo/.tickets/proj/ticket/review.log",
			},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from 'g' key")
	}

	msg := cmd()
	diffMsg := msg.(openDiffViewMsg)
	if diffMsg.phase != "review" {
		t.Errorf("phase = %q, want %q", diffMsg.phase, "review")
	}
}

func TestLogView_GKey_NoEntry_ReturnsNilCmd(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd != nil {
		t.Error("expected nil cmd when no entry is selected")
	}
}

func TestLogView_GKey_EmptyLogfile_ReturnsNilCmd(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{Message: "some message", Logfile: ""},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd != nil {
		t.Error("expected nil cmd when logfile is empty")
	}
}

func TestLogView_GKey_InvalidLogfilePath_ReturnsNilCmd(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{Message: "some message", Logfile: "/random/path.log"},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd != nil {
		t.Error("expected nil cmd when logfile path has no valid identifier")
	}
}

// ── KeyBinding description tests ─────────────────────────────────────────────

func TestLogView_KeyBindings_GDescription(t *testing.T) {
	v := LogView{}
	for _, kb := range v.KeyBindings() {
		if kb.Key == "g" {
			if kb.Description != "View diff" {
				t.Errorf("'g' description = %q, want %q", kb.Description, "View diff")
			}
			return
		}
	}
	t.Error("'g' keybinding not found in LogView.KeyBindings()")
}
