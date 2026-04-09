package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
)

// ── Helper ───────────────────────────────────────────────────────────────────

// makeDiffViewInViewerMode creates a DiffView that is already in viewer mode
// with rendered diff content ready for testing.
func makeDiffViewInViewerMode(files []diff.File, width, height int) DiffView {
	v := DiffView{
		width:      width,
		height:     height,
		identifier: "proj/ticket",
		phase:      "implement",
	}
	v.viewer = newDiffViewerModel(files, width, height, "proj/ticket", "implement")
	return v
}

// makeViewerModel creates a standalone DiffViewerModel for direct testing.
func makeViewerModel(files []diff.File, width, height int) *DiffViewerModel {
	return newDiffViewerModel(files, width, height, "proj/ticket", "implement")
}

// sampleFiles returns a small set of diff files for testing.
func sampleFiles() []diff.File {
	return []diff.File{
		{
			Name: "internal/ui/app.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func main()",
					NewStart: 10,
					NewCount: 3,
					Lines: []diff.Line{
						{Type: diff.LineContext, Content: "fmt.Println(\"hello\")"},
						{Type: diff.LineRemoved, Content: "fmt.Println(\"old\")"},
						{Type: diff.LineAdded, Content: "fmt.Println(\"new\")"},
					},
				},
			},
		},
		{
			Name: "internal/db/project_context_test.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{
					Context:  "func TestFoo()",
					NewStart: 1,
					NewCount: 2,
					Lines: []diff.Line{
						{Type: diff.LineAdded, Content: "package db"},
						{Type: diff.LineAdded, Content: ""},
					},
				},
			},
		},
	}
}

// largeSampleFiles returns diff files with enough lines to require scrolling
// even at height=24. Each file has many hunk lines so the total rendered
// output exceeds any reasonable pane height.
func largeSampleFiles() []diff.File {
	var lines []diff.Line
	for i := 0; i < 30; i++ {
		lines = append(lines, diff.Line{Type: diff.LineAdded, Content: "line content"})
	}
	return []diff.File{
		{
			Name: "first_file.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{Context: "func A()", NewStart: 1, NewCount: 30, Lines: lines},
			},
		},
		{
			Name: "second_file.go",
			Type: diff.Normal,
			Hunks: []diff.Hunk{
				{Context: "func B()", NewStart: 1, NewCount: 30, Lines: lines},
			},
		},
	}
}

// ── Viewer mode entry tests ──────────────────────────────────────────────────

// TestEnterViewerMode verifies that newDiffViewerModel sets viewer state correctly.
func TestEnterViewerMode(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 24)

	if m.text == "" {
		t.Error("expected text to be non-empty")
	}
	if len(m.fileStarts) != 2 {
		t.Errorf("expected 2 file starts, got %d", len(m.fileStarts))
	}
	if len(m.fileNames) != 2 {
		t.Errorf("expected 2 file names, got %d", len(m.fileNames))
	}
	if m.offset != 0 {
		t.Errorf("expected offset to be 0, got %d", m.offset)
	}
}

// TestEnterViewerMode_EmptyFiles handles empty diff.
func TestEnterViewerMode_EmptyFiles(t *testing.T) {
	m := makeViewerModel(nil, 80, 24)
	if m.text != "" {
		t.Errorf("expected empty text, got %q", m.text)
	}
	if len(m.fileStarts) != 0 {
		t.Errorf("expected 0 file starts, got %d", len(m.fileStarts))
	}
}

// ── Viewer status bar tests ──────────────────────────────────────────────────

// TestViewerStatusBar_Content verifies the two-line status bar content.
func TestViewerStatusBar_Content(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 24)

	bar := m.renderStatusBar()
	lines := strings.Split(bar, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines in status bar, got %d", len(lines))
	}

	// First line: ticket info and file index.
	if !strings.Contains(lines[0], "Ticket: proj/ticket (implement)") {
		t.Errorf("first line should contain ticket info, got %q", lines[0])
	}
	if !strings.Contains(lines[0], "File 1 of 2") {
		t.Errorf("first line should contain file index, got %q", lines[0])
	}

	// Second line: current filename.
	if !strings.Contains(lines[1], "internal/ui/app.go") {
		t.Errorf("second line should contain filename, got %q", lines[1])
	}
}

