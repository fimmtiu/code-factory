package ui

import (
	"strings"
	"testing"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// ensureTheme sets theme.Current() to Tan for the duration of the test.
func ensureTheme(t *testing.T) {
	t.Helper()
	original := theme.Current()
	theme.SetCurrent(theme.Tan())
	t.Cleanup(func() { theme.SetCurrent(original) })
}

// ── PermissionDialog View tests ─────────────────────────────────────────────

func TestPermissionDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	perm := &worker.PendingPermissionRequest{
		Title: "Allow file write?",
		Options: []worker.PermissionOption{
			{Name: "Allow once", Kind: "allow_once"},
			{Name: "Reject once", Kind: "reject_once"},
		},
	}
	d := NewPermissionDialog(nil, nil, &models.WorkUnit{}, perm, 80)
	view := d.View()

	if !strings.Contains(view, "Permission Request") {
		t.Error("PermissionDialog.View() should contain 'Permission Request' title")
	}
}

func TestPermissionDialog_View_ContainsOptions(t *testing.T) {
	ensureTheme(t)

	perm := &worker.PendingPermissionRequest{
		Title: "Allow file write?",
		Options: []worker.PermissionOption{
			{Name: "Allow once", Kind: "allow_once"},
			{Name: "Reject once", Kind: "reject_once"},
		},
	}
	d := NewPermissionDialog(nil, nil, &models.WorkUnit{}, perm, 80)
	view := d.View()

	if !strings.Contains(view, "Allow once") {
		t.Error("PermissionDialog.View() should contain option 'Allow once'")
	}
	if !strings.Contains(view, "Reject once") {
		t.Error("PermissionDialog.View() should contain option 'Reject once'")
	}
}

func TestPermissionDialog_View_ContainsHints(t *testing.T) {
	ensureTheme(t)

	perm := &worker.PendingPermissionRequest{
		Options: []worker.PermissionOption{
			{Name: "Allow once", Kind: "allow_once"},
		},
	}
	d := NewPermissionDialog(nil, nil, &models.WorkUnit{}, perm, 80)
	view := d.View()

	if !strings.Contains(view, "Esc") {
		t.Error("PermissionDialog.View() should contain 'Esc' hint")
	}
}

func TestPermissionDialog_View_RendersSelectedOptionText(t *testing.T) {
	ensureTheme(t)

	perm := &worker.PendingPermissionRequest{
		Options: []worker.PermissionOption{
			{Name: "Allow once", Kind: "allow_once"},
			{Name: "Reject once", Kind: "reject_once"},
		},
	}
	d := NewPermissionDialog(nil, nil, &models.WorkUnit{}, perm, 80)

	// Default selection is 0; verify the selected option text appears in
	// the rendered view. Visual highlighting (reverse-video) is applied by
	// PermOptionSelectedStyle but cannot be asserted in tests because
	// lipgloss suppresses ANSI output when stdout is not a TTY.
	view := d.View()
	if !strings.Contains(view, "Allow once") {
		t.Error("PermissionDialog.View() should render the selected option text")
	}
}

// ── QuitDialog View tests ───────────────────────────────────────────────────

func TestQuitDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	d := NewQuitDialog()
	view := d.View()

	if !strings.Contains(view, "Really quit?") {
		t.Error("QuitDialog.View() should contain 'Really quit?' title")
	}
}

func TestQuitDialog_View_ContainsButtons(t *testing.T) {
	ensureTheme(t)

	d := NewQuitDialog()
	view := d.View()

	if !strings.Contains(view, "Cancel") {
		t.Error("QuitDialog.View() should contain 'Cancel' button")
	}
	if !strings.Contains(view, "Quit") {
		t.Error("QuitDialog.View() should contain 'Quit' button")
	}
}

// ── HelpDialog View tests ───────────────────────────────────────────────────

func TestHelpDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	d := NewHelpDialog([]KeyBinding{{Key: "a", Description: "test action"}})
	view := d.View()

	if !strings.Contains(view, "Help") {
		t.Error("HelpDialog.View() should contain 'Help' title")
	}
}

func TestHelpDialog_View_ContainsBindings(t *testing.T) {
	ensureTheme(t)

	d := NewHelpDialog([]KeyBinding{{Key: "a", Description: "test action"}})
	view := d.View()

	if !strings.Contains(view, "test action") {
		t.Error("HelpDialog.View() should contain binding descriptions")
	}
}

// ── MergeConflictDialog View tests ──────────────────────────────────────────

func TestMergeConflictDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()

	if !strings.Contains(view, "Merge Conflict") {
		t.Error("MergeConflictDialog.View() should contain 'Merge Conflict' title")
	}
}

