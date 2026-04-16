package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/diff"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// diffLineKind classifies each rendered line for the line-select feature.
type diffLineKind int

const (
	diffLineNonSelectable diffLineKind = iota // blank, filename, "Deleted", hunk header, etc.
	diffLineHunkContent                       // context, added, or removed line inside a hunk
)

// diffLineMeta records per-line metadata produced during rendering. The viewer
// uses this to determine which lines are selectable and which file they belong to.
type diffLineMeta struct {
	kind      diffLineKind
	fileIndex int // index into the files slice (-1 for non-selectable)
	lineNum   int // new-file line number for selectable lines; 0 otherwise
}

// renderedDiff holds the formatted diff output together with the line offsets
// where each file section starts. Computing offsets during rendering avoids
// fragile re-parsing of the output string.
type renderedDiff struct {
	text       string         // the formatted diff output
	fileStarts []int          // line offset where each file's blank-separator line begins
	lineMeta   []diffLineMeta // per-line metadata for the entire rendered output
}

// renderContext bundles per-render state that would otherwise be threaded
// through renderDiffResult -> renderHunk as individual parameters. Adding
// a new per-line annotation source (e.g. lint warnings) requires only a
// new field here and a method on this struct, not a signature change to
// every rendering function in the chain.
type renderContext struct {
	paneWidth   int
	crLocations map[string]bool // "file:line" -> true for lines with change requests
	meta        []diffLineMeta  // per-line metadata accumulated during rendering
	fileIndex   int             // current file index during rendering
	fileName    string          // current file name during rendering
}

// HasAnnotation returns true if the current file and the given line number
// has a change request annotation. This centralises the "file:line" key
// format in one place instead of scattering fmt.Sprintf calls across the
// hot loop.
func (rc *renderContext) HasAnnotation(lineNum int) bool {
	return rc.crLocations[fmt.Sprintf("%s:%d", rc.fileName, lineNum)]
}

// appendMeta appends n metadata entries for the current file with the given
// kind and line number.
func (rc *renderContext) appendMeta(n int, kind diffLineKind, lineNum int) {
	for range n {
		rc.meta = append(rc.meta, diffLineMeta{kind: kind, fileIndex: rc.fileIndex, lineNum: lineNum})
	}
}

// crEmojiSuffix is the 4-visual-column suffix appended to lines with CRs:
// space + speech balloon (2 cells) + space = 4 columns.
const crEmojiSuffix = " \U0001F4AC "

// renderDiff produces a formatted diff string from parsed diff files.
// Each file begins with a blank line (including the first file). paneWidth
// controls the full-width background padding for hunk headers and
// added/removed lines.
func renderDiff(files []diff.File, paneWidth int) string {
	return renderDiffResult(files, paneWidth, nil, nil).text
}

// renderDiffResult is the full-featured renderer that returns both the
// formatted text and per-file start offsets. collapsed controls per-file
// collapse state; nil means all expanded. crLocations maps "file:line"
// strings to true for lines that have change requests; those lines render
// with a speech balloon emoji suffix and wrap 4 columns sooner.
func renderDiffResult(files []diff.File, paneWidth int, collapsed []bool, crLocations map[string]bool) renderedDiff {
	if len(files) == 0 {
		return renderedDiff{}
	}

	var sb strings.Builder
	lineCount := 0 // tracks the current line number in the output
	fileStarts := make([]int, 0, len(files))
	rc := &renderContext{
		paneWidth:   paneWidth,
		crLocations: crLocations,
	}

	for i, f := range files {
		rc.fileIndex = i
		rc.fileName = f.Name
		isCollapsed := len(collapsed) > i && collapsed[i]
		fileStarts = append(fileStarts, lineCount)
		sb.WriteString("\n") // blank line before each file (including the first)
		rc.appendMeta(1, diffLineNonSelectable, 0)
		lineCount++

		indicator := "▽ "
		if isCollapsed {
			indicator = "▶ "
		}
		sb.WriteString(theme.Current().DiffFileHeaderStyle.Render(indicator + f.Name + ":"))
		sb.WriteString("\n")
		rc.appendMeta(1, diffLineNonSelectable, 0)
		lineCount++

		if isCollapsed {
			continue
		}

		switch f.Type {
		case diff.Binary:
			sb.WriteString("  ")
			sb.WriteString(theme.Current().EmptyStateStyle.Render("(binary stuff)"))
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
		case diff.Delete:
			sb.WriteString("  ")
			sb.WriteString(theme.Current().DiffDeletedMsgStyle.Render("Deleted"))
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
		case diff.Rename:
			sb.WriteString("  ")
			sb.WriteString(theme.Current().DiffRenamedMsgStyle.Render("Renamed to "))
			sb.WriteString(f.RenameTo)
			sb.WriteString("\n")
			rc.appendMeta(1, diffLineNonSelectable, 0)
			lineCount++
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, rc)
			}
		default:
			// Normal and New files: render hunks.
			for _, h := range f.Hunks {
				lineCount += renderHunk(&sb, h, rc)
			}
		}
	}

	// Trim the trailing newline so callers get clean output.
	return renderedDiff{
		text:       strings.TrimRight(sb.String(), "\n"),
		fileStarts: fileStarts,
		lineMeta:   rc.meta,
	}
}

