package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/models"
)

func TestNewModel_HasFiveViews(t *testing.T) {
	m := NewModel(nil, nil, 5)
	for i := 0; i < 5; i++ {
		if m.views[i] == nil {
			t.Errorf("views[%d] is nil", i)
		}
	}
}

func TestNewModel_ViewDiffIsDiffView(t *testing.T) {
	m := NewModel(nil, nil, 5)
	if _, ok := m.views[ViewDiff].(DiffView); !ok {
		t.Errorf("views[ViewDiff] is %T, want DiffView", m.views[ViewDiff])
	}
}

func TestF5SwitchesToDiffsView(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	// Ensure we start on a different view.
	m.activeView = ViewProject

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF5})
	model := updated.(Model)
	if model.activeView != ViewDiff {
		t.Errorf("after F5, activeView = %d, want %d (ViewDiff)", model.activeView, ViewDiff)
	}
}

func TestRenderHeader_ShowsFiveTabs(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	header := m.renderHeader()
	for _, label := range []string{"F1:", "F2:", "F3:", "F4:", "F5:"} {
		if !strings.Contains(header, label) {
			t.Errorf("header missing %q tab label", label)
		}
	}
}

func TestRenderHeader_ShowsDiffsTab(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	header := m.renderHeader()
	if !strings.Contains(header, "Diffs") {
		t.Error("header missing 'Diffs' tab name")
	}
}

func TestGlobalKeyBindings_ContainsF5(t *testing.T) {
	found := false
	for _, kb := range globalKeyBindings {
		if kb.Key == "F5" {
			found = true
			break
		}
	}
	if !found {
		t.Error("globalKeyBindings missing F5 entry")
	}
}

// ── openDiffViewMsg tests ────────────────────────────────────────────────────

func TestOpenDiffViewMsg_HasExpectedFields(t *testing.T) {
	msg := openDiffViewMsg{identifier: "proj/ticket", phase: "implement"}
	if msg.identifier != "proj/ticket" {
		t.Errorf("identifier = %q, want %q", msg.identifier, "proj/ticket")
	}
	if msg.phase != "implement" {
		t.Errorf("phase = %q, want %q", msg.phase, "implement")
	}
}

func TestOpenDiffViewMsg_SwitchesToDiffView(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	m.activeView = ViewCommand

	updated, _ := m.Update(openDiffViewMsg{identifier: "proj/ticket", phase: "implement"})
	model := updated.(Model)
	if model.activeView != ViewDiff {
		t.Errorf("after openDiffViewMsg, activeView = %d, want %d (ViewDiff)", model.activeView, ViewDiff)
	}
}

func TestOpenDiffViewMsg_SetsDiffViewTicketContext(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40
	m.activeView = ViewCommand

	updated, _ := m.Update(openDiffViewMsg{identifier: "proj/ticket", phase: "review"})
	model := updated.(Model)
	dv, ok := model.views[ViewDiff].(DiffView)
	if !ok {
		t.Fatal("views[ViewDiff] is not a DiffView")
	}
	if dv.identifier != "proj/ticket" {
		t.Errorf("DiffView.identifier = %q, want %q", dv.identifier, "proj/ticket")
	}
	if dv.phase != "review" {
		t.Errorf("DiffView.phase = %q, want %q", dv.phase, "review")
	}
}

func TestOpenDiffViewMsg_ResetsSelectionToHead(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	// Pre-populate the DiffView with some state to verify reset.
	dv := m.views[ViewDiff].(DiffView)
	dv.cursor = 5
	dv.anchor = 3
	m.views[ViewDiff] = dv

	updated, _ := m.Update(openDiffViewMsg{identifier: "proj/ticket", phase: "implement"})
	model := updated.(Model)
	dvAfter := model.views[ViewDiff].(DiffView)
	if dvAfter.cursor != 0 {
		t.Errorf("DiffView.cursor = %d, want 0 (HEAD)", dvAfter.cursor)
	}
	if dvAfter.anchor != 0 {
		t.Errorf("DiffView.anchor = %d, want 0 (HEAD)", dvAfter.anchor)
	}
}

func TestOpenDiffViewMsg_SetsViewerToNil(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, _ := m.Update(openDiffViewMsg{identifier: "proj/ticket", phase: "implement"})
	model := updated.(Model)
	dv := model.views[ViewDiff].(DiffView)
	if dv.viewer != nil {
		t.Error("DiffView.viewer should be nil after openDiffViewMsg (commit selector screen)")
	}
}

func TestOpenDiffViewMsg_ReturnsFetchCmd(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	_, cmd := m.Update(openDiffViewMsg{identifier: "proj/ticket", phase: "implement"})
	// The cmd should be non-nil: it should trigger a commit list fetch.
	// We can't inspect the exact cmd, but we can verify it's not nil.
	if cmd == nil {
		t.Error("openDiffViewMsg should return a non-nil cmd to fetch commits")
	}
}

// ── openViewChangeRequestDialogMsg tests ─────────────────────────────────────

// TestOpenViewChangeRequestDialogMsg_CreatesDialog verifies that the root model
// creates a ViewChangeRequestDialog when receiving openViewChangeRequestDialogMsg.
func TestOpenViewChangeRequestDialogMsg_CreatesDialog(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	cr := models.ChangeRequest{
		CodeLocation: "main.go:42",
		Description:  "Please fix this",
		Author:       "alice",
		Status:       "Open",
	}

	updated, cmd := m.Update(openViewChangeRequestDialogMsg{
		cr:           cr,
		identifier:   "proj/ticket",
		worktreePath: "/tmp/worktree",
	})
	model := updated.(Model)
	if model.dialog == nil {
		t.Fatal("expected dialog to be set after openViewChangeRequestDialogMsg")
	}
	if _, ok := model.dialog.(ViewChangeRequestDialog); !ok {
		t.Errorf("dialog is %T, want ViewChangeRequestDialog", model.dialog)
	}
	if cmd != nil {
		t.Error("expected nil cmd from openViewChangeRequestDialogMsg")
	}
}

// TestOpenViewChangeRequestDialogMsg_DialogHasCRData verifies the dialog
// contains the correct CR data.
func TestOpenViewChangeRequestDialogMsg_DialogHasCRData(t *testing.T) {
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	cr := models.ChangeRequest{
		CodeLocation: "main.go:42",
		Description:  "Please fix this",
		Author:       "alice",
		Status:       "Open",
	}

	updated, _ := m.Update(openViewChangeRequestDialogMsg{
		cr:           cr,
		identifier:   "proj/ticket",
		worktreePath: "/tmp/worktree",
	})
	model := updated.(Model)
	d, ok := model.dialog.(ViewChangeRequestDialog)
	if !ok {
		t.Fatal("dialog is not a ViewChangeRequestDialog")
	}
	if d.cr.CodeLocation != "main.go:42" {
		t.Errorf("dialog cr.CodeLocation = %q, want %q", d.cr.CodeLocation, "main.go:42")
	}
	if d.cr.Description != "Please fix this" {
		t.Errorf("dialog cr.Description = %q, want %q", d.cr.Description, "Please fix this")
	}
	if d.identifier != "proj/ticket" {
		t.Errorf("dialog identifier = %q, want %q", d.identifier, "proj/ticket")
	}
	if d.worktreePath != "/tmp/worktree" {
		t.Errorf("dialog worktreePath = %q, want %q", d.worktreePath, "/tmp/worktree")
	}
}