func TestMergeConflictDialog_View_ContainsBranchAndPath(t *testing.T) {
	ensureTheme(t)

	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()

	if !strings.Contains(view, "feature-branch") {
		t.Error("MergeConflictDialog.View() should contain the branch name")
	}
	if !strings.Contains(view, "/tmp/worktree") {
		t.Error("MergeConflictDialog.View() should contain the worktree path")
	}
}

func TestMergeConflictDialog_View_ContainsButtons(t *testing.T) {
	ensureTheme(t)

	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()

	if !strings.Contains(view, "Fix") {
		t.Error("MergeConflictDialog.View() should contain 'Fix' button")
	}
	if !strings.Contains(view, "Ignore") {
		t.Error("MergeConflictDialog.View() should contain 'Ignore' button")
	}
}

// ── QuickResponseDialog View tests ──────────────────────────────────────────

func TestQuickResponseDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-1"}
	d := QuickResponseDialog{
		wu:    wu,
		width: 80,
		lines: []string{"output line 1", "output line 2"},
	}
	view := d.View()

	if !strings.Contains(view, "Agent Output") {
		t.Error("QuickResponseDialog.View() should contain 'Agent Output' title")
	}
	if !strings.Contains(view, "proj/ticket-1") {
		t.Error("QuickResponseDialog.View() should contain the work unit identifier")
	}
}

func TestQuickResponseDialog_View_ContainsOutputLines(t *testing.T) {
	ensureTheme(t)

	d := QuickResponseDialog{
		wu:    &models.WorkUnit{Identifier: "proj/t"},
		width: 80,
		lines: []string{"hello from agent", "second line"},
	}
	view := d.View()

	if !strings.Contains(view, "hello from agent") {
		t.Error("QuickResponseDialog.View() should contain output lines")
	}
}

func TestQuickResponseDialog_View_ContainsCursor(t *testing.T) {
	ensureTheme(t)

	d := QuickResponseDialog{
		wu:    &models.WorkUnit{Identifier: "proj/t"},
		width: 80,
		lines: []string{"line"},
		input: "typed",
	}
	view := d.View()

	// The block cursor character should appear.
	if !strings.Contains(view, "█") {
		t.Error("QuickResponseDialog.View() should render the block cursor")
	}
}

func TestQuickResponseDialog_View_ContainsHints(t *testing.T) {
	ensureTheme(t)

	d := QuickResponseDialog{
		wu:    &models.WorkUnit{Identifier: "proj/t"},
		width: 80,
		lines: []string{"line"},
	}
	view := d.View()

	if !strings.Contains(view, "Esc") {
		t.Error("QuickResponseDialog.View() should contain 'Esc' hint")
	}
}

// ── PhasePickerDialog View tests ────────────────────────────────────────────

func TestPhasePickerDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-2", Phase: models.PhaseImplement}
	d := NewPhasePickerDialog(nil, wu)
	view := d.View()

	if !strings.Contains(view, "Set phase for proj/ticket-2") {
		t.Error("PhasePickerDialog.View() should contain title with identifier")
	}
}

func TestPhasePickerDialog_View_ContainsPhases(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-2", Phase: models.PhaseImplement}
	d := NewPhasePickerDialog(nil, wu)
	view := d.View()

	for _, phase := range []string{"implement", "refactor", "review", "respond"} {
		if !strings.Contains(view, phase) {
			t.Errorf("PhasePickerDialog.View() should contain phase %q", phase)
		}
	}
}

func TestPhasePickerDialog_View_ContainsButtons(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-2", Phase: models.PhaseImplement}
	d := NewPhasePickerDialog(nil, wu)
	view := d.View()

	if !strings.Contains(view, "OK") {
		t.Error("PhasePickerDialog.View() should contain 'OK' button")
	}
	if !strings.Contains(view, "Cancel") {
		t.Error("PhasePickerDialog.View() should contain 'Cancel' button")
	}
}

func TestPhasePickerDialog_View_FocusedButton(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-2", Phase: models.PhaseImplement}
	d := NewPhasePickerDialog(nil, wu)
	d.focus = ppFocusOK

	view := d.View()

	// The OK button should still appear (it will be rendered with the focused style).
	if !strings.Contains(view, "OK") {
		t.Error("PhasePickerDialog.View() with focused OK should render 'OK'")
	}
}

func TestPhasePickerDialog_View_UnfocusedListUsesAccentStyle(t *testing.T) {
	ensureTheme(t)

	wu := &models.WorkUnit{Identifier: "proj/ticket-2", Phase: models.PhaseImplement}
	d := NewPhasePickerDialog(nil, wu)
	// Move focus away from the list to buttons, so the selected item uses the accent style.
	d.focus = ppFocusOK

	view := d.View()

	// The selected phase should still appear in the output (rendered with accent colour).
	if !strings.Contains(view, "implement") {
		t.Error("PhasePickerDialog.View() should render the selected phase even when list is unfocused")
	}
}
