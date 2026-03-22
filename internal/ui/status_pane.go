package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/tickets/internal/models"
)

// StatusPane renders a compact summary of project and ticket statistics.
// It is not interactive.
type StatusPane struct{}

// statusCounts holds the computed counts for projects or tickets.
type statusCounts struct {
	total      int
	open       int
	inProgress int
	done       int
}

// View renders the status pane with the given dimensions.
// width and height are the outer (total) dimensions including the border.
func (sp StatusPane) View(units []*models.WorkUnit, width, height int) string {
	proj, ticket := computeCounts(units)

	titleStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	lines := []string{
		titleStyle.Render("Status"),
		"",
		labelStyle.Render("Projects:"),
		fmt.Sprintf("  Total:       %d", proj.total),
		fmt.Sprintf("  Open:        %d", proj.open),
		fmt.Sprintf("  In Progress: %d", proj.inProgress),
		fmt.Sprintf("  Done:        %d", proj.done),
		"",
		labelStyle.Render("Tickets:"),
		fmt.Sprintf("  Total:       %d", ticket.total),
		fmt.Sprintf("  Open:        %d", ticket.open),
		fmt.Sprintf("  In Progress: %d", ticket.inProgress),
		fmt.Sprintf("  Done:        %d", ticket.done),
	}

	contentHeight := height - 2 // top and bottom borders each consume 1 row
	content := ""
	for i, line := range lines {
		if i >= contentHeight {
			break
		}
		if i > 0 {
			content += "\n"
		}
		content += line
	}

	paneStyle := lipgloss.NewStyle().
		Width(width - 2).
		Height(contentHeight).
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(lipgloss.Color("240"))

	return paneStyle.Render(content)
}

// computeCounts separates units into projects and tickets and tallies statuses.
func computeCounts(units []*models.WorkUnit) (proj statusCounts, ticket statusCounts) {
	for _, u := range units {
		if u.IsProject {
			proj.total++
			switch u.Status {
			case models.ProjectOpen:
				proj.open++
			case models.ProjectInProgress:
				proj.inProgress++
			case models.ProjectDone:
				proj.done++
			}
		} else {
			ticket.total++
			switch u.Status {
			case models.StatusOpen:
				ticket.open++
			case models.StatusInProgress, models.StatusReviewReady, models.StatusInReview:
				ticket.inProgress++
			case models.StatusDone:
				ticket.done++
			}
		}
	}
	return proj, ticket
}
