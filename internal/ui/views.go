package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// viewPaneStyle is the blue single-line border applied to Command, Worker, and Log views.
var viewPaneStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("12")) // blue

// emptyStateStyle is applied to placeholder messages such as "No actionable tickets"
// so they read as secondary/hint text rather than content.
var emptyStateStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("242")).
	Italic(true)

// viewBorderOverhead is the number of rows (and columns) consumed by viewPaneStyle.
const viewBorderOverhead = 2

// ViewID is an enum for the four main views.
type ViewID int

const (
	ViewProject ViewID = iota
	ViewCommand
	ViewWorker
	ViewLog
)

// viewNames maps ViewID to a display name.
var viewNames = map[ViewID]string{
	ViewProject: "Projects",
	ViewCommand: "Commands",
	ViewWorker:  "Workers",
	ViewLog:     "Log",
}

// viewModel is the interface that each view sub-model must satisfy.
type viewModel interface {
	tea.Model
	KeyBindings() []KeyBinding
}

// nextView returns the next view in the cycle (project → command → worker → log → project).
func nextView(current ViewID) ViewID {
	return (current + 1) % 4
}

// prevView returns the previous view in the cycle.
func prevView(current ViewID) ViewID {
	return (current + 3) % 4
}
