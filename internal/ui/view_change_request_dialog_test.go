package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

func sampleCR() models.ChangeRequest {
	return models.ChangeRequest{
		ID:           "42",
		CommitHash:   "abc123",
		CodeLocation: "main.go:10",
		Status:       models.ChangeRequestOpen,
		Date:         time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
		Author:       "alice",
		Description:  "Fix the widget rendering bug",
	}
}

// newTestViewCRDialog creates a ViewChangeRequestDialog for testing without
// requiring a database or filesystem. Code context and description wrapping
// are set directly rather than being fetched.
func newTestViewCRDialog(cr models.ChangeRequest) *ViewChangeRequestDialog {
	fileName, lineNum := parseCodeLocationForDisplay(cr.CodeLocation)
	contentWidth := 60
	var descLines []string
	for _, line := range strings.Split(cr.Description, "\n") {
		descLines = append(descLines, wrapLine(line, contentWidth)...)
	}
	return &ViewChangeRequestDialog{
		cr:            cr,
		identifier:    "test/ticket-1",
		worktreePath:  "/tmp/worktree",
		fileName:      fileName,
		lineNum:       lineNum,
		codeContext:   "> 10 | fmt.Println(\"hello\")",
		descLines:     descLines,
		descOffset:    0,
		width:         80,
		contentHeight: 8,
	}
}

// ── View tests ───────────────────────────────────────────────────────────────

func TestViewCRDialog_View_UsesThemeDialogBoxStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DialogBoxStyle = lipgloss.NewStyle().Border(lipgloss.NormalBorder())
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "┌") {
		t.Error("ViewChangeRequestDialog.View() did not use theme.Current().DialogBoxStyle")
	}
}

func TestViewCRDialog_View_ContainsTitle(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "Change Request") {
		t.Error("ViewChangeRequestDialog.View() missing 'Change Request' title")
	}
}

func TestViewCRDialog_View_ContainsFileName(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "File:") {
		t.Error("ViewChangeRequestDialog.View() missing 'File:' label")
	}
	if !strings.Contains(view, "main.go") {
		t.Error("ViewChangeRequestDialog.View() missing file name")
	}
}

func TestViewCRDialog_View_ContainsLineNumber(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "Line:") {
		t.Error("ViewChangeRequestDialog.View() missing 'Line:' label")
	}
	if !strings.Contains(view, "10") {
		t.Error("ViewChangeRequestDialog.View() missing line number")
	}
}

func TestViewCRDialog_View_ContainsStatus(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "Status:") {
		t.Error("ViewChangeRequestDialog.View() missing 'Status:' label")
	}
	if !strings.Contains(view, "open") {
		t.Error("ViewChangeRequestDialog.View() missing status value")
	}
}

func TestViewCRDialog_View_ContainsCodeContext(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "fmt.Println") {
		t.Error("ViewChangeRequestDialog.View() missing code context")
	}
}

func TestViewCRDialog_View_ContainsDescription(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "Description:") {
		t.Error("ViewChangeRequestDialog.View() missing 'Description:' label")
	}
	if !strings.Contains(view, "Fix the widget") {
		t.Error("ViewChangeRequestDialog.View() missing description text")
	}
}

func TestViewCRDialog_View_ContainsOKButton(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "OK") {
		t.Error("ViewChangeRequestDialog.View() missing 'OK' button")
	}
}

func TestViewCRDialog_View_UsesThemeDetailLabelStyle(t *testing.T) {
	saveTheme(t)
	theme.Current().DetailLabelStyle = lipgloss.NewStyle().Italic(true)
	d := newTestViewCRDialog(sampleCR())
	view := d.View()
	if !strings.Contains(view, "File:") {
		t.Error("ViewChangeRequestDialog.View() should render detail labels")
	}
}

func TestViewCRDialog_View_DismissedStatus(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Status = models.ChangeRequestDismissed
	d := newTestViewCRDialog(cr)
	view := d.View()
	if !strings.Contains(view, "dismissed") {
		t.Error("ViewChangeRequestDialog.View() should show 'dismissed' status")
	}
}

// ── Scrolling tests ──────────────────────────────────────────────────────────

