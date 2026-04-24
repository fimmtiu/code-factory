package ui

import (
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ansiEscapeRe matches ANSI escape sequences (CSI and OSC).
var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x1b\\|\x1b\][^\x07]*\x07`)

// stripAnsi removes ANSI escape sequences from a string.
func stripAnsi(s string) string {
	return ansiEscapeRe.ReplaceAllString(s, "")
}

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
	lineMeta   []diffLineMeta // per-line selectability and file ownership
	offset     int            // first visible line in the viewer pane

	// Collapse state: stored so we can re-render when the user toggles a file.
	files     []diff.File
	collapsed []bool

	// Content-pane dimensions (excluding status bar, separator, and chrome).
	// Set by DiffView on creation and resize via setSize.
	paneWidth  int
	paneHeight int

	// Line select mode state.
	lineSelectMode bool // true when the user is selecting individual lines
	selectedLine   int  // index of the currently selected line in the rendered text
	frozenFileIdx  int  // file index frozen on exit from line select; -1 when not frozen
}

// newDiffViewerModel creates a DiffViewerModel from parsed diff files.
// paneWidth and paneHeight are the dimensions of the content area only
// (DiffView accounts for the status bar, separator, and chrome).
func newDiffViewerModel(files []diff.File, paneWidth, paneHeight int) *DiffViewerModel {
	m := &DiffViewerModel{
		paneWidth:     paneWidth,
		paneHeight:    paneHeight,
		files:         files,
		collapsed:     make([]bool, len(files)),
		frozenFileIdx: -1,
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
// currently displayed. In line-select mode, this is the file owning the
// selected line. After exiting line-select mode, the file index is frozen
// until the user scrolls. Otherwise it is the file at the top of the pane.
func (m *DiffViewerModel) currentFileIndex() int {
	if m.lineSelectMode && m.selectedLine >= 0 && m.selectedLine < len(m.lineMeta) {
		return m.lineMeta[m.selectedLine].fileIndex
	}
	if m.frozenFileIdx >= 0 {
		return m.frozenFileIdx
	}
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
	wasLineSelect := m.lineSelectMode
	m.lineSelectMode = false
	m.rerender()
	// Scroll to the toggled file's header so the user sees the change.
	if idx < len(m.fileStarts) {
		m.offset = m.fileStarts[idx]
	}
	m.clampScroll()
	if wasLineSelect {
		m.enterLineSelect()
	}
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
	wasLineSelect := m.lineSelectMode
	m.lineSelectMode = false
	m.rerender()
	if wasLineSelect {
		m.enterLineSelect()
	}
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
	m.lineMeta = rd.lineMeta
	m.clampScroll()
}

// ── Line select mode ────────────────────────────────────────────────────────

// isSelectable returns true if line at index i is a hunk content line.
func (m *DiffViewerModel) isSelectable(i int) bool {
	return i >= 0 && i < len(m.lineMeta) && m.lineMeta[i].kind == diffLineHunkContent
}

// nearestSelectable searches outward from start in both directions and returns
// the nearest selectable line, or -1 if none exists within [lo, hi).
func (m *DiffViewerModel) nearestSelectable(start, lo, hi int) int {
	if hi > len(m.lineMeta) {
		hi = len(m.lineMeta)
	}
	if lo < 0 {
		lo = 0
	}
	for d := 0; d < hi-lo; d++ {
		up := start - d
		down := start + d
		if up >= lo && up < hi && m.isSelectable(up) {
			return up
		}
		if down >= lo && down < hi && m.isSelectable(down) {
			return down
		}
	}
	return -1
}

// enterLineSelect enters line-select mode. The selection is placed on the
// selectable line nearest to the vertical midpoint of the visible pane.
// If no selectable line is visible, the mode is not entered.
func (m *DiffViewerModel) enterLineSelect() {
	mid := m.offset + m.paneHeight/2
	sel := m.nearestSelectable(mid, m.offset, m.offset+m.paneHeight)
	if sel == -1 {
		return
	}
	m.lineSelectMode = true
	m.selectedLine = sel
	m.frozenFileIdx = -1
}

// exitLineSelect leaves line-select mode and freezes the current file index
// so it persists until the user scrolls.
func (m *DiffViewerModel) exitLineSelect() {
	if !m.lineSelectMode {
		return
	}
	m.frozenFileIdx = m.currentFileIndex()
	m.lineSelectMode = false
}

// nextSelectableLine returns the next selectable line from the current
// selectedLine in the given direction (+1 for down, -1 for up), or -1
// if there is no selectable line in that direction.
func (m *DiffViewerModel) nextSelectableLine(direction int) int {
	i := m.selectedLine + direction
	for i >= 0 && i < len(m.lineMeta) {
		if m.isSelectable(i) {
			return i
		}
		i += direction
	}
	return -1
}

// moveSelection moves the selected line by n steps in the given direction,
// skipping non-selectable lines and scrolling to keep the selection visible.
func (m *DiffViewerModel) moveSelection(n, direction int) {
	for i := 0; i < n; i++ {
		next := m.nextSelectableLine(direction)
		if next == -1 {
			break
		}
		m.selectedLine = next
	}
	m.scrollToSelection()
}

// scrollToSelection adjusts the scroll offset so the selected line is visible.
func (m *DiffViewerModel) scrollToSelection() {
	if m.selectedLine < m.offset {
		m.offset = m.selectedLine
	}
	if m.selectedLine >= m.offset+m.paneHeight {
		m.offset = m.selectedLine - m.paneHeight + 1
	}
	m.clampScroll()
}

// ── Selected line info ───────────────────────────────────────────────────────

// selectedLineInfo returns the file name, line number, and a few lines of code
// context around the currently selected line. Returns empty values if not in
// line select mode or if no line is selected.
func (m *DiffViewerModel) selectedLineInfo() (fileName string, lineNum int, context string) {
	if !m.lineSelectMode || m.selectedLine < 0 || m.selectedLine >= len(m.lineMeta) {
		return "", 0, ""
	}
	meta := m.lineMeta[m.selectedLine]
	if meta.fileIndex >= 0 && meta.fileIndex < len(m.fileNames) {
		fileName = m.fileNames[meta.fileIndex]
	}
	lineNum = meta.lineNum

	// Extract a few lines of context around the selected line.
	lines := strings.Split(m.text, "\n")
	start := m.selectedLine - 2
	if start < 0 {
		start = 0
	}
	end := m.selectedLine + 3
	if end > len(lines) {
		end = len(lines)
	}
	var ctxLines []string
	for i := start; i < end; i++ {
		ctxLines = append(ctxLines, stripAnsi(lines[i]))
	}
	context = strings.Join(ctxLines, "\n")
	return
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
	if m.lineSelectMode {
		return m.handleLineSelectKey(msg)
	}

	switch msg.String() {
	case "up":
		m.clearFrozenFileIdx()
		m.scrollUp(1)
	case "down":
		m.clearFrozenFileIdx()
		m.scrollDown(1)
	case "pgup", "b":
		m.clearFrozenFileIdx()
		m.scrollUp(m.paneHeight)
	case "pgdown", " ":
		m.clearFrozenFileIdx()
		m.scrollDown(m.paneHeight)
	case "<":
		m.clearFrozenFileIdx()
		m.offset = 0
	case ">":
		m.clearFrozenFileIdx()
		m.offset = m.totalLines()
		m.clampScroll()
	case "enter":
		m.enterLineSelect()
	case "c":
		m.toggleCollapse()
	case "C":
		m.toggleCollapseAll()
	}
	return nil
}

// handleLineSelectKey processes key events while in line-select mode.
func (m *DiffViewerModel) handleLineSelectKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "up":
		m.moveSelection(1, -1)
	case "down":
		m.moveSelection(1, 1)
	case "pgup", "b":
		m.moveSelection(m.paneHeight, -1)
	case "pgdown", " ":
		m.moveSelection(m.paneHeight, 1)
	case "<":
		m.moveSelection(len(m.lineMeta), -1)
	case ">":
		m.moveSelection(len(m.lineMeta), 1)
	case "esc":
		m.exitLineSelect()
	case "c":
		m.toggleCollapse()
	case "C":
		m.toggleCollapseAll()
	}
	return nil
}

// clearFrozenFileIdx removes the frozen file index so that scrolling
// resumes normal file tracking.
func (m *DiffViewerModel) clearFrozenFileIdx() {
	m.frozenFileIdx = -1
}

// isViewerExitKey returns true if the key should close the viewer.
// In line-select mode, only Tab exits the viewer; Escape exits line-select
// and Enter is consumed by line-select entry, so neither closes the viewer.
func isViewerExitKey(viewer *DiffViewerModel, msg tea.KeyMsg) bool {
	if viewer.lineSelectMode {
		return msg.String() == "tab"
	}
	switch msg.String() {
	case "tab", "esc":
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
			theme.Current().EmptyStateStyle.Render("No diff content"))
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

		// Highlight the selected line in line-select mode.
		if m.lineSelectMode && m.selectedLine >= start && m.selectedLine < end {
			idx := m.selectedLine - start
			visible[idx] = theme.Current().DiffLineSelectStyle.Width(paneW).Render(
				truncateLine(stripAnsi(visible[idx]), paneW))
		}

		content = strings.Join(visible, "\n")
	}

	rendered := theme.Current().ViewPaneStyle.Width(paneW).Height(m.paneHeight).Render(clipLines(content, m.paneHeight))
	return injectScrollbar(rendered, "│", "█", m.offset, m.totalLines(), m.paneHeight)
}

// KeyBindings returns key bindings shown when the viewer is active.
func (m *DiffViewerModel) KeyBindings() []KeyBinding {
	if m.lineSelectMode {
		return []KeyBinding{
			{Key: "↑/↓", Description: "Move selection"},
			{Key: "b/Space", Description: "Page up/down"},
			{Key: "</>", Description: "Jump to top/bottom"},
			{Key: "R", Description: "Create change request"},
			{Key: "c", Description: "Collapse/expand file"},
			{Key: "C", Description: "Collapse/expand all"},
			{Key: "T", Description: "Open terminal in worktree"},
			{Key: "E", Description: "Open worktree in Cursor"},
			{Key: "Esc", Description: "Exit line select"},
			{Key: "Tab", Description: "Back to selector"},
		}
	}
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll"},
		{Key: "b/Space", Description: "Page up/down"},
		{Key: "</>", Description: "Jump to top/bottom"},
		{Key: "Enter", Description: "Select lines"},
		{Key: "c", Description: "Collapse/expand file"},
		{Key: "C", Description: "Collapse/expand all"},
		{Key: "T", Description: "Open terminal in worktree"},
		{Key: "E", Description: "Open worktree in Cursor"},
		{Key: "Tab/Esc", Description: "Back to selector"},
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
