package ui

import "github.com/charmbracelet/lipgloss"

// Semantic colour palette shared across all views.
var (
	colourPrimary   = lipgloss.Color("62")  // blue — focused borders, selected backgrounds
	colourAccent    = lipgloss.Color("67")  // slate-blue — inactive tabs, secondary highlights
	colourSuccess   = lipgloss.Color("28")  // green — done/closed items, progress fill
	colourWarning   = lipgloss.Color("166") // amber — needs-attention, refactor phase, logfile indicator
	colourDanger    = lipgloss.Color("196") // red — errors
	colourMuted     = lipgloss.Color("245") // grey — hints, unfocused borders, dimmed text
	colourOnPrimary = lipgloss.Color("230") // near-white — text on primary/blue backgrounds
)