// TestViewerStatusBar_FileIndexUpdates verifies that scrolling past a file
// boundary updates the file index.
func TestViewerStatusBar_FileIndexUpdates(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 40)

	// Scroll past the first file.
	if len(m.fileStarts) < 2 {
		t.Fatal("need at least 2 files for this test")
	}
	m.offset = m.fileStarts[1]

	bar := m.renderStatusBar()
	if !strings.Contains(bar, "File 2 of 2") {
		t.Errorf("expected 'File 2 of 2' after scroll, got %q", bar)
	}
	if !strings.Contains(bar, "project_context_test.go") {
		t.Errorf("expected second filename in status bar, got %q", bar)
	}
}

// ── Current file index tests ─────────────────────────────────────────────────

// TestCurrentFileIndex_AtStart verifies file index is 0 at the beginning.
func TestCurrentFileIndex_AtStart(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 24)
	m.offset = 0

	idx := m.currentFileIndex()
	if idx != 0 {
		t.Errorf("expected file index 0, got %d", idx)
	}
}

// TestCurrentFileIndex_BetweenFiles verifies file index between file boundaries.
func TestCurrentFileIndex_BetweenFiles(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 24)

	if len(m.fileStarts) < 2 {
		t.Fatal("need at least 2 files")
	}

	// Just before the second file starts.
	m.offset = m.fileStarts[1] - 1
	idx := m.currentFileIndex()
	if idx != 0 {
		t.Errorf("expected file index 0 (before second file), got %d", idx)
	}

	// At the second file start.
	m.offset = m.fileStarts[1]
	idx = m.currentFileIndex()
	if idx != 1 {
		t.Errorf("expected file index 1 (at second file), got %d", idx)
	}
}

// TestCurrentFileIndex_EmptyFiles returns 0 for empty diff.
func TestCurrentFileIndex_EmptyFiles(t *testing.T) {
	m := makeViewerModel(nil, 80, 24)
	idx := m.currentFileIndex()
	if idx != 0 {
		t.Errorf("expected file index 0 for empty, got %d", idx)
	}
}

// ── Left-truncation tests ────────────────────────────────────────────────────

// TestLeftTruncateFilename_Short verifies short names are not truncated.
func TestLeftTruncateFilename_Short(t *testing.T) {
	result := leftTruncateFilename("main.go", 40)
	if result != "main.go" {
		t.Errorf("expected 'main.go', got %q", result)
	}
}

// TestLeftTruncateFilename_Long verifies long names are left-truncated with ellipsis.
func TestLeftTruncateFilename_Long(t *testing.T) {
	long := "internal/db/project_context_test.go"
	result := leftTruncateFilename(long, 20)
	if len([]rune(result)) > 20 {
		t.Errorf("result too long: %q (%d runes)", result, len([]rune(result)))
	}
	if !strings.HasPrefix(result, "…") {
		t.Errorf("expected left-truncated with ellipsis, got %q", result)
	}
	if !strings.HasSuffix(result, "test.go") {
		t.Errorf("expected to end with 'test.go', got %q", result)
	}
}

// TestLeftTruncateFilename_ExactFit handles names that exactly fit.
func TestLeftTruncateFilename_ExactFit(t *testing.T) {
	name := "abcde"
	result := leftTruncateFilename(name, 5)
	if result != "abcde" {
		t.Errorf("expected 'abcde', got %q", result)
	}
}

// TestLeftTruncateFilename_VerySmallWidth handles very small widths.
func TestLeftTruncateFilename_VerySmallWidth(t *testing.T) {
	result := leftTruncateFilename("internal/long/path.go", 2)
	if len([]rune(result)) > 2 {
		t.Errorf("result too long: %q", result)
	}
}

// TestLeftTruncateFilename_WidthOne returns the full ellipsis character.
func TestLeftTruncateFilename_WidthOne(t *testing.T) {
	result := leftTruncateFilename("internal/long/path.go", 1)
	if result != "…" {
		t.Errorf("expected full ellipsis for maxWidth=1, got %q", result)
	}
	// Verify it is valid UTF-8.
	for i, r := range result {
		if r == '\uFFFD' && i == 0 {
			t.Error("result contains replacement character, indicating invalid UTF-8")
		}
	}
}

// TestLeftTruncateFilename_WidthZero returns empty string.
func TestLeftTruncateFilename_WidthZero(t *testing.T) {
	result := leftTruncateFilename("internal/long/path.go", 0)
	if result != "" {
		t.Errorf("expected empty string for maxWidth=0, got %q", result)
	}
}

// TestLeftTruncateFilename_NegativeWidth returns empty string.
func TestLeftTruncateFilename_NegativeWidth(t *testing.T) {
	result := leftTruncateFilename("internal/long/path.go", -1)
	if result != "" {
		t.Errorf("expected empty string for negative maxWidth, got %q", result)
	}
}

