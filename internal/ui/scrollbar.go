package ui

import "strings"

// scrollbarThumb calculates the 0-indexed start row and height of a scroll
// thumb within the innerH content rows of a pane.
// Returns (0, 0) when all content fits and no indicator is needed.
func scrollbarThumb(innerH, offset, total int) (start, size int) {
	if total <= innerH || innerH <= 0 {
		return 0, 0
	}
	size = max(1, innerH*innerH/total)
	maxOffset := total - innerH
	if maxOffset <= 0 {
		return 0, size
	}
	start = (offset * (innerH - size)) / maxOffset
	return start, size
}

// injectScrollbar replaces the right border character on the appropriate rows
// of a lipgloss-rendered pane with thumbChar to show a scroll position indicator.
//
// borderChar must match the pane's right-border character (e.g. "│" for
// NormalBorder/RoundedBorder, "║" for DoubleBorder/accentBorder).
// innerH is the number of content rows (pane height minus the two border rows).
func injectScrollbar(rendered, borderChar, thumbChar string, offset, total, innerH int) string {
	thumbStart, thumbSize := scrollbarThumb(innerH, offset, total)
	if thumbSize == 0 {
		return rendered
	}

	lines := strings.Split(rendered, "\n")
	// Drop a trailing empty element produced by a trailing newline, if present.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	for i, line := range lines {
		if i == 0 || i == len(lines)-1 {
			continue // skip top and bottom border rows
		}
		contentRow := i - 1 // 0-indexed within content rows
		if contentRow >= thumbStart && contentRow < thumbStart+thumbSize {
			idx := strings.LastIndex(line, borderChar)
			if idx >= 0 {
				lines[i] = line[:idx] + thumbChar + line[idx+len(borderChar):]
			}
		}
	}
	return strings.Join(lines, "\n")
}
