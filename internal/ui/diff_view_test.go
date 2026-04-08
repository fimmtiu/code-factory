package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewDiffView_ImplementsViewModel(t *testing.T) {
	var v viewModel = NewDiffView()
	_ = v // Compile-time check that DiffView satisfies the viewModel interface.
}

func TestNewDiffView_InitialScreen(t *testing.T) {
	v := NewDiffView()
	if v.screen != screenCommitSelector {
		t.Errorf("initial screen = %d, want screenCommitSelector (%d)", v.screen, screenCommitSelector)
	}
}

func TestNewDiffView_InitialState(t *testing.T) {
	v := NewDiffView()
	if v.currentTicket != "" {
		t.Errorf("currentTicket = %q, want empty", v.currentTicket)
	}
	if v.worktreePath != "" {
		t.Errorf("worktreePath = %q, want empty", v.worktreePath)
	}
	if v.forkPoint != "" {
		t.Errorf("forkPoint = %q, want empty", v.forkPoint)
	}
	if v.startCommit != 0 {
		t.Errorf("startCommit = %d, want 0", v.startCommit)
	}
	if v.endCommit != 0 {
		t.Errorf("endCommit = %d, want 0", v.endCommit)
	}
}

func TestDiffView_EmptyStateWhenNoTicket(t *testing.T) {
	v := NewDiffView()
	v.width = 80
	v.height = 24
	output := v.View()
	if !strings.Contains(output, "No ticket selected") {
		t.Errorf("View() with no ticket should contain 'No ticket selected', got:\n%s", output)
	}
}

func TestDiffView_KeyBindingsReturnsSlice(t *testing.T) {
	v := NewDiffView()
	bindings := v.KeyBindings()
	if bindings == nil {
		t.Fatal("KeyBindings() returned nil")
	}
}

func TestDiffView_InitReturnsNil(t *testing.T) {
	v := NewDiffView()
	cmd := v.Init()
	if cmd != nil {
		t.Error("Init() should return nil for now")
	}
}

func TestDiffView_UpdateHandlesWindowSize(t *testing.T) {
	v := NewDiffView()
	updated, _ := v.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	dv := updated.(DiffView)
	if dv.width != 120 {
		t.Errorf("width = %d, want 120", dv.width)
	}
	if dv.height != 40 {
		t.Errorf("height = %d, want 40", dv.height)
	}
}

func TestDiffView_ScreenEnum(t *testing.T) {
	// Verify the two screen constants exist and have distinct values.
	if screenCommitSelector == screenDiffViewer {
		t.Error("screenCommitSelector and screenDiffViewer should be distinct")
	}
}