// ── Scroll navigation tests ─────────────────────────────────────────────────

// TestViewerScrollDown verifies scrolling down by 1.
func TestViewerScrollDown(t *testing.T) {
	files := largeSampleFiles()
	m := makeViewerModel(files, 80, 24)
	m.offset = 0

	m.scrollDown(1)
	if m.offset != 1 {
		t.Errorf("expected offset 1 after scrollDown(1), got %d", m.offset)
	}
}

// TestViewerScrollUp verifies scrolling up from a non-zero offset.
func TestViewerScrollUp(t *testing.T) {
	files := largeSampleFiles()
	m := makeViewerModel(files, 80, 24)
	m.offset = 5

	m.scrollUp(2)
	if m.offset != 3 {
		t.Errorf("expected offset 3 after scrollUp(2), got %d", m.offset)
	}
}

// TestViewerScrollUp_ClampsAtZero verifies scroll doesn't go negative.
func TestViewerScrollUp_ClampsAtZero(t *testing.T) {
	files := largeSampleFiles()
	m := makeViewerModel(files, 80, 24)
	m.offset = 2

	m.scrollUp(10)
	if m.offset != 0 {
		t.Errorf("expected offset 0 after clamping, got %d", m.offset)
	}
}

// TestViewerScrollDown_ClampsAtMax verifies scroll doesn't go past the end.
func TestViewerScrollDown_ClampsAtMax(t *testing.T) {
	files := largeSampleFiles()
	m := makeViewerModel(files, 80, 24)

	totalLines := len(strings.Split(m.text, "\n"))
	m.scrollDown(totalLines + 100)

	maxOffset := totalLines - m.paneHeight()
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset != maxOffset {
		t.Errorf("expected offset %d after clamping, got %d", maxOffset, m.offset)
	}
}

// ── Key handling tests (through DiffView.Update) ─────────────────────────────

// TestViewerKeyUp scrolls up.
func TestViewerKeyUp(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	v.viewer.offset = 3

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyUp})
	dv := updated.(DiffView)
	if dv.viewer.offset != 2 {
		t.Errorf("expected offset 2 after up key, got %d", dv.viewer.offset)
	}
}

// TestViewerKeyDown scrolls down.
func TestViewerKeyDown(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	v.viewer.offset = 0

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyDown})
	dv := updated.(DiffView)
	if dv.viewer.offset != 1 {
		t.Errorf("expected offset 1 after down key, got %d", dv.viewer.offset)
	}
}

// TestViewerKeyPgDown scrolls down by page.
func TestViewerKeyPgDown(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	v.viewer.offset = 0

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	dv := updated.(DiffView)
	if dv.viewer.offset != dv.viewer.paneHeight() {
		t.Errorf("expected offset %d after pgdown, got %d", dv.viewer.paneHeight(), dv.viewer.offset)
	}
}

// TestViewerKeyPgUp scrolls up by page.
func TestViewerKeyPgUp(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	paneH := v.viewer.paneHeight()
	v.viewer.offset = paneH + 5

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	dv := updated.(DiffView)
	if dv.viewer.offset != 5 {
		t.Errorf("expected offset 5 after pgup, got %d", dv.viewer.offset)
	}
}

// TestViewerKeyEscape returns to commit selector.
func TestViewerKeyEscape(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyEscape})
	dv := updated.(DiffView)
	if dv.viewer != nil {
		t.Error("expected viewer to be nil after Escape")
	}
}

// TestViewerKeyTab returns to commit selector.
func TestViewerKeyTab(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyTab})
	dv := updated.(DiffView)
	if dv.viewer != nil {
		t.Error("expected viewer to be nil after Tab")
	}
}

// TestViewerKeyEnter returns to commit selector.
func TestViewerKeyEnter(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	dv := updated.(DiffView)
	if dv.viewer != nil {
		t.Error("expected viewer to be nil after Enter")
	}
}

// TestViewerIgnoresUnhandledKeys verifies that random keys don't cause issues.
func TestViewerIgnoresUnhandledKeys(t *testing.T) {
	files := largeSampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	v.viewer.offset = 2

	updated, _ := v.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	dv := updated.(DiffView)
	if dv.viewer.offset != 2 {
		t.Errorf("unhandled key should not change offset, got %d", dv.viewer.offset)
	}
	if dv.viewer == nil {
		t.Error("unhandled key should not exit viewer mode")
	}
}

// ── Viewer pane height tests ─────────────────────────────────────────────────

