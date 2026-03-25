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

// PageUp scrolls the detail view up by n lines, clamped at 0.
func (dp *DetailPane) PageUp(n int) {
	dp.scrollY -= n
	if dp.scrollY < 0 {
		dp.scrollY = 0
	}
}

// PageDown scrolls the detail view down by n lines, clamped to the last line.
func (dp *DetailPane) PageDown(n int) {
	if dp.unit == nil {
		return
	}
	lines := dp.buildLines()
	maxScroll := len(lines) - 1
	if maxScroll < 0 {
		maxScroll = 0
	}
	dp.scrollY += n
	if dp.scrollY > maxScroll {
		dp.scrollY = maxScroll
	}
}

// View renders the detail pane with the given outer dimensions.
// focused controls the border highlight colour.
func (dp DetailPane) View(width, height int, focused bool) string {
	// Full border takes 1 row/col on each side; content area is 2 smaller.
	contentHeight := height - 2

	borderColor := lipgloss.Color("240")
	if focused {
		borderColor = lipgloss.Color("12")
	}

	paneStyle := lipgloss.NewStyle().
		Width(width-2).
		Height(contentHeight).
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(borderColor)

	if dp.unit == nil {
		return paneStyle.Render("No item selected")
	}

	lines := dp.buildLines()

	start := dp.scrollY
	if start > len(lines) {
		start = len(lines)
	}
	visible := lines[start:]
	if len(visible) > contentHeight {
		visible = visible[:contentHeight]
	}

	return paneStyle.Render(strings.Join(visible, "\n"))
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

	// Phase / Status line
	lines = append(lines, labelStyle.Render("Phase: ")+dp.unit.Phase+"  "+labelStyle.Render("Status: ")+dp.unit.Status)

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

	// Change requests
	if len(dp.unit.ChangeRequests) > 0 {
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render("Change requests:"))
		for _, cr := range dp.unit.ChangeRequests {
			header := cr.CodeLocation + " [" + cr.Status + "] (id: " + cr.ID + ")"
			lines = append(lines, "  "+labelStyle.Render(header))
			dateFmt := cr.Date.Format("2006-01-02 15:04:05")
			lines = append(lines, "    "+labelStyle.Render(cr.Author)+" ("+dateFmt+")")
			for _, textLine := range strings.Split(cr.Description, "\n") {
				lines = append(lines, "      "+textLine)
			}
		}
	}

	return lines
}
