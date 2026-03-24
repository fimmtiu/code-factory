package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/tickets/internal/models"
)

// StatusPane renders a compact summary of ticket statistics.
// It is not interactive.
type StatusPane struct {
	repoName string
}

// ticketCounts holds the computed counts for tickets.
type ticketCounts struct {
	total      int
	idle       int
	inProgress int
	done       int
}

// View renders the status pane with the given dimensions.
// width and height are the outer (total) dimensions including the border.
func (sp StatusPane) View(units []*models.WorkUnit, width, height int) string {
	counts := computeTicketCounts(units)

	titleStyle := lipgloss.NewStyle().Bold(true).Underline(true)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12"))

	title := sp.repoName
	if title == "" {
		title = "Status"
	}

	lines := []string{
		titleStyle.Render(title),
		"",
		labelStyle.Render("Tickets:"),
		fmt.Sprintf("  Total:       %d", counts.total),
		fmt.Sprintf("  Idle:        %d", counts.idle),
		fmt.Sprintf("  In Progress: %d", counts.inProgress),
		fmt.Sprintf("  Done:        %d", counts.done),
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
		Width(width-2).
		Height(contentHeight).
		Border(lipgloss.NormalBorder(), true).
		BorderForeground(lipgloss.Color("240"))

	return paneStyle.Render(content)
}

// computeTicketCounts tallies ticket phases and statuses, ignoring projects.
func computeTicketCounts(units []*models.WorkUnit) ticketCounts {
	var counts ticketCounts
	for _, u := range units {
		if u.IsProject {
			continue
		}
		counts.total++
		switch u.Phase {
		case models.PhaseDone:
			counts.done++
		default:
			if u.Status == models.StatusInProgress {
				counts.inProgress++
			} else {
				counts.idle++
			}
		}
	}
	return counts
}
