package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── EditChangeRequestDialog constructor tests ───────────────────────────────

func TestNewEditChangeRequestDialog_ReturnsCorrectType(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "proj/ticket", "main.go", 42, "func main() {}", "/tmp/wt", 80)
	// Verify it's an EditChangeRequestDialog (compile-time check via type assertion).
	var _ EditChangeRequestDialog = d
	if d.fileName != "main.go" {
		t.Errorf("fileName = %q, want %q", d.fileName, "main.go")
	}
	if d.lineNum != 42 {
		t.Errorf("lineNum = %d, want 42", d.lineNum)
	}
	if d.identifier != "proj/ticket" {
		t.Errorf("identifier = %q, want %q", d.identifier, "proj/ticket")
	}
}

func TestNewEditChangeRequestDialog_SetsTextAreaWidth(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	// The text area should have been created (non-zero width).
	if d.width != 80 {
		t.Errorf("width = %d, want 80", d.width)
	}
}

func TestNewEditChangeRequestDialog_InitialFocusIsTextArea(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	if d.focused != crFocusTextArea {
		t.Errorf("focused = %d, want %d (crFocusTextArea)", d.focused, crFocusTextArea)
	}
}

func TestNewEditChangeRequestDialog_MinimumTextWidth(t *testing.T) {
	ensureTheme(t)
	// Very small width should be clamped to minimum.
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 5)
	if d.width != 5 {
		t.Errorf("width = %d, want 5", d.width)
	}
}

// ── EditChangeRequestDialog satisfies dialog interface ──────────────────────

func TestEditChangeRequestDialog_ImplementsDialogInterface(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	// Compile-time check: EditChangeRequestDialog must satisfy the dialog interface.
	var _ dialog = d
}

// ── EditChangeRequestDialog Init ────────────────────────────────────────────

func TestEditChangeRequestDialog_Init_ReturnsNil(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	if cmd := d.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

// ── EditChangeRequestDialog View tests ──────────────────────────────────────

func TestEditChangeRequestDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "main.go", 10, "some context", "/tmp", 80)
	view := d.View()
	if !strings.Contains(view, "New Change Request") {
		t.Error("View() should contain 'New Change Request' title")
	}
}

func TestEditChangeRequestDialog_View_ContainsFileAndLine(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "main.go", 42, "some context", "/tmp", 80)
	view := d.View()
	if !strings.Contains(view, "main.go:42") {
		t.Error("View() should contain file:line info")
	}
}

func TestEditChangeRequestDialog_View_ContainsButtons(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	view := d.View()
	if !strings.Contains(view, "Cancel") {
		t.Error("View() should contain 'Cancel' button")
	}
	if !strings.Contains(view, "OK") {
		t.Error("View() should contain 'OK' button")
	}
}

func TestEditChangeRequestDialog_View_ContainsCodeContext(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "func hello() {}", "/tmp", 80)
	view := d.View()
	if !strings.Contains(view, "func hello()") {
		t.Error("View() should contain code context")
	}
}

func TestEditChangeRequestDialog_View_ShowsErrorMessage(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	d.errMsg = "Description cannot be empty"
	view := d.View()
	if !strings.Contains(view, "Description cannot be empty") {
		t.Error("View() should display error message when set")
	}
}

// ── EditChangeRequestDialog Update tests ────────────────────────────────────

func TestEditChangeRequestDialog_Update_EscDismisses(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("Esc should return a dismiss command")
	}
}

func TestEditChangeRequestDialog_Update_TabCyclesFocus(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	if d.focused != crFocusTextArea {
		t.Fatal("initial focus should be text area")
	}

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyTab})
	d2 := updated.(EditChangeRequestDialog)
	if d2.focused != crFocusCancel {
		t.Errorf("after Tab: focused = %d, want %d (crFocusCancel)", d2.focused, crFocusCancel)
	}

	updated, _ = d2.Update(tea.KeyMsg{Type: tea.KeyTab})
	d3 := updated.(EditChangeRequestDialog)
	if d3.focused != crFocusOK {
		t.Errorf("after 2nd Tab: focused = %d, want %d (crFocusOK)", d3.focused, crFocusOK)
	}

	updated, _ = d3.Update(tea.KeyMsg{Type: tea.KeyTab})
	d4 := updated.(EditChangeRequestDialog)
	if d4.focused != crFocusTextArea {
		t.Errorf("after 3rd Tab: focused = %d, want %d (crFocusTextArea)", d4.focused, crFocusTextArea)
	}
}

