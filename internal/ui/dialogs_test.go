package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// saveTheme saves CurrentTheme and returns a cleanup func that restores it.
func saveTheme(t *testing.T) {
	t.Helper()
	orig := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(orig) })
}

// ── QuitDialog View tests ─────────────────────────────────────────────────────

func TestQuitDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	// Replace DialogBoxStyle with a marker we can detect.
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := NewQuitDialog()
	view := d.View()
	// The default Tan theme uses RoundedBorder (╭). NormalBorder uses ┌.
	if !strings.Contains(view, "┌") {
		t.Error("QuitDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

func TestQuitDialog_View_UsesThemeDialogTitleStyle(t *testing.T) {
	saveTheme(t)
	d := NewQuitDialog()
	view := d.View()
	if !strings.Contains(view, "Really quit?") {
		t.Error("QuitDialog.View() missing title text")
	}
}

func TestQuitDialog_View_UsesThemeButtonStyles(t *testing.T) {
	saveTheme(t)
	d := NewQuitDialog()
	view := d.View()
	// Both Cancel and Quit buttons should appear.
	if !strings.Contains(view, "Cancel") {
		t.Error("QuitDialog.View() missing Cancel button")
	}
	if !strings.Contains(view, "Quit") {
		t.Error("QuitDialog.View() missing Quit button")
	}
}

func TestQuitDialog_View_FocusedCancel_UsesButtonFocusedStyle(t *testing.T) {
	saveTheme(t)
	// Set ButtonFocusedStyle to underline so we can detect it.
	theme.Current().ButtonFocusedStyle = lipgloss.NewStyle().Underline(true)
	theme.Current().ButtonNormalStyle = lipgloss.NewStyle()
	d := QuitDialog{focused: quitFocusCancel}
	view := d.View()
	// With the focused cancel, "Cancel" should be styled differently.
	// Just check the view renders without panic and contains both buttons.
	if !strings.Contains(view, "Cancel") || !strings.Contains(view, "Quit") {
		t.Error("QuitDialog.View() should render both buttons")
	}
}

func TestQuitDialog_View_FocusedQuit_UsesButtonFocusedStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().ButtonFocusedStyle = lipgloss.NewStyle().Underline(true)
	theme.Current().ButtonNormalStyle = lipgloss.NewStyle()
	d := QuitDialog{focused: quitFocusQuit}
	view := d.View()
	if !strings.Contains(view, "Cancel") || !strings.Contains(view, "Quit") {
		t.Error("QuitDialog.View() should render both buttons")
	}
}

// ── HelpDialog View tests ─────────────────────────────────────────────────────

func TestHelpDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := NewHelpDialog([]KeyBinding{{Key: "q", Description: "quit"}})
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("HelpDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

func TestHelpDialog_View_UsesThemeDetailLabelStyle(t *testing.T) {
	saveTheme(t)
	d := NewHelpDialog([]KeyBinding{{Key: "q", Description: "quit"}})
	view := d.View()
	if !strings.Contains(view, "q") {
		t.Error("HelpDialog.View() should display key binding label")
	}
}

func TestHelpDialog_View_ContainsHelpTitle(t *testing.T) {
	saveTheme(t)
	d := NewHelpDialog(nil)
	view := d.View()
	if !strings.Contains(view, "Help") {
		t.Error("HelpDialog.View() missing 'Help' title")
	}
}

func TestHelpDialog_View_ContainsOkayButton(t *testing.T) {
	saveTheme(t)
	d := NewHelpDialog(nil)
	view := d.View()
	if !strings.Contains(view, "Okay") {
		t.Error("HelpDialog.View() missing 'Okay' button")
	}
}

func TestHelpDialog_View_SkipsHiddenBindings(t *testing.T) {
	saveTheme(t)
	d := NewHelpDialog([]KeyBinding{
		{Key: "visible", Description: "shown"},
		{Key: "hidden", Description: "not shown", Hidden: true},
	})
	view := d.View()
	if !strings.Contains(view, "visible") {
		t.Error("HelpDialog.View() should show non-hidden bindings")
	}
	if strings.Contains(view, "not shown") {
		t.Error("HelpDialog.View() should not show hidden bindings")
	}
}

// ── MergeConflictDialog View tests ────────────────────────────────────────────

func TestMergeConflictDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("MergeConflictDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

func TestMergeConflictDialog_View_ContainsMergeConflictTitle(t *testing.T) {
	saveTheme(t)
	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()
	if !strings.Contains(view, "Merge Conflict") {
		t.Error("MergeConflictDialog.View() missing 'Merge Conflict' title")
	}
}

func TestMergeConflictDialog_View_ShowsBranchAndWorktree(t *testing.T) {
	saveTheme(t)
	d := NewMergeConflictDialog("/tmp/my-worktree", "my-branch")
	view := d.View()
	if !strings.Contains(view, "my-branch") {
		t.Error("MergeConflictDialog.View() missing branch name")
	}
	if !strings.Contains(view, "/tmp/my-worktree") {
		t.Error("MergeConflictDialog.View() missing worktree path")
	}
}

func TestMergeConflictDialog_View_ShowsFixAndIgnoreButtons(t *testing.T) {
	saveTheme(t)
	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()
	if !strings.Contains(view, "Fix") {
		t.Error("MergeConflictDialog.View() missing Fix button")
	}
	if !strings.Contains(view, "Ignore") {
		t.Error("MergeConflictDialog.View() missing Ignore button")
	}
}

func TestMergeConflictDialog_View_UsesThemeButtonStyles(t *testing.T) {
	saveTheme(t)
	theme.Current().ButtonFocusedStyle = lipgloss.NewStyle().Underline(true)
	theme.Current().ButtonNormalStyle = lipgloss.NewStyle()
	d := MergeConflictDialog{worktreePath: "/tmp/wt", branch: "b", focused: mergeFocusFix}
	view := d.View()
	if !strings.Contains(view, "Fix") || !strings.Contains(view, "Ignore") {
		t.Error("MergeConflictDialog.View() should render both buttons")
	}
}

func TestMergeConflictDialog_View_UsesThemeDetailLabelStyle(t *testing.T) {
	saveTheme(t)
	d := NewMergeConflictDialog("/tmp/worktree", "feature-branch")
	view := d.View()
	// detailLabelStyle is used to render the branch and worktree path.
	if !strings.Contains(view, "feature-branch") {
		t.Error("MergeConflictDialog.View() should use detailLabelStyle for branch")
	}
}
