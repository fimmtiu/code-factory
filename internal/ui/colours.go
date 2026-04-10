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

	// Greys — ordered light to dark.
	colourBrightWhite = lipgloss.Color("255") // bright white — newly-arrived lines, button text
	colourLightGrey   = lipgloss.Color("250") // light grey — inactive hints, placeholder text
	colourMidGrey     = lipgloss.Color("246") // mid grey — default log message text
	colourDimGrey     = lipgloss.Color("245") // dim grey — dismissed items, worker output
	colourSubtleGrey  = lipgloss.Color("242") // subtle grey — empty-state text, aged timestamps
	colourTimestamp3  = lipgloss.Color("239") // timestamp 1–5 min
	colourDarkGrey    = lipgloss.Color("238") // dark grey — progress bar empty, popup backgrounds
	colourTimestamp1  = lipgloss.Color("236") // timestamp < 1 min, dialog shadow

	// Borders.
	colourBorderBlue = lipgloss.Color("12") // bright blue — pane and dialog borders

	// Worker status colours.
	colourWorkerAwaiting = lipgloss.Color("9")  // red — awaiting permission
	colourWorkerBusy     = lipgloss.Color("22") // dark green — busy/active, also user-review/closed
	colourWorkerPaused   = lipgloss.Color("3")  // yellow — paused

	// Log message colours.
	colourLogWorker   = lipgloss.Color("33") // blue — worker name in log entries
	colourLogError    = lipgloss.Color("88") // dark red — error messages
	colourLogPermReq  = lipgloss.Color("94") // orange-brown — permission requests
	colourLogPermResp = lipgloss.Color("75") // soft blue — permission responses
	colourLogCommit   = lipgloss.Color("74") // teal — commit messages
	colourLogClaim    = lipgloss.Color("34") // green — claim messages
	colourLogRelease  = lipgloss.Color("21") // blue — release messages

	// Phase badge colours.
	colourPhaseImplement = lipgloss.Color("37")  // teal
	colourPhaseRefactor  = lipgloss.Color("166") // amber (same as colourWarning)
	colourPhaseReview    = lipgloss.Color("69")  // blue
	colourPhaseRespond   = lipgloss.Color("135") // purple
	colourPhaseBlocked   = lipgloss.Color("124") // dark red

	// Diff view colours.
	colourDiffHunkHeader = lipgloss.Color("159") // light blue — @@ hunk headers
	colourDiffAdded      = lipgloss.Color("194") // green — added lines
	colourDiffRemoved    = lipgloss.Color("224") // pink — removed lines
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
				Foreground(colourDimGrey)

	// diffStatusBarStyle wraps the status bar with a three-sided blue border
	// (top, left, right — no bottom) so it sits flush against the panes below.
	diffStatusBarStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colourBorderBlue).
				BorderBottom(false)
)