func TestEditChangeRequestDialog_Update_ShiftTabCyclesBackward(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	d2 := updated.(EditChangeRequestDialog)
	if d2.focused != crFocusOK {
		t.Errorf("after Shift+Tab from textarea: focused = %d, want %d (crFocusOK)", d2.focused, crFocusOK)
	}
}

func TestEditChangeRequestDialog_Update_CancelEnterDismisses(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	d.focused = crFocusCancel

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("Enter on Cancel button should return a dismiss command")
	}
}

func TestEditChangeRequestDialog_Update_SubmitEmptyShowsError(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	d.focused = crFocusOK

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d2 := updated.(EditChangeRequestDialog)
	if d2.errMsg == "" {
		t.Error("submitting empty description should set errMsg")
	}
	if !strings.Contains(d2.errMsg, "empty") {
		t.Errorf("errMsg = %q, should mention empty description", d2.errMsg)
	}
}

func TestEditChangeRequestDialog_Update_TabClearsError(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 80)
	d.errMsg = "some error"

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyTab})
	d2 := updated.(EditChangeRequestDialog)
	if d2.errMsg != "" {
		t.Errorf("Tab should clear errMsg, got %q", d2.errMsg)
	}
}

// ── openEditChangeRequestDialogMsg tests ────────────────────────────────────

func TestOpenEditChangeRequestDialogMsg_HasExpectedFields(t *testing.T) {
	msg := openEditChangeRequestDialogMsg{
		identifier:   "proj/ticket",
		fileName:     "main.go",
		lineNum:      10,
		context:      "func foo() {}",
		worktreePath: "/tmp/wt",
	}
	if msg.identifier != "proj/ticket" {
		t.Errorf("identifier = %q, want %q", msg.identifier, "proj/ticket")
	}
	if msg.fileName != "main.go" {
		t.Errorf("fileName = %q, want %q", msg.fileName, "main.go")
	}
	if msg.lineNum != 10 {
		t.Errorf("lineNum = %d, want 10", msg.lineNum)
	}
	if msg.context != "func foo() {}" {
		t.Errorf("context = %q, want %q", msg.context, "func foo() {}")
	}
	if msg.worktreePath != "/tmp/wt" {
		t.Errorf("worktreePath = %q, want %q", msg.worktreePath, "/tmp/wt")
	}
}

// ── Integration: app.go routes openEditChangeRequestDialogMsg ───────────────

func TestModel_Update_OpenEditChangeRequestDialogMsg_SetsDialog(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, _ := m.Update(openEditChangeRequestDialogMsg{
		identifier:   "proj/ticket",
		fileName:     "main.go",
		lineNum:      42,
		context:      "some code",
		worktreePath: "/tmp/wt",
	})
	model := updated.(Model)
	if model.dialog == nil {
		t.Fatal("dialog should be set after openEditChangeRequestDialogMsg")
	}
	// Verify the dialog is an EditChangeRequestDialog.
	if _, ok := model.dialog.(EditChangeRequestDialog); !ok {
		t.Errorf("dialog is %T, want EditChangeRequestDialog", model.dialog)
	}
}

// ── Theme integration test for EditChangeRequestDialog ──────────────────────

func TestEditChangeRequestDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	// NormalBorder uses "┌" while the default Tan theme uses RoundedBorder "╭".
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := NewEditChangeRequestDialog(nil, "id", "file.go", 1, "ctx", "/tmp", 120)
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("EditChangeRequestDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}