func TestViewCRDialog_Update_DownScrollsDescription(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = strings.Repeat("line\n", 20)
	d := newTestViewCRDialog(cr)
	d.contentHeight = 5

	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 1 {
		t.Errorf("expected descOffset=1 after down key, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_UpScrollsClampsToZero(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	d.descOffset = 0

	msg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 0 {
		t.Errorf("expected descOffset=0 after up key at top, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_UpScrollsFromOffset(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = strings.Repeat("line\n", 20)
	d := newTestViewCRDialog(cr)
	d.contentHeight = 5
	d.descOffset = 3

	msg := tea.KeyMsg{Type: tea.KeyUp}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 2 {
		t.Errorf("expected descOffset=2 after up key from 3, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_PgDownScrollsByPage(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = strings.Repeat("line\n", 30)
	d := newTestViewCRDialog(cr)
	d.contentHeight = 5

	msg := tea.KeyMsg{Type: tea.KeyPgDown}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 5 {
		t.Errorf("expected descOffset=5 after pgdown, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_PgUpScrollsByPage(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = strings.Repeat("line\n", 30)
	d := newTestViewCRDialog(cr)
	d.contentHeight = 5
	d.descOffset = 10

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 5 {
		t.Errorf("expected descOffset=5 after pgup from 10, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_PgUpClampsToZero(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = strings.Repeat("line\n", 30)
	d := newTestViewCRDialog(cr)
	d.contentHeight = 5
	d.descOffset = 2

	msg := tea.KeyMsg{Type: tea.KeyPgUp}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 0 {
		t.Errorf("expected descOffset=0 after pgup from 2 with page 5, got %d", result.descOffset)
	}
}

func TestViewCRDialog_Update_DownClampsToMax(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	d.contentHeight = 50 // larger than descLines, so max offset is 0
	d.descOffset = 0

	msg := tea.KeyMsg{Type: tea.KeyDown}
	updated, _ := d.Update(msg)
	result := updated.(*ViewChangeRequestDialog)
	if result.descOffset != 0 {
		t.Errorf("expected descOffset=0 when content fits in view, got %d", result.descOffset)
	}
}

// ── Dismiss tests ────────────────────────────────────────────────────────────

func TestViewCRDialog_Update_EscDismisses(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())

	msg := tea.KeyMsg{Type: tea.KeyEscape}
	_, cmd := d.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command from esc key")
	}
	result := cmd()
	if _, ok := result.(dismissDialogMsg); !ok {
		t.Errorf("expected dismissDialogMsg from esc, got %T", result)
	}
}

func TestViewCRDialog_Update_EnterDismisses(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	_, cmd := d.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command from enter key")
	}
	result := cmd()
	if _, ok := result.(dismissDialogMsg); !ok {
		t.Errorf("expected dismissDialogMsg from enter, got %T", result)
	}
}

// ── Status toggle tests ─────────────────────────────────────────────────────

func TestViewCRDialog_Update_XTogglesSendsCmd(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	// CR is open, so x should send a command to dismiss it.
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	_, cmd := d.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command from x key when CR is open")
	}
}

func TestViewCRDialog_Update_XOnDismissedSendsReopenCmd(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Status = models.ChangeRequestDismissed
	d := newTestViewCRDialog(cr)

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	_, cmd := d.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command from x key when CR is dismissed")
	}
}

func TestViewCRDialog_Update_StatusToggledMsgUpdatesStatus(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())

	statusMsg := viewCRStatusToggledMsg{status: models.ChangeRequestDismissed}
	updated, _ := d.Update(statusMsg)
	result := updated.(*ViewChangeRequestDialog)
	if result.cr.Status != models.ChangeRequestDismissed {
		t.Errorf("expected status 'dismissed' after toggle msg, got %q", result.cr.Status)
	}
}

func TestViewCRDialog_Update_StatusToggledMsgReopens(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Status = models.ChangeRequestDismissed
	d := newTestViewCRDialog(cr)

	statusMsg := viewCRStatusToggledMsg{status: models.ChangeRequestOpen}
	updated, _ := d.Update(statusMsg)
	result := updated.(*ViewChangeRequestDialog)
	if result.cr.Status != models.ChangeRequestOpen {
		t.Errorf("expected status 'open' after toggle msg, got %q", result.cr.Status)
	}
}

func TestViewCRDialog_ToggleStatus_NilDatabaseReturnsNil(t *testing.T) {
	ensureTheme(t)
	d := newTestViewCRDialog(sampleCR())
	d.database = nil // explicitly nil

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	_, cmd := d.Update(msg)
	if cmd == nil {
		t.Fatal("expected a command from x key")
	}
	result := cmd()
	if result != nil {
		t.Errorf("expected nil from toggle when database is nil, got %T", result)
	}
}

// ── Init test ────────────────────────────────────────────────────────────────

func TestViewCRDialog_Init_ReturnsNil(t *testing.T) {
	d := newTestViewCRDialog(sampleCR())
	if cmd := d.Init(); cmd != nil {
		t.Error("Init() should return nil")
	}
}

// ── Description scrolling boundary tests ─────────────────────────────────────

func TestViewCRDialog_View_DescriptionScrolled(t *testing.T) {
	ensureTheme(t)
	cr := sampleCR()
	cr.Description = "line0\nline1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9"
	d := newTestViewCRDialog(cr)
	d.contentHeight = 3
	d.descOffset = 2

	view := d.View()
	// line2 should be visible, line0 and line1 should not
	if !strings.Contains(view, "line2") {
		t.Error("ViewChangeRequestDialog.View() should show line2 when scrolled to offset 2")
	}
}

// ── openViewChangeRequestDialogMsg tests ─────────────────────────────────────

func TestOpenViewChangeRequestDialogMsg_Fields(t *testing.T) {
	cr := sampleCR()
	msg := openViewChangeRequestDialogMsg{
		cr:           cr,
		identifier:   "test/ticket-1",
		worktreePath: "/tmp/worktree",
	}
	if msg.cr.ID != "42" {
		t.Error("openViewChangeRequestDialogMsg should carry the CR")
	}
	if msg.identifier != "test/ticket-1" {
		t.Error("openViewChangeRequestDialogMsg should carry the identifier")
	}
	if msg.worktreePath != "/tmp/worktree" {
		t.Error("openViewChangeRequestDialogMsg should carry the worktree path")
	}
}
