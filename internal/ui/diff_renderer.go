package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/diff"
)

// Diff rendering styles.
var (
	diffHunkHeaderStyle = lipgloss.NewStyle().Background(lipgloss.Color("159"))
	diffAddedStyle      = lipgloss.NewStyle().Background(lipgloss.Color("156"))
	diffRemovedStyle    = lipgloss.NewStyle().Background(lipgloss.Color("219"))
	diffFileHeaderStyle = lipgloss.NewStyle().Bold(true)
	diffDeletedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("52"))
	diffRenamedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("18"))
)

// renderDiff produces a formatted diff string from parsed diff files.
// Each file begins with a blank line (including the first file). paneWidth
// controls the full-width background padding for hunk headers and
// added/removed lines.
func renderDiff(files []diff.File, paneWidth int) string {
	if len(files) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, f := range files {
		sb.WriteString("\n") // blank line before each file (including the first)
		sb.WriteString(renderFileHeader(f))
		sb.WriteString("\n")

		switch f.Type {
		case diff.Binary:
			sb.WriteString("  ")
			sb.WriteString(emptyStateStyle.Render("(binary stuff)"))
			sb.WriteString("\n")
		case diff.Delete:
			sb.WriteString("  ")
			sb.WriteString(diffDeletedMsgStyle.Render("Deleted"))
			sb.WriteString("\n")
		case diff.Rename:
			sb.WriteString("  ")
			sb.WriteString(diffRenamedMsgStyle.Render("Renamed to "))
			sb.WriteString(f.RenameTo)
			sb.WriteString("\n")
		default:
			// Normal and New files: render hunks.
			for _, h := range f.Hunks {
				renderHunk(&sb, h, paneWidth)
			}
		}
	}

	// Trim the trailing newline so callers get clean output.
	return strings.TrimRight(sb.String(), "\n")
}

// renderFileHeader returns the bold "filename:" header line.
func renderFileHeader(f diff.File) string {
	return diffFileHeaderStyle.Render(f.Name + ":")
}

// renderHunk renders a single hunk: the @@ header followed by content lines.
func renderHunk(sb *strings.Builder, h diff.Hunk, paneWidth int) {
	// Hunk header.
	header := "@@"
	if h.Context != "" {
		header += " " + h.Context
	}
	sb.WriteString(padToWidth(diffHunkHeaderStyle, header, paneWidth))
	sb.WriteString("\n")

	// Determine the line-number column width from the max line number in this hunk.
	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		switch line.Type {
		case diff.LineRemoved:
			// Blank line-number space, then content with pink background.
			prefix := strings.Repeat(" ", numWidth) + " "
			content := prefix + line.Content
			sb.WriteString(padToWidth(diffRemovedStyle, content, paneWidth))
		case diff.LineAdded:
			// Line number on the left, then content with green background.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			content := prefix + line.Content
			sb.WriteString(padToWidth(diffAddedStyle, content, paneWidth))
			lineNum++
		case diff.LineContext:
			// Line number on the left, plain text (no background).
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			sb.WriteString(prefix + line.Content)
			lineNum++
		}
		sb.WriteString("\n")
	}
}

// padToWidth pads text with spaces to fill paneWidth, then applies the style.
// This ensures background colours extend to the full pane width.
func padToWidth(style lipgloss.Style, text string, paneWidth int) string {
	textLen := len([]rune(text))
	if textLen < paneWidth {
		text += strings.Repeat(" ", paneWidth-textLen)
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

// fileNamesFromDiff returns ordered file names from the parsed diff.
func fileNamesFromDiff(files []diff.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

// fileStartLines returns the line offset where each file begins in the
// rendered output. Each file's section starts with its blank separator line.
func fileStartLines(rendered string, files []diff.File) []int {
	if len(files) == 0 {
		return nil
	}

	lines := strings.Split(rendered, "\n")
	starts := make([]int, 0, len(files))
	fileIdx := 0

	for i, line := range lines {
		if fileIdx >= len(files) {
			break
		}
		// Each file starts with a blank line, followed by the bold filename.
		if line == "" && i+1 < len(lines) && strings.Contains(lines[i+1], files[fileIdx].Name) {
			starts = append(starts, i)
			fileIdx++
		}
	}

	return starts
}
