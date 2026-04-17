package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/models"
)

// TODO(theme-migration): The colour constants and style variables below are
// the legacy styling system. Views are being incrementally migrated to read
// from theme.Current() instead. Once all views have been migrated, these
// variables should be deleted and this file removed. Currently only log_view.go
// reads from the theme; all other views still use the variables here.

// ── Colour palette ──────────────────────────────────────────────────────────

// MIGRATION IN PROGRESS: The colours and styles in this file are being
// migrated to theme.Current() view-by-view. Diff views have already been
// migrated; the remaining views still reference these variables as their
// authoritative source. New code should use theme.Current() fields; do
// not add new references to the variables below.

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
	colourPhaseBlocked   = lipgloss.Color("124") // dark red
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
		models.PhaseBlocked:   lipgloss.NewStyle().Foreground(colourPhaseBlocked),
		models.PhaseDone:      lipgloss.NewStyle().Foreground(colourSuccess),
	}

	detailLabelStyle    = lipgloss.NewStyle().Bold(true)
	progressFilledStyle = lipgloss.NewStyle().Foreground(colourSuccess)
	progressEmptyStyle  = lipgloss.NewStyle().Foreground(colourDarkGrey)
	repoNameStyle       = lipgloss.NewStyle().Bold(true).Underline(true)
)

// ── Log view styles ─────────────────────────────────────────────────────────

var (
	logSelectedStyle = lipgloss.NewStyle().
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	logWorkerStyle = lipgloss.NewStyle().
			Foreground(colourLogWorker)
)
