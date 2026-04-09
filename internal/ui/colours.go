package ui

import "github.com/charmbracelet/lipgloss"

// Semantic colour palette shared across all views.
var (
	colourPrimary   = lipgloss.Color("62")  // blue — focused borders, selected backgrounds
	colourAccent    = lipgloss.Color("67")  // slate-blue — inactive tabs, secondary highlights
	colourSuccess   = lipgloss.Color("28")  // green — done/closed items, progress fill
	colourWarning   = lipgloss.Color("166") // amber — needs-attention, refactor phase, logfile indicator
	colourDanger    = lipgloss.Color("196") // red — errors
	colourMuted     = lipgloss.Color("240") // grey — hints, unfocused borders, dimmed text
	colourOnPrimary = lipgloss.Color("230") // near-white — text on primary/blue backgrounds

	// Diff view colours.
	colourDiffHunkHeader = lipgloss.Color("159") // light blue — @@ hunk headers
	colourDiffAdded      = lipgloss.Color("156") // green — added lines
	colourDiffRemoved    = lipgloss.Color("219") // pink — removed lines
	colourDiffDeleted    = lipgloss.Color("52")  // dark red — "Deleted" file message
	colourDiffRenamed    = lipgloss.Color("18")  // blue — "Renamed to" file message
)

// ── Diff view styles ─────────────────────────────────────────────────────────

var (
	// diffSelectedStyle highlights the currently focused commit row.
	diffSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	// diffRangeStyle highlights other commits in the selected range.
	diffRangeStyle = lipgloss.NewStyle().
			Background(colourAccent).
			Foreground(colourOnPrimary)

	// diffSeparatorStyle renders the fork-point separator line.
	diffSeparatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")) // medium grey
)
