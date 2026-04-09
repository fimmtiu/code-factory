package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
)

// viewerStatusBarHeight is the number of lines consumed by the viewer's
// two-line status bar (ticket info + filename).
const viewerStatusBarHeight = 2

// ── Messages ─────────────────────────────────────────────────────────────────

// diffContentMsg carries the parsed diff files after an async fetch.
type diffContentMsg struct {
	files []diff.File
}

// ── DiffViewerModel ──────────────────────────────────────────────────────────

// DiffViewerModel is the sub-screen for displaying a scrollable diff.
// DiffView holds a *DiffViewerModel that is non-nil when the viewer is active.
type DiffViewerModel struct {
	text       string // pre-rendered diff content
	fileStarts []int  // line offset where each file begins
	fileNames  []string
	offset     int // first visible line in the viewer pane

	// Dimensions and context inherited from DiffView on creation.
	width      int
	height     int
	identifier string
	phase      string
}

// newDiffViewerModel creates a DiffViewerModel from parsed diff files.
func newDiffViewerModel(files []diff.File, width, height int, identifier, phase string) *DiffViewerModel {
	m := &DiffViewerModel{
		width:      width,
		height:     height,
		identifier: identifier,
		phase:      phase,
	}

	if len(files) == 0 {
		return m
	}

	w := width - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	rd := renderDiffResult(files, w)
	m.text = rd.text
	m.fileStarts = rd.fileStarts
	m.fileNames = fileNamesFromDiff(files)
	return m
}

// ── Dimension helpers ────────────────────────────────────────────────────────

// paneHeight returns the number of visible lines in the viewer pane.
func (m *DiffViewerModel) paneHeight() int {
	h := m.height - chromeHeight - viewerStatusBarHeight - separatorLineHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// totalLines returns the total number of lines in the rendered diff.
func (m *DiffViewerModel) totalLines() int {
	if m.text == "" {
		return 0
	}
	return len(strings.Split(m.text, "\n"))
}

// ── Scroll ───────────────────────────────────────────────────────────────────

// scrollDown scrolls the viewer down by n lines.
func (m *DiffViewerModel) scrollDown(n int) {
	m.offset += n
	m.clampScroll()
}

// scrollUp scrolls the viewer up by n lines.
func (m *DiffViewerModel) scrollUp(n int) {
	m.offset -= n
	m.clampScroll()
}

// clampScroll ensures the viewer offset stays in bounds.
func (m *DiffViewerModel) clampScroll() {
	total := m.totalLines()
	paneH := m.paneHeight()

	maxOffset := total - paneH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.offset > maxOffset {
		m.offset = maxOffset
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

// ── File tracking ────────────────────────────────────────────────────────────

// currentFileIndex returns the 0-based index of the file whose diff is
// currently at the top of the visible viewer area.
func (m *DiffViewerModel) currentFileIndex() int {
	if len(m.fileStarts) == 0 {
		return 0
	}
	idx := 0
	for i, start := range m.fileStarts {
		if start <= m.offset {
			idx = i
		} else {
			break
		}
	}
	return idx
}

// ── Left-truncation ──────────────────────────────────────────────────────────

// leftTruncateFilename truncates a filename from the left with an ellipsis
// if it exceeds maxWidth runes.
func leftTruncateFilename(name string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	runes := []rune(name)
	if len(runes) <= maxWidth {
		return name
	}
	if maxWidth == 1 {
		return "…"
	}
	// Keep the rightmost (maxWidth-1) runes plus ellipsis.
	return "…" + string(runes[len(runes)-(maxWidth-1):])
}

// ── Update ───────────────────────────────────────────────────────────────────

// Update handles key events for the viewer screen.
func (m *DiffViewerModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return nil
}

// handleKey processes key events. Returns nil for all keys; the caller
// checks shouldClose() to detect exit keys.
func (m *DiffViewerModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		m.scrollUp(1)
	case "down":
		m.scrollDown(1)
	case "pgup":
		m.scrollUp(m.paneHeight())
	case "pgdown":
		m.scrollDown(m.paneHeight())
	}
	return nil
}

// isExitKey returns true if the key should close the viewer.
func isViewerExitKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "tab", "esc", "enter":
		return true
	}
	return false
}

// ── Rendering ────────────────────────────────────────────────────────────────

// renderStatusBar renders the two-line status bar for the viewer screen.
func (m *DiffViewerModel) renderStatusBar() string {
	fileIdx := m.currentFileIndex()
	totalFiles := len(m.fileNames)

	// Line 1: "Ticket: <id> (<phase>)" left, "File X of Y" right.
	left := fmt.Sprintf("Ticket: %s (%s)", m.identifier, m.phase)
	right := ""
	if totalFiles > 0 {
		right = fmt.Sprintf("File %d of %d", fileIdx+1, totalFiles)
	}
	spacer := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacer < 2 {
		spacer = 2
	}
	line1 := left + strings.Repeat(" ", spacer) + right

	// Line 2: current filename (left-truncated if needed).
	var line2 string
	if totalFiles > 0 && fileIdx < totalFiles {
		line2 = leftTruncateFilename(m.fileNames[fileIdx], m.width)
	}

	return line1 + "\n" + line2
}

// View renders the complete viewer screen.
func (m *DiffViewerModel) View() string {
	statusBar := m.renderStatusBar()
	separator := strings.Repeat("─", m.width)

	paneH := m.paneHeight()
	paneW := m.width - viewBorderOverhead
	if paneW < 1 {
		paneW = 1
	}

	var content string
	if m.text == "" {
		content = lipgloss.Place(paneW, paneH, lipgloss.Center, lipgloss.Center,
			emptyStateStyle.Render("No diff content"))
	} else {
		lines := strings.Split(m.text, "\n")
		end := m.offset + paneH
		if end > len(lines) {
			end = len(lines)
		}
		start := m.offset
		if start > len(lines) {
			start = len(lines)
		}
		visible := lines[start:end]
		content = strings.Join(visible, "\n")
	}

	pane := viewPaneStyle.Width(paneW).Height(paneH).Render(clipLines(content, paneH))

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, separator, pane)
}

// KeyBindings returns key bindings shown when the viewer is active.
func (m *DiffViewerModel) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll"},
		{Key: "PgUp/PgDn", Description: "Page scroll"},
		{Key: "Tab/Esc/Enter", Description: "Back to selector"},
	}
}

// ── Async diff fetch ─────────────────────────────────────────────────────────

// fetchDiffCmd runs git diff between two commits and parses the result.
func fetchDiffCmd(worktreePath string, startCommit, endCommit commitEntry) tea.Cmd {
	return func() tea.Msg {
		raw, err := fetchDiff(worktreePath, startCommit, endCommit)
		if err != nil {
			return diffContentMsg{files: nil}
		}
		files := diff.Parse(raw)
		return diffContentMsg{files: files}
	}
}
