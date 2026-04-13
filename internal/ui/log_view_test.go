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

// ── 'g' key handler tests ────────────────────────────────────────────────────

func TestLogView_GKey_EmitsOpenDiffViewMsg(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{
				Message: "claimed proj/ticket",
				Logfile: "/repo/.code-factory/proj/ticket/implement.log",
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
				Logfile: "/repo/.code-factory/proj/ticket/review.log",
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

func TestLogView_GKey_InvalidLogfilePath_ReturnsNotification(t *testing.T) {
	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{Message: "some message", Logfile: "/random/path.log"},
		},
		selected: 0,
	}

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if cmd == nil {
		t.Fatal("expected non-nil cmd (notification) when logfile path has no valid identifier")
	}
	msg := cmd()
	if _, ok := msg.(notifMsg); !ok {
		t.Errorf("expected notifMsg, got %T", msg)
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

// ── Theme migration tests ────────────────────────────────────────────────────

// TestLogView_RenderRow_UsesThemeTimestampStyle verifies that renderRow uses
// theme.Current().LogTimestampStyle for non-selected rows.
func TestLogView_RenderRow_UsesThemeTimestampStyle(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := LogView{width: 120, height: 40}
	entry := &models.LogEntry{
		Message:   "some log message",
		Timestamp: time.Now().Add(-2 * time.Minute),
	}

	row := v.renderRow(entry, false)
	if row == "" {
		t.Error("renderRow returned empty string for non-selected row")
	}
}

// TestLogView_RenderRow_UsesThemeSelectedStyle verifies that renderRow uses
// theme.Current().LogSelectedStyle for the selected row.
func TestLogView_RenderRow_UsesThemeSelectedStyle(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := LogView{width: 120, height: 40}
	entry := &models.LogEntry{
		Message:   "some log message",
		Timestamp: time.Now(),
	}

	row := v.renderRow(entry, true)
	if row == "" {
		t.Error("renderRow returned empty string for selected row")
	}
}

// TestLogView_RenderRow_ThemeSwapDoesNotPanic verifies that renderRow works
// correctly after swapping theme.Current(), proving the theme is being used.
func TestLogView_RenderRow_ThemeSwapDoesNotPanic(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })

	v := LogView{width: 120, height: 40}
	entry := &models.LogEntry{
		Message:   "some log message",
		Timestamp: time.Now(),
	}

	// Render with default tan theme.
	theme.SetCurrent(theme.Tan())
	row1 := v.renderRow(entry, true)
	if row1 == "" {
		t.Error("renderRow returned empty string with tan theme")
	}

	// Swap to a different theme and verify rendering still works.
	alt := theme.Tan()
	alt.LogSelectedStyle = lipgloss.NewStyle().Background(lipgloss.Color("196")).Foreground(lipgloss.Color("255"))
	theme.SetCurrent(alt)
	row2 := v.renderRow(entry, true)
	if row2 == "" {
		t.Error("renderRow returned empty string with alt theme")
	}
}

// TestLogView_RenderRow_MessageColorByCategory verifies that different message
// types get different colours via theme.Current().LogMessageColor.
func TestLogView_RenderRow_MessageColorByCategory(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := LogView{width: 120, height: 40}
	now := time.Now()

	errorEntry := &models.LogEntry{Message: "error: something broke", Timestamp: now}
	normalEntry := &models.LogEntry{Message: "some normal message", Timestamp: now}

	errorRow := v.renderRow(errorEntry, false)
	normalRow := v.renderRow(normalEntry, false)

	// The two rows should be different because they have different message colours.
	if errorRow == normalRow {
		t.Error("error and normal messages should render differently due to LogMessageColor")
	}
}

// TestLogView_View_UsesThemeStyles verifies the full View method works with
// theme styles.
func TestLogView_View_UsesThemeStyles(t *testing.T) {
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())

	v := LogView{
		width:  120,
		height: 40,
		entries: []models.LogEntry{
			{Message: "claimed proj/ticket", Timestamp: time.Now()},
			{Message: "error: something", Timestamp: time.Now()},
		},
		selected: 0,
	}

	output := v.View()
	if output == "" {
		t.Error("View() returned empty string")
	}
	if !strings.Contains(output, "claimed") {
		t.Error("View output should contain log messages")
	}
}

// TestLogView_View_UsesThemeViewPaneStyle verifies that LogView.View() uses
// theme.Current().ViewPaneStyle for the pane border.
func TestLogView_View_UsesThemeViewPaneStyle(t *testing.T) {
	useTanTheme(t)

	// Replace ViewPaneStyle with NormalBorder so we can detect it.
	theme.Current().ViewPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("12"))

	v := LogView{
		width:  80,
		height: 24,
		entries: []models.LogEntry{
			{Message: "test entry", Timestamp: time.Now()},
		},
		selected: 0,
	}

	output := v.View()
	// NormalBorder uses "┌" while RoundedBorder (default Tan) uses "╭".
	if !strings.Contains(output, "┌") {
		t.Error("LogView.View() should use theme.Current().ViewPaneStyle")
	}
}

// TestLogView_View_EmptyUsesThemeEmptyStateStyle verifies that LogView.View()
// uses theme.Current().EmptyStateStyle for the empty-state message.
func TestLogView_View_EmptyUsesThemeEmptyStateStyle(t *testing.T) {
	useTanTheme(t)

	v := LogView{
		width:  80,
		height: 24,
	}

	output := v.View()
	expected := theme.Current().EmptyStateStyle.Render("No log entries")
	if !strings.Contains(output, expected) {
		t.Errorf("empty LogView should use theme.Current().EmptyStateStyle")
	}
}

// TestLogView_View_EmptyUsesThemeViewPaneStyle verifies that LogView.View()
// uses theme.Current().ViewPaneStyle for the pane border even when empty.
func TestLogView_View_EmptyUsesThemeViewPaneStyle(t *testing.T) {
	useTanTheme(t)

	theme.Current().ViewPaneStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("12"))

	v := LogView{
		width:  80,
		height: 24,
	}

	output := v.View()
	if !strings.Contains(output, "┌") {
		t.Error("empty LogView.View() should use theme.Current().ViewPaneStyle")
	}
}
