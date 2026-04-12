package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
	"github.com/fimmtiu/code-factory/internal/git"
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
// DiffView owns the status bar rendering and passes only the content-pane
// dimensions into the viewer, so the viewer has no duplicate copies of
// identifier, phase, or full terminal size.
type DiffViewerModel struct {
	text       string // pre-rendered diff content
	fileStarts []int  // line offset where each file begins
	fileNames  []string
	offset     int // first visible line in the viewer pane

	// Collapse state: stored so we can re-render when the user toggles a file.
	files     []diff.File
	collapsed []bool

	// Content-pane dimensions (excluding status bar, separator, and chrome).
	// Set by DiffView on creation and resize via setSize.
	paneWidth  int
	paneHeight int
}

// newDiffViewerModel creates a DiffViewerModel from parsed diff files.
// paneWidth and paneHeight are the dimensions of the content area only
// (DiffView accounts for the status bar, separator, and chrome).
func newDiffViewerModel(files []diff.File, paneWidth, paneHeight int) *DiffViewerModel {
	m := &DiffViewerModel{
		paneWidth:  paneWidth,
		paneHeight: paneHeight,
		files:      files,
		collapsed:  make([]bool, len(files)),
	}

	if len(files) == 0 {
		return m
	}

	m.rerender()
	m.fileNames = fileNamesFromDiff(files)
	return m
}

// setSize updates the content-pane dimensions and re-clamps the scroll offset.
func (m *DiffViewerModel) setSize(paneWidth, paneHeight int) {
	m.paneWidth = paneWidth
	m.paneHeight = paneHeight
	m.clampScroll()
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
	maxOffset := total - m.paneHeight
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

// ── Collapse/expand ─────────────────────────────────────────────────────────

// toggleCollapse toggles the collapsed state of the current file.
// It is a no-op for files with no hunks to display.
func (m *DiffViewerModel) toggleCollapse() {
	idx := m.currentFileIndex()
	if idx < 0 || idx >= len(m.files) {
		return
	}
	if len(m.files[idx].Hunks) == 0 {
		return
	}
	m.collapsed[idx] = !m.collapsed[idx]
	m.rerender()
	// Scroll to the toggled file's header so the user sees the change.
	if idx < len(m.fileStarts) {
		m.offset = m.fileStarts[idx]
	}
	m.clampScroll()
}

// toggleCollapseAll collapses all files if any are expanded, or expands all
// files if all are already collapsed.
func (m *DiffViewerModel) toggleCollapseAll() {
	if len(m.files) == 0 {
		return
	}
	// Determine target state: collapse all unless every collapsible file is
	// already collapsed.
	allCollapsed := true
	for i, f := range m.files {
		if len(f.Hunks) > 0 && !m.collapsed[i] {
			allCollapsed = false
			break
		}
	}
	target := !allCollapsed
	for i, f := range m.files {
		if len(f.Hunks) > 0 {
			m.collapsed[i] = target
		}
	}
	m.rerender()
}

// rerender re-renders the diff text from the stored files and collapse state.
func (m *DiffViewerModel) rerender() {
	w := m.paneWidth - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	rd := renderDiffResult(m.files, w, m.collapsed)
	m.text = rd.text
	m.fileStarts = rd.fileStarts
	m.clampScroll()
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

// Update handles key events for the viewer screen. Window resize is handled
// by DiffView, which calls setSize directly.
func (m *DiffViewerModel) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return nil
}

// handleKey processes key events. Returns nil for all keys; the caller
// checks isViewerExitKey() to detect exit keys.
func (m *DiffViewerModel) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		m.scrollUp(1)
	case "down":
		m.scrollDown(1)
	case "pgup", "b":
		m.scrollUp(m.paneHeight)
	case "pgdown", " ":
		m.scrollDown(m.paneHeight)
	case "c":
		m.toggleCollapse()
	case "C":
		m.toggleCollapseAll()
	}
	return nil
}

