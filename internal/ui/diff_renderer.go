package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/diff"
)

// Diff rendering styles.
var (
	diffHunkHeaderStyle = lipgloss.NewStyle().Background(colourDiffHunkHeader)
	diffAddedStyle      = lipgloss.NewStyle().Background(colourDiffAdded)
	diffRemovedStyle    = lipgloss.NewStyle().Background(colourDiffRemoved)
	diffFileHeaderStyle = lipgloss.NewStyle().Bold(true)
	diffDeletedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(colourDiffDeleted)
	diffRenamedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(colourDiffRenamed)
)

// renderedDiff holds the formatted diff output together with the line offsets
// where each file section starts. Computing offsets during rendering avoids
// fragile re-parsing of the output string.
type renderedDiff struct {
	text       string // the formatted diff output
	fileStarts []int  // line offset where each file's blank-separator line begins
}

// renderDiff produces a formatted diff string from parsed diff files.
// Each file begins with a blank line (including the first file). paneWidth
// controls the full-width background padding for hunk headers and
// added/removed lines.
func renderDiff(files []diff.File, paneWidth int) string {
	return renderDiffResult(files, paneWidth, nil).text
}

// renderDiffResult is the full-featured renderer that returns both the
// formatted text and per-file start offsets. collapsed controls per-file
// collapse state; nil means all expanded.
func renderDiffResult(files []diff.File, paneWidth int, collapsed []bool) renderedDiff {
	if len(files) == 0 {
		return renderedDiff{}
	}

	var sb strings.Builder
	lineCount := 0 // tracks the current line number in the output
	fileStarts := make([]int, 0, len(files))

	for i, f := range files {
		isCollapsed := len(collapsed) > i && collapsed[i]
		fileStarts = append(fileStarts, lineCount)
		sb.WriteString("\n") // blank line before each file (including the first)
		lineCount++

		indicator := "▽ "
		if isCollapsed {
			indicator = "▶ "
		}
		sb.WriteString(diffFileHeaderStyle.Render(indicator + f.Name + ":"))
		sb.WriteString("\n")
		lineCount++

		if isCollapsed {
			continue
		}

		switch f.Type {
		case diff.Binary:
			sb.WriteString("  ")
			sb.WriteString(emptyStateStyle.Render("(binary stuff)"))
			sb.WriteString("\n")
			lineCount++
		case diff.Delete:
			sb.WriteString("  ")
			sb.WriteString(diffDeletedMsgStyle.Render("Deleted"))
			sb.WriteString("\n")
			lineCount++
		case diff.Rename:
			sb.WriteString("  ")
			sb.WriteString(diffRenamedMsgStyle.Render("Renamed to "))
			sb.WriteString(f.RenameTo)
			sb.WriteString("\n")
			lineCount++
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, paneWidth)
			}
		default:
			// Normal and New files: render hunks.
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, paneWidth)
			}
		}
	}

	// Trim the trailing newline so callers get clean output.
	return renderedDiff{
		text:       strings.TrimRight(sb.String(), "\n"),
		fileStarts: fileStarts,
	}
}

// renderHunk renders a single hunk: the @@ header followed by content lines.
// It returns the number of lines written.
func renderHunk(sb *strings.Builder, h diff.Hunk, paneWidth int) int {
	lines := 0

	// Hunk header.
	header := "@@"
	if h.Context != "" {
		header += " " + h.Context
	}
	sb.WriteString(padToWidth(diffHunkHeaderStyle, header, paneWidth))
	sb.WriteString("\n")
	lines++

	// Determine the line-number column width from the max line number in this hunk.
	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		text := expandTabs(line.Content)
		switch line.Type {
		case diff.LineRemoved:
			// Blank line-number space, then content with pink background.
			prefix := strings.Repeat(" ", numWidth) + " "
			content := prefix + text
			sb.WriteString(padToWidth(diffRemovedStyle, content, paneWidth))
		case diff.LineAdded:
			// Line number on the left, then content with green background.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			content := prefix + text
			sb.WriteString(padToWidth(diffAddedStyle, content, paneWidth))
			lineNum++
		case diff.LineContext:
			// Line number on the left, plain text (no background).
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			sb.WriteString(prefix + text)
			lineNum++
		}
		sb.WriteString("\n")
		lines++
	}
	return lines
}

// padToWidth pads text with spaces to fill paneWidth, then applies the style.
// This ensures background colours extend to the full pane width.
func padToWidth(style lipgloss.Style, text string, paneWidth int) string {
	textWidth := lipgloss.Width(text)
	if textWidth < paneWidth {
		text += strings.Repeat(" ", paneWidth-textWidth)
	}
	return style.Render(text)
}

// digitCount returns the number of decimal digits in n.
func digitCount(n int) int {
	if n <= 0 {
		return 1
	}
	count := 0
	for n > 0 {
		count++
		n /= 10
	}
	return count
}

// expandTabs replaces tab characters with spaces. Terminals render tabs at
// variable widths, but lipgloss.Width counts each tab as 1 column, so styled
// lines padded to the pane width overshoot and wrap. Using fixed 4-space tabs
// keeps the width calculation and the terminal rendering in agreement.
func expandTabs(s string) string {
	return strings.ReplaceAll(s, "\t", "    ")
}

// fileNamesFromDiff returns ordered file names from the parsed diff.
func fileNamesFromDiff(files []diff.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}
