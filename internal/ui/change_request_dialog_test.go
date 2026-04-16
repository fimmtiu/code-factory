package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// testCRLocation returns a default crLocation for tests.
func testCRLocation() crLocation {
	return crLocation{identifier: "id", fileName: "file.go", lineNum: 1, context: "ctx", worktreePath: "/tmp"}
}

// newTestCRDialog creates a default EditChangeRequestDialog for tests (new-CR mode).
func newTestCRDialog(t *testing.T, width int) EditChangeRequestDialog {
	t.Helper()
	ensureTheme(t)
	return NewEditChangeRequestDialog(nil, testCRLocation(), nil, width)
}

// newTestEditCRDialog creates an EditChangeRequestDialog in edit mode for tests.
func newTestEditCRDialog(t *testing.T, width int) EditChangeRequestDialog {
	t.Helper()
	ensureTheme(t)
	return NewEditChangeRequestDialog(nil, testCRLocation(), &models.ChangeRequest{
		ID:          "42",
		Status:      models.ChangeRequestOpen,
		Description: "existing description",
	}, width)
}

// ── EditChangeRequestDialog constructor tests ───────────────────────────────

func TestNewEditChangeRequestDialog_ReturnsCorrectType(t *testing.T) {
	ensureTheme(t)
	loc := crLocation{identifier: "proj/ticket", fileName: "main.go", lineNum: 42, context: "func main() {}", worktreePath: "/tmp/wt"}
	d := NewEditChangeRequestDialog(nil, loc, nil, 80)
	// Verify it's an EditChangeRequestDialog (compile-time check via type assertion).
	var _ EditChangeRequestDialog = d
	if d.location.fileName != "main.go" {
		t.Errorf("fileName = %q, want %q", d.location.fileName, "main.go")
	}
	if d.location.lineNum != 42 {
		t.Errorf("lineNum = %d, want 42", d.location.lineNum)
	}
	if d.location.identifier != "proj/ticket" {
		t.Errorf("identifier = %q, want %q", d.location.identifier, "proj/ticket")
	}
}

func TestNewEditChangeRequestDialog_StoresWidth(t *testing.T) {
	d := newTestCRDialog(t, 80)
	if d.width != 80 {
		t.Errorf("width = %d, want 80", d.width)
	}
}

func TestNewEditChangeRequestDialog_SetsTextAreaWidth(t *testing.T) {
	d := newTestCRDialog(t, 80)
	taWidth := d.textAreaWidth()
	if taWidth <= 0 {
		t.Errorf("textAreaWidth() = %d, want > 0", taWidth)
	}
	if taWidth >= 80 {
		t.Errorf("textAreaWidth() = %d, should be less than outer width 80", taWidth)
	}
}

func TestNewEditChangeRequestDialog_InitialFocusIsTextArea(t *testing.T) {
	d := newTestCRDialog(t, 80)
	if d.focused != crFocusTextArea {
		t.Errorf("focused = %d, want %d (crFocusTextArea)", d.focused, crFocusTextArea)
	}
}

func TestNewEditChangeRequestDialog_MinimumTextWidth(t *testing.T) {
	// Very small width should result in textAreaWidth() clamping to minimum of 20.
	d := newTestCRDialog(t, 5)
	if d.width != 5 {
		t.Errorf("width = %d, want 5", d.width)
	}
	if taWidth := d.textAreaWidth(); taWidth < 20 {
		t.Errorf("textAreaWidth() = %d, want >= 20 (minimum clamp)", taWidth)
	}
}

// ── EditChangeRequestDialog satisfies dialog interface ──────────────────────

func TestEditChangeRequestDialog_ImplementsDialogInterface(t *testing.T) {
	d := newTestCRDialog(t, 80)
	// Compile-time check: EditChangeRequestDialog must satisfy the dialog interface.
	var _ dialog = d
}

// ── EditChangeRequestDialog Init ────────────────────────────────────────────

