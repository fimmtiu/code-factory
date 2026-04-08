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

// ── Viewer state (added to DiffView) ─────────────────────────────────────────

// enterViewerMode initialises the viewer state on DiffView. It pre-renders
// the diff and computes file start offsets.
func (v *DiffView) enterViewerMode(files []diff.File, paneWidth int) {
	v.viewerActive = true
	v.viewerOffset = 0

	if len(files) == 0 {
		v.viewerText = ""
		v.viewerFileStarts = nil
		v.viewerFileNames = nil
		return
	}

	w := paneWidth - viewBorderOverhead
	if w < 1 {
		w = 1
	}
	rd := renderDiffResult(files, w)
	v.viewerText = rd.text
	v.viewerFileStarts = rd.fileStarts
	v.viewerFileNames = fileNamesFromDiff(files)
}

// exitViewerMode returns to the commit selector screen.
func (v *DiffView) exitViewerMode() {
	v.viewerActive = false
	v.viewerText = ""
	v.viewerFileStarts = nil
	v.viewerFileNames = nil
	v.viewerOffset = 0
}

// ── Dimension helpers ────────────────────────────────────────────────────────

// viewerPaneHeight returns the number of visible lines in the viewer pane.
func (v DiffView) viewerPaneHeight() int {
	h := v.height - chromeHeight - viewerStatusBarHeight - separatorLineHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// viewerTotalLines returns the total number of lines in the rendered diff.
func (v DiffView) viewerTotalLines() int {
	if v.viewerText == "" {
		return 0
	}
	return len(strings.Split(v.viewerText, "\n"))
}

// ── Scroll ───────────────────────────────────────────────────────────────────

// viewerScrollDown scrolls the viewer down by n lines.
func (v *DiffView) viewerScrollDown(n int) {
	v.viewerOffset += n
	v.viewerClampScroll()
}

// viewerScrollUp scrolls the viewer up by n lines.
func (v *DiffView) viewerScrollUp(n int) {
	v.viewerOffset -= n
	v.viewerClampScroll()
}

// viewerClampScroll ensures the viewer offset stays in bounds.
func (v *DiffView) viewerClampScroll() {
	total := v.viewerTotalLines()
	paneH := v.viewerPaneHeight()

	maxOffset := total - paneH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.viewerOffset > maxOffset {
		v.viewerOffset = maxOffset
	}
	if v.viewerOffset < 0 {
		v.viewerOffset = 0
	}
}

// ── File tracking ────────────────────────────────────────────────────────────

// currentFileIndex returns the 0-based index of the file whose diff is
// currently at the top of the visible viewer area.
func (v DiffView) currentFileIndex() int {
	if len(v.viewerFileStarts) == 0 {
		return 0
	}
	idx := 0
	for i, start := range v.viewerFileStarts {
		if start <= v.viewerOffset {
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
	runes := []rune(name)
	if len(runes) <= maxWidth {
		return name
	}
	if maxWidth <= 1 {
		return "…"[:maxWidth]
	}
	// Keep the rightmost (maxWidth-1) runes plus ellipsis.
	return "…" + string(runes[len(runes)-(maxWidth-1):])
}

// ── Key handling (viewer mode) ───────────────────────────────────────────────

// handleViewerKey handles key events when the viewer screen is active.
func (v DiffView) handleViewerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		v.viewerScrollUp(1)
	case "down":
		v.viewerScrollDown(1)
	case "pgup":
		v.viewerScrollUp(v.viewerPaneHeight())
	case "pgdown":
		v.viewerScrollDown(v.viewerPaneHeight())
	case "tab", "esc", "enter":
		v.exitViewerMode()
	default:
		return v, nil
	}
	return v, nil
}

// ── Rendering (viewer mode) ──────────────────────────────────────────────────

// renderViewerStatusBar renders the two-line status bar for the viewer screen.
func (v DiffView) renderViewerStatusBar() string {
	fileIdx := v.currentFileIndex()
	totalFiles := len(v.viewerFileNames)

	// Line 1: "Ticket: <id> (<phase>)" left, "File X of Y" right.
	left := fmt.Sprintf("Ticket: %s (%s)", v.identifier, v.phase)
	right := ""
	if totalFiles > 0 {
		right = fmt.Sprintf("File %d of %d", fileIdx+1, totalFiles)
	}
	spacer := v.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spacer < 2 {
		spacer = 2
	}
	line1 := left + strings.Repeat(" ", spacer) + right

	// Line 2: current filename (left-truncated if needed).
	var line2 string
	if totalFiles > 0 && fileIdx < totalFiles {
		line2 = leftTruncateFilename(v.viewerFileNames[fileIdx], v.width)
	}

	return line1 + "\n" + line2
}

// viewerView renders the complete viewer screen.
func (v DiffView) viewerView() string {
	statusBar := v.renderViewerStatusBar()
	separator := strings.Repeat("─", v.width)

	paneH := v.viewerPaneHeight()
	paneW := v.width - viewBorderOverhead
	if paneW < 1 {
		paneW = 1
	}

	var content string
	if v.viewerText == "" {
		content = lipgloss.Place(paneW, paneH, lipgloss.Center, lipgloss.Center,
			emptyStateStyle.Render("No diff content"))
	} else {
		lines := strings.Split(v.viewerText, "\n")
		end := v.viewerOffset + paneH
		if end > len(lines) {
			end = len(lines)
		}
		start := v.viewerOffset
		if start > len(lines) {
			start = len(lines)
		}
		visible := lines[start:end]
		content = strings.Join(visible, "\n")
	}

	pane := viewPaneStyle.Width(paneW).Height(paneH).Render(clipLines(content, paneH))

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, separator, pane)
}

// viewerKeyBindings returns key bindings shown when the viewer is active.
func viewerKeyBindings() []KeyBinding {
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
		var raw string
		var err error

		if startCommit.Hash == uncommittedHash && endCommit.Hash == uncommittedHash {
			// Both are uncommitted — show working tree diff.
			raw, err = gitOutput(worktreePath, "diff")
		} else if endCommit.Hash == uncommittedHash {
			// Range from a commit to uncommitted changes.
			raw, err = gitOutput(worktreePath, "diff", startCommit.Hash)
		} else {
			// Normal range: parent of start to end.
			raw, err = gitOutput(worktreePath, "diff", startCommit.Hash+"^.."+endCommit.Hash)
		}
		if err != nil {
			return diffContentMsg{files: nil}
		}
		files := diff.Parse(raw)
		return diffContentMsg{files: files}
	}
}
