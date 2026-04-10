package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/models"
)

// ── Colour palette ──────────────────────────────────────────────────────────

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

// ── Shared styles ───────────────────────────────────────────────────────────

var (
	// viewPaneStyle is the blue single-line border applied to Command, Worker,
	// and Log views.
	viewPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourBorderBlue)

	// emptyStateStyle is applied to placeholder messages such as
	// "No actionable tickets" so they read as secondary/hint text.
	emptyStateStyle = lipgloss.NewStyle().
			Foreground(colourSubtleGrey).
			Italic(true)

	// hintKeyStyle renders a keystroke label bold in the muted hint colour.
	hintKeyStyle = lipgloss.NewStyle().Foreground(colourMuted).Bold(true)

	// hintDescStyle renders hint description text in the normal muted colour.
	hintDescStyle = lipgloss.NewStyle().Foreground(colourMuted)
)

// ── App chrome styles ───────────────────────────────────────────────────────

var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	tabBaseStyle = lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1)

	activeTabStyle = lipgloss.NewStyle().
			Foreground(colourOnPrimary).
			Background(colourPrimary).
			Inherit(tabBaseStyle)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(colourAccent).
				Inherit(tabBaseStyle)

	// helpHintStyle adds padding around hint text; colouring is done by
	// hintKeyStyle / hintDescStyle.
	helpHintStyle = lipgloss.NewStyle().Padding(0, 1)
)

// ── Dialog styles ───────────────────────────────────────────────────────────

var (
	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourPrimary).
			Padding(1, 2)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Background(colourPrimary).
				Foreground(colourOnPrimary).
				Padding(0, 1).
				MarginBottom(1)

	buttonBaseStyle = lipgloss.NewStyle().Padding(0, 2)

	buttonNormalStyle = lipgloss.NewStyle().
				Background(colourLightGrey).
				Foreground(colourBrightWhite).
				Inherit(buttonBaseStyle)

	buttonFocusedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary).
				Inherit(buttonBaseStyle)

	// editorWaitingStyle is used for the "Waiting for editor..." overlay.
	editorWaitingStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colourBorderBlue).
				BorderBackground(colourDarkGrey).
				Background(colourDarkGrey).
				Foreground(colourOnPrimary).
				Bold(true).
				Padding(0, 2)

	// notifStyle is the visual style for ephemeral pop-up notifications.
	// Dark background with bright text and an amber border for high visibility.
	notifStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourWarning).
			BorderBackground(colourDarkGrey).
			Background(colourDarkGrey).
			Foreground(colourOnPrimary).
			Bold(true).
			Padding(0, 2)
)

// ── Permission dialog styles ────────────────────────────────────────────────

var (
	permOptionNormalStyle = lipgloss.NewStyle().Padding(0, 1)

	permOptionSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Reverse(true).
				Padding(0, 1)
)

// ── Quick-response dialog styles ────────────────────────────────────────────

var (
	quickResponseOutputStyle = lipgloss.NewStyle().
					Foreground(colourMuted)

	quickResponseInputStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colourPrimary).
				Padding(0, 1)
)

// ── Ticket dialog styles ────────────────────────────────────────────────────

var (
	tdSelectedStyle   = lipgloss.NewStyle().Background(colourPrimary).Foreground(colourOnPrimary)
	tdDismissedStyle  = lipgloss.NewStyle().Foreground(colourDimGrey)
	tdClosedStyle     = lipgloss.NewStyle().Foreground(colourWorkerBusy)
	tdSectionStyle    = lipgloss.NewStyle().Bold(true)
	hintInactiveStyle = lipgloss.NewStyle().Foreground(colourLightGrey)
)

// ── Command view styles ─────────────────────────────────────────────────────

var (
	cmdSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	cmdNeedsAttentionStyle = lipgloss.NewStyle().
				Foreground(colourWarning)

	cmdUserReviewStyle = lipgloss.NewStyle().
				Foreground(colourWorkerBusy)

	cmdErrorStyle = lipgloss.NewStyle().Foreground(colourDanger)
)

// ── Project view styles ─────────────────────────────────────────────────────