func TestEditChangeRequestDialog_Init_ReturnsNil(t *testing.T) {
	d := newTestCRDialog(t, 80)
	if cmd := d.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

// ── EditChangeRequestDialog View tests ──────────────────────────────────────

func TestEditChangeRequestDialog_View_ContainsNewTitle(t *testing.T) {
	ensureTheme(t)
	loc := crLocation{identifier: "id", fileName: "main.go", lineNum: 10, context: "some context", worktreePath: "/tmp"}
	d := NewEditChangeRequestDialog(nil, loc, nil, 80)
	view := d.View()
	if !strings.Contains(view, "New Change Request") {
		t.Error("View() should contain 'New Change Request' title when creating")
	}
}

func TestEditChangeRequestDialog_View_ContainsFileAndLine(t *testing.T) {
	ensureTheme(t)
	loc := crLocation{identifier: "id", fileName: "main.go", lineNum: 42, context: "ctx", worktreePath: "/tmp"}
	d := NewEditChangeRequestDialog(nil, loc, nil, 80)
	view := d.View()
	if !strings.Contains(view, "main.go:42") {
		t.Error("View() should contain file:line info")
	}
}

func TestEditChangeRequestDialog_View_ContainsButtons(t *testing.T) {
	d := newTestCRDialog(t, 80)
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
	loc := crLocation{identifier: "id", fileName: "file.go", lineNum: 1, context: "func hello() {}", worktreePath: "/tmp"}
	d := NewEditChangeRequestDialog(nil, loc, nil, 80)
	view := d.View()
	if !strings.Contains(view, "func hello()") {
		t.Error("View() should contain code context")
	}
}

func TestEditChangeRequestDialog_View_ShowsErrorMessage(t *testing.T) {
	d := newTestCRDialog(t, 80)
	d.errMsg = "Description cannot be empty"
	view := d.View()
	if !strings.Contains(view, "Description cannot be empty") {
		t.Error("View() should display error message when set")
	}
}

// ── EditChangeRequestDialog Update tests ────────────────────────────────────

func TestEditChangeRequestDialog_Update_EscDismisses(t *testing.T) {
	d := newTestCRDialog(t, 80)
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Error("Esc should return a dismiss command")
	}
}

func TestEditChangeRequestDialog_Update_TabCyclesFocus(t *testing.T) {
	d := newTestCRDialog(t, 80)
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
	d := newTestCRDialog(t, 80)

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	d2 := updated.(EditChangeRequestDialog)
	if d2.focused != crFocusOK {
		t.Errorf("after Shift+Tab from textarea: focused = %d, want %d (crFocusOK)", d2.focused, crFocusOK)
	}
}

func TestEditChangeRequestDialog_Update_CancelEnterDismisses(t *testing.T) {
	d := newTestCRDialog(t, 80)
	d.focused = crFocusCancel

	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Error("Enter on Cancel button should return a dismiss command")
	}
}

func TestEditChangeRequestDialog_Update_SubmitEmptyShowsError(t *testing.T) {
	d := newTestCRDialog(t, 80)
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
	d := newTestCRDialog(t, 80)
	d.errMsg = "some error"

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyTab})
	d2 := updated.(EditChangeRequestDialog)
	if d2.errMsg != "" {
		t.Errorf("Tab should clear errMsg, got %q", d2.errMsg)
	}
}

// ── Integration: app.go routes openEditChangeRequestDialogMsg ───────────────