// isViewerExitKey returns true if the key should close the viewer.
func isViewerExitKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "tab", "esc", "enter":
		return true
	}
	return false
}

// ── Rendering ────────────────────────────────────────────────────────────────

// renderViewerStatusBar renders the two-line status bar for the viewer screen.
// This is called by DiffView, which owns the identifier and phase fields.
func renderViewerStatusBar(width int, identifier, phase string, isProject bool, startHash, endHash string, viewer *DiffViewerModel) string {
	fileIdx := viewer.currentFileIndex()
	totalFiles := len(viewer.fileNames)

	// Line 1: "Ticket/Project: <id> (<phase>)" left, "File X of Y" right.
	left1 := renderDiffLabel(identifier, phase, isProject)
	right1 := ""
	if totalFiles > 0 {
		right1 = fmt.Sprintf("File %d of %d", fileIdx+1, totalFiles)
	}
	spacer := width - lipgloss.Width(left1) - lipgloss.Width(right1)
	if spacer < 2 {
		spacer = 2
	}
	line1 := left1 + strings.Repeat(" ", spacer) + right1

	// Line 2: current filename left, commit range right.
	var left2 string
	if totalFiles > 0 && fileIdx < totalFiles {
		left2 = viewer.fileNames[fileIdx]
	}
	right2 := shortCommitLabel(startHash, endHash)
	available := width - lipgloss.Width(right2) - 2
	if available < 0 {
		available = 0
	}
	left2 = leftTruncateFilename(left2, available)
	spacer = width - lipgloss.Width(left2) - lipgloss.Width(right2)
	if spacer < 2 {
		spacer = 2
	}
	line2 := left2 + strings.Repeat(" ", spacer) + right2

	return line1 + "\n" + line2
}

// shortCommitLabel returns "Commit <hash>" for a single commit or
// "Commits <start> - <end>" for a range, using 4-character short hashes.
func shortCommitLabel(startHash, endHash string) string {
	short := func(h string) string {
		if len(h) > 4 {
			return h[:4]
		}
		return h
	}
	if startHash == endHash {
		return "Commit " + short(endHash)
	}
	return "Commits " + short(startHash) + " to " + short(endHash)
}

// renderPane renders just the diff content pane (no status bar or separator).
func (m *DiffViewerModel) renderPane() string {
	paneW := m.paneWidth - viewBorderOverhead
	if paneW < 1 {
		paneW = 1
	}

	var content string
	if m.text == "" {
		content = lipgloss.Place(paneW, m.paneHeight, lipgloss.Center, lipgloss.Center,
			emptyStateStyle.Render("No diff content"))
	} else {
		lines := strings.Split(m.text, "\n")
		end := m.offset + m.paneHeight
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

	rendered := viewPaneStyle.Width(paneW).Height(m.paneHeight).Render(clipLines(content, m.paneHeight))
	return injectScrollbar(rendered, "│", "█", m.offset, m.totalLines(), m.paneHeight)
}

// KeyBindings returns key bindings shown when the viewer is active.
func (m *DiffViewerModel) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll"},
		{Key: "b/Space", Description: "Page up/down"},
		{Key: "c", Description: "Collapse/expand file"},
		{Key: "C", Description: "Collapse/expand all"},
		{Key: "Tab/Esc/Enter", Description: "Back to selector"},
	}
}

// ── Async diff fetch ─────────────────────────────────────────────────────────

// fetchDiffCmd runs git diff between two commits and parses the result.
func fetchDiffCmd(worktreePath string, startCommit, endCommit git.CommitEntry) tea.Cmd {
	return func() tea.Msg {
		raw, err := git.FetchDiff(worktreePath, startCommit, endCommit)
		if err != nil {
			return diffContentMsg{files: nil}
		}
		files := diff.Parse(raw)
		return diffContentMsg{files: files}
	}
}