// TestViewerPaneHeight verifies the height calculation.
func TestViewerPaneHeight(t *testing.T) {
	m := makeViewerModel(nil, 80, 24)
	// height - chromeHeight - viewerStatusBarHeight(2) - separatorLineHeight(1) - viewBorderOverhead(2)
	expected := 24 - chromeHeight - viewerStatusBarHeight - separatorLineHeight - viewBorderOverhead
	got := m.paneHeight()
	if got != expected {
		t.Errorf("paneHeight: got %d, want %d", got, expected)
	}
}

// TestViewerPaneHeight_Small verifies minimum height is 1.
func TestViewerPaneHeight_Small(t *testing.T) {
	m := makeViewerModel(nil, 80, 5)
	h := m.paneHeight()
	if h < 1 {
		t.Errorf("paneHeight should be at least 1, got %d", h)
	}
}

// ── View rendering tests ────────────────────────────────────────────────────

// TestViewerView_ContainsStatusBar verifies the viewer View() includes status bar elements.
func TestViewerView_ContainsStatusBar(t *testing.T) {
	files := sampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)

	output := v.View()
	if !strings.Contains(output, "Ticket: proj/ticket") {
		t.Error("expected ticket info in viewer output")
	}
	if !strings.Contains(output, "File 1 of 2") {
		t.Error("expected file index in viewer output")
	}
}

// TestViewerView_ContainsSeparator verifies the horizontal separator is present.
func TestViewerView_ContainsSeparator(t *testing.T) {
	files := sampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)

	output := v.View()
	if !strings.Contains(output, "─") {
		t.Error("expected horizontal separator in viewer output")
	}
}

// TestViewerView_EmptyDiff shows a placeholder for empty diff.
func TestViewerView_EmptyDiff(t *testing.T) {
	v := makeDiffViewInViewerMode(nil, 80, 24)
	output := v.View()
	if !strings.Contains(output, "No diff content") {
		t.Error("expected 'No diff content' for empty diff")
	}
}

// ── KeyBindings test ─────────────────────────────────────────────────────────

// TestViewerKeyBindings verifies key bindings are returned in viewer mode.
func TestViewerKeyBindings(t *testing.T) {
	v := makeDiffViewInViewerMode(sampleFiles(), 80, 24)
	bindings := v.KeyBindings()
	if len(bindings) == 0 {
		t.Error("expected non-empty key bindings")
	}
	// Should mention scroll keys.
	found := false
	for _, b := range bindings {
		if strings.Contains(b.Description, "croll") || strings.Contains(b.Key, "↑") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected scroll-related key binding")
	}
}

// ── switchToDiffViewerMsg handling ───────────────────────────────────────────

// TestDiffViewReceivesDiffContent verifies that receiving a diffContentMsg
// switches to viewer mode.
func TestDiffViewReceivesDiffContent(t *testing.T) {
	v := DiffView{
		width:      80,
		height:     24,
		identifier: "proj/ticket",
		phase:      "implement",
	}

	files := sampleFiles()
	updated, _ := v.Update(diffContentMsg{files: files})
	dv := updated.(DiffView)
	if dv.viewer == nil {
		t.Error("expected viewer to be non-nil after diffContentMsg")
	}
	if dv.viewer.text == "" {
		t.Error("expected non-empty viewer text after diffContentMsg")
	}
}

// ── Window resize in viewer mode ─────────────────────────────────────────────

// TestViewerWindowResize verifies viewer recalculates on resize.
func TestViewerWindowResize(t *testing.T) {
	files := sampleFiles()
	v := makeDiffViewInViewerMode(files, 80, 24)
	v.viewer.offset = 5

	updated, _ := v.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	dv := updated.(DiffView)
	if dv.width != 120 || dv.height != 40 {
		t.Errorf("expected 120x40, got %dx%d", dv.width, dv.height)
	}
	if dv.viewer.width != 120 || dv.viewer.height != 40 {
		t.Errorf("expected viewer 120x40, got %dx%d", dv.viewer.width, dv.viewer.height)
	}
}

// ── Viewer View() width consistency ──────────────────────────────────────────

// TestViewerStatusBarWidth checks that the status bar lines use the full width.
func TestViewerStatusBarWidth(t *testing.T) {
	files := sampleFiles()
	m := makeViewerModel(files, 80, 24)

	bar := m.renderStatusBar()
	lines := strings.Split(bar, "\n")

	// First line should span approximately the full width.
	firstLineWidth := lipgloss.Width(lines[0])
	if firstLineWidth < 60 || firstLineWidth > 80 {
		t.Errorf("first status bar line width %d, expected ~80", firstLineWidth)
	}
}