// accentBorder is a DoubleBorder variant that replaces the left edge with a
// solid half-block character (▌) to draw a coloured accent bar on the focused pane.
var accentBorder = func() lipgloss.Border {
	b := lipgloss.DoubleBorder()
	b.Left = "▌"
	b.TopLeft = "╭"
	b.BottomLeft = "╰"
	return b
}()

var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(accentBorder).
				BorderForeground(colourBorderBlue)

	unfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colourMuted)

	statusPaneStyle = unfocusedBorderStyle

	// Tree item styles.
	treeBlockedStyle  = lipgloss.NewStyle().Foreground(colourMuted)
	treeDoneStyle     = lipgloss.NewStyle().Underline(true)
	treeDefaultStyle  = lipgloss.NewStyle()
	treeSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	// Phase badge styles (keyed by ticket phase).
	phaseBadgeStyles = map[models.TicketPhase]lipgloss.Style{
		models.PhaseImplement: lipgloss.NewStyle().Foreground(colourPhaseImplement),
		models.PhaseRefactor:  lipgloss.NewStyle().Foreground(colourPhaseRefactor),
		models.PhaseReview:    lipgloss.NewStyle().Foreground(colourPhaseReview),
		models.PhaseRespond:   lipgloss.NewStyle().Foreground(colourPhaseRespond),
		models.PhaseBlocked:   lipgloss.NewStyle().Foreground(colourPhaseBlocked),
		models.PhaseDone:      lipgloss.NewStyle().Foreground(colourSuccess),
	}

	detailLabelStyle    = lipgloss.NewStyle().Bold(true)
	progressFilledStyle = lipgloss.NewStyle().Foreground(colourSuccess)
	progressEmptyStyle  = lipgloss.NewStyle().Foreground(colourDarkGrey)
	repoNameStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
)

// ── Worker view styles ──────────────────────────────────────────────────────

var (
	workerStatusStyle = lipgloss.NewStyle().Bold(true)

	workerIdleStyle     = lipgloss.NewStyle().Foreground(colourMuted).Inherit(workerStatusStyle)
	workerAwaitingStyle = lipgloss.NewStyle().Foreground(colourWorkerAwaiting).Inherit(workerStatusStyle)
	workerBusyStyle     = lipgloss.NewStyle().Foreground(colourWorkerBusy).Inherit(workerStatusStyle)
	workerPausedStyle   = lipgloss.NewStyle().Foreground(colourWorkerPaused).Inherit(workerStatusStyle)

	workerOutputStyle = lipgloss.NewStyle().
				Foreground(colourDimGrey)

	workerNoOutputStyle = lipgloss.NewStyle().
				Foreground(colourLightGrey)

	workerNewLineStyle = lipgloss.NewStyle().
				Foreground(colourBrightWhite).Bold(true)
)

// ── Log view styles ─────────────────────────────────────────────────────────

var (
	logSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	logWorkerStyle = lipgloss.NewStyle().
			Foreground(colourLogWorker)
)

// ── Diff view styles ────────────────────────────────────────────────────────

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

	// commitHashStyle renders the short hash prefix in bold medium grey.
	commitHashStyle = lipgloss.NewStyle().Bold(true).Foreground(colourMuted)

	// diffLabelBold is the style for the "Ticket: <id>" / "Project: <id>" label.
	diffLabelBold = lipgloss.NewStyle().Bold(true)

	// diffErrorStyle renders error messages in the diff view status bar.
	diffErrorStyle = lipgloss.NewStyle().Foreground(colourDanger)
)

// ── Diff renderer styles ────────────────────────────────────────────────────

var (
	diffHunkHeaderStyle = lipgloss.NewStyle().Background(colourDiffHunkHeader)
	diffAddedStyle      = lipgloss.NewStyle().Background(colourDiffAdded)
	diffRemovedStyle    = lipgloss.NewStyle().Background(colourDiffRemoved)
	diffFileHeaderStyle = lipgloss.NewStyle().Bold(true)
	diffDeletedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(colourDiffDeleted)
	diffRenamedMsgStyle = lipgloss.NewStyle().Bold(true).Foreground(colourDiffRenamed)
)