func TestModel_Update_OpenEditChangeRequestDialogMsg_SetsDialog(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, _ := m.Update(openEditChangeRequestDialogMsg{
		location: crLocation{
			identifier:   "proj/ticket",
			fileName:     "main.go",
			lineNum:      42,
			context:      "some code",
			worktreePath: "/tmp/wt",
		},
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
	d := NewEditChangeRequestDialog(nil, testCRLocation(), nil, 120)
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("EditChangeRequestDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

// ── Edit mode (existingCR) tests ────────────────────────────────────────────

func TestNewEditChangeRequestDialog_WithExistingCR_StoresField(t *testing.T) {
	d := newTestEditCRDialog(t, 80)
	if d.existingCR == nil {
		t.Fatal("existingCR should be non-nil in edit mode")
	}
	if d.existingCR.ID != "42" {
		t.Errorf("existingCR.ID = %q, want %q", d.existingCR.ID, "42")
	}
}

func TestNewEditChangeRequestDialog_WithExistingCR_PrePopulatesTextArea(t *testing.T) {
	d := newTestEditCRDialog(t, 80)
	got := d.textArea.Value()
	if got != "existing description" {
		t.Errorf("textArea.Value() = %q, want %q", got, "existing description")
	}
}

func TestNewEditChangeRequestDialog_WithNilCR_EmptyTextArea(t *testing.T) {
	d := newTestCRDialog(t, 80)
	if d.existingCR != nil {
		t.Error("existingCR should be nil for new-CR mode")
	}
	if got := d.textArea.Value(); got != "" {
		t.Errorf("textArea.Value() = %q, want empty", got)
	}
}

func TestEditChangeRequestDialog_View_EditMode_ContainsEditTitle(t *testing.T) {
	d := newTestEditCRDialog(t, 80)
	view := d.View()
	if !strings.Contains(view, "Edit Change Request") {
		t.Error("View() should contain 'Edit Change Request' title when editing")
	}
	if strings.Contains(view, "New Change Request") {
		t.Error("View() should not contain 'New Change Request' when editing")
	}
}

func TestEditChangeRequestDialog_View_EditMode_ShowsStatus(t *testing.T) {
	d := newTestEditCRDialog(t, 80)
	view := d.View()
	if !strings.Contains(view, "Status:") {
		t.Error("View() should contain 'Status:' label when editing")
	}
	if !strings.Contains(view, models.ChangeRequestOpen) {
		t.Errorf("View() should contain status %q", models.ChangeRequestOpen)
	}
}

func TestEditChangeRequestDialog_View_NewMode_NoStatusLine(t *testing.T) {
	d := newTestCRDialog(t, 80)
	view := d.View()
	if strings.Contains(view, "Status:") {
		t.Error("View() should not contain 'Status:' when creating a new CR")
	}
}

func TestEditChangeRequestDialog_Submit_EditMode_ReturnsCmd(t *testing.T) {
	d := newTestEditCRDialog(t, 80)
	d.focused = crFocusOK

	// The text area already has "existing description" pre-populated.
	updated, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d2 := updated.(EditChangeRequestDialog)
	if d2.errMsg != "" {
		t.Errorf("submit in edit mode should not set errMsg, got %q", d2.errMsg)
	}
	if cmd == nil {
		t.Error("submit in edit mode should return a command")
	}
}

func TestEditChangeRequestDialog_Submit_EditMode_EmptyShowsError(t *testing.T) {
	ensureTheme(t)
	d := NewEditChangeRequestDialog(nil, testCRLocation(), &models.ChangeRequest{
		ID:          "42",
		Status:      models.ChangeRequestOpen,
		Description: "",
	}, 80)
	d.focused = crFocusOK

	updated, _ := d.Update(tea.KeyMsg{Type: tea.KeyEnter})
	d2 := updated.(EditChangeRequestDialog)
	if d2.errMsg == "" {
		t.Error("submitting empty description in edit mode should set errMsg")
	}
}

// ── Integration: app.go routes openEditChangeRequestDialogMsg with existingCR ─

func TestModel_Update_OpenEditChangeRequestDialogMsg_WithExistingCR(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	cr := &models.ChangeRequest{
		ID:          "99",
		Status:      models.ChangeRequestOpen,
		Description: "fix the bug",
	}

	updated, _ := m.Update(openEditChangeRequestDialogMsg{
		location: crLocation{
			identifier:   "proj/ticket",
			fileName:     "main.go",
			lineNum:      42,
			context:      "some code",
			worktreePath: "/tmp/wt",
		},
		existingCR: cr,
	})
	model := updated.(Model)
	if model.dialog == nil {
		t.Fatal("dialog should be set after openEditChangeRequestDialogMsg with existingCR")
	}
	d, ok := model.dialog.(EditChangeRequestDialog)
	if !ok {
		t.Fatalf("dialog is %T, want EditChangeRequestDialog", model.dialog)
	}
	if d.existingCR == nil {
		t.Error("dialog.existingCR should be non-nil")
	}
	if d.existingCR.ID != "99" {
		t.Errorf("dialog.existingCR.ID = %q, want %q", d.existingCR.ID, "99")
	}
}

// ── crSavedMsg notification tests ───────────────────────────────────────────

func TestModel_Update_CrSavedMsg_Created_ShowsCreatedNotification(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, cmd := m.Update(crSavedMsg{})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a notification command")
	}
	// Execute the command to get the notification message.
	msg := cmd()
	notif, ok := msg.(notifMsg)
	if !ok {
		t.Fatalf("expected notifMsg, got %T", msg)
	}
	if notif.text != "Change request created" {
		t.Errorf("expected notification %q, got %q", "Change request created", notif.text)
	}
	_ = model
}

func TestModel_Update_CrSavedMsg_Edited_ShowsUpdatedNotification(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, cmd := m.Update(crSavedMsg{edited: true})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a notification command")
	}
	msg := cmd()
	notif, ok := msg.(notifMsg)
	if !ok {
		t.Fatalf("expected notifMsg, got %T", msg)
	}
	if notif.text != "Change request updated" {
		t.Errorf("expected notification %q, got %q", "Change request updated", notif.text)
	}
	_ = model
}

func TestModel_Update_CrSavedMsg_Error_ShowsErrorNotification(t *testing.T) {
	ensureTheme(t)
	m := NewModel(nil, nil, 5)
	m.width = 120
	m.height = 40

	updated, cmd := m.Update(crSavedMsg{errMsg: "db error"})
	model := updated.(Model)
	if cmd == nil {
		t.Fatal("expected a notification command")
	}
	msg := cmd()
	notif, ok := msg.(notifMsg)
	if !ok {
		t.Fatalf("expected notifMsg, got %T", msg)
	}
	if notif.text != "CR failed: db error" {
		t.Errorf("expected notification %q, got %q", "CR failed: db error", notif.text)
	}
	_ = model
}
