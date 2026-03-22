package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/tickets/internal/models"
)

// DetailPane shows the full details of the currently highlighted work unit.
// It supports scrolling when the content overflows the pane height.
type DetailPane struct {
	unit    *models.WorkUnit
	scrollY int
}

// SetUnit sets the work unit to display and resets the scroll position.
func (dp *DetailPane) SetUnit(unit *models.WorkUnit) {
	dp.unit = unit
	dp.scrollY = 0
}

// ScrollUp scrolls the detail view up by one line, clamped at 0.
func (dp *DetailPane) ScrollUp() {
	if dp.scrollY > 0 {
		dp.scrollY--
	}
}

// ScrollDown scrolls the detail view down by one line, clamped to the
// maximum scroll offset based on content length.
func (dp *DetailPane) ScrollDown() {
	if dp.unit == nil {
		return
	}
	lines := dp.buildLines()
	maxScroll := len(lines) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	if dp.scrollY < maxScroll {
		dp.scrollY++
	}
}

// View renders the detail pane as a string with the given dimensions.
func (dp DetailPane) View(width, height int) string {
	if dp.unit == nil {
		paneStyle := lipgloss.NewStyle().Width(width).Height(height)
		return paneStyle.Render("No item selected")
	}

	lines := dp.buildLines()

	// Apply scroll offset
	start := dp.scrollY
	if start > len(lines) {
		start = len(lines)
	}
	visible := lines[start:]
	if len(visible) > height {
		visible = visible[:height]
	}

	content := strings.Join(visible, "\n")

	paneStyle := lipgloss.NewStyle().
		Width(width).
		Height(height).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder())

	return paneStyle.Render(content)
}

// buildLines constructs the text lines for the current unit.
func (dp DetailPane) buildLines() []string {
	if dp.unit == nil {
		return nil
	}

	labelStyle := lipgloss.NewStyle().Bold(true)

	var lines []string

	// Header: identifier
	lines = append(lines, labelStyle.Render(dp.unit.Identifier))

	// Status line
	lines = append(lines, labelStyle.Render("Status: ")+dp.unit.Status)

	// Dependencies
	if len(dp.unit.Dependencies) > 0 {
		lines = append(lines, labelStyle.Render("Dependencies: ")+strings.Join(dp.unit.Dependencies, ", "))
	} else {
		lines = append(lines, labelStyle.Render("Dependencies: ")+"none")
	}

	// Blank line separator
	lines = append(lines, "")

	// Description (may be multi-line)
	if dp.unit.Description != "" {
		descLines := strings.Split(dp.unit.Description, "\n")
		lines = append(lines, descLines...)
	}

	return lines
}