// renderHunk renders a single hunk: the @@ header followed by content lines.
// It returns the number of lines written and appends per-line metadata to rc.
func renderHunk(sb *strings.Builder, h diff.Hunk, rc *renderContext) int {
	lines := 0

	// Hunk header.
	header := "@@"
	if h.Context != "" {
		header += " " + h.Context
	}
	styled, n := padToWidth(theme.Current().DiffHunkHeaderStyle, header, rc.paneWidth)
	sb.WriteString(styled)
	sb.WriteString("\n")
	rc.appendMeta(n, diffLineNonSelectable, 0)
	lines += n

	// Determine the line-number column width from the max line number in this hunk.
	maxLineNum := h.NewStart + h.NewCount
	numWidth := digitCount(maxLineNum)

	lineNum := h.NewStart
	for _, line := range h.Lines {
		text := expandTabs(line.Content)
		hasCR := rc.HasAnnotation(lineNum)
		switch line.Type {
		case diff.LineRemoved:
			// Blank line-number space, then content with pink background.
			prefix := strings.Repeat(" ", numWidth) + " "
			content := prefix + text
			n := writeStyledLine(sb, theme.Current().DiffRemovedStyle, content, rc.paneWidth, hasCR)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
		case diff.LineAdded:
			// Line number on the left, then content with green background.
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			content := prefix + text
			n := writeStyledLine(sb, theme.Current().DiffAddedStyle, content, rc.paneWidth, hasCR)
			rc.appendMeta(n, diffLineHunkContent, lineNum)
			lines += n
			lineNum++
		case diff.LineContext:
			// Line number on the left, plain text (no background).
			prefix := fmt.Sprintf("%*d ", numWidth, lineNum)
			if hasCR {
				sb.WriteString(truncateToWidth(prefix+text, rc.paneWidth-4) + crEmojiSuffix)
			} else {
				sb.WriteString(prefix + text)
			}
			rc.appendMeta(1, diffLineHunkContent, lineNum)
			lines++
			lineNum++
		}
		sb.WriteString("\n")
	}
	return lines
}

// writeStyledLine writes a padded, styled content line to sb, appending the CR
// emoji suffix when hasCR is true. It returns the number of visual lines written.
func writeStyledLine(sb *strings.Builder, style lipgloss.Style, content string, paneWidth int, hasCR bool) int {
	if hasCR {
		styled, n := padToWidth(style, content, paneWidth-4)
		sb.WriteString(appendCREmojiToStyled(styled, n))
		return n
	}
	styled, n := padToWidth(style, content, paneWidth)
	sb.WriteString(styled)
	return n
}

// appendCREmojiToStyled appends the CR emoji suffix to a styled line.
// For single-line output (n == 1), the emoji is appended directly.
// For multi-line (wrapped) output, the emoji is appended to the first line
// and 4 spaces are appended to subsequent lines so all visual lines have
// the same total width.
func appendCREmojiToStyled(styled string, n int) string {
	if n == 1 {
		return styled + crEmojiSuffix
	}
	parts := strings.Split(styled, "\n")
	parts[0] += crEmojiSuffix
	for i := 1; i < len(parts); i++ {
		parts[i] += "    "
	}
	return strings.Join(parts, "\n")
}

// truncateToWidth truncates text to fit within maxWidth visual columns.
func truncateToWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= maxWidth {
		return text
	}
	runes := []rune(text)
	for i := len(runes); i > 0; i-- {
		if lipgloss.Width(string(runes[:i])) <= maxWidth {
			return string(runes[:i])
		}
	}
	return ""
}

// padToWidth pads text with spaces to fill paneWidth, then applies the style.
// If the text is wider than paneWidth, it wraps into multiple lines so that
// background colours extend to the full pane width on every visual line.
// Returns the styled text and the number of visual lines it occupies.
func padToWidth(style lipgloss.Style, text string, paneWidth int) (string, int) {
	if paneWidth <= 0 {
		return style.Render(text), 1
	}
	textWidth := lipgloss.Width(text)
	if textWidth <= paneWidth {
		if textWidth < paneWidth {
			text += strings.Repeat(" ", paneWidth-textWidth)
		}
		return style.Render(text), 1
	}
	// Text is wider than the pane: wrap into multiple lines, each padded
	// to paneWidth, so the background colour fills every visual line.
	var parts []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := 0
		for end < len(runes) {
			if lipgloss.Width(string(runes[:end+1])) > paneWidth {
				break
			}
			end++
		}
		if end == 0 {
			end = 1 // at least one rune per line
		}
		chunk := string(runes[:end])
		runes = runes[end:]
		if w := lipgloss.Width(chunk); w < paneWidth {
			chunk += strings.Repeat(" ", paneWidth-w)
		}
		parts = append(parts, style.Render(chunk))
	}
	return strings.Join(parts, "\n"), len(parts)
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
