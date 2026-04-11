package theme

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// Tan returns a Theme that reproduces the original hard-coded styles from
// internal/ui/styles.go and internal/ui/log_view.go.
func Tan() *Theme {
	// ── Colour palette ──────────────────────────────────────────────────
	colourPrimary := lipgloss.Color("62")
	colourAccent := lipgloss.Color("67")
	colourSuccess := lipgloss.Color("28")
	colourWarning := lipgloss.Color("166")
	colourDanger := lipgloss.Color("196")
	colourMuted := lipgloss.Color("240")
	colourOnPrimary := lipgloss.Color("230")

	colourBrightWhite := lipgloss.Color("255")
	colourLightGrey := lipgloss.Color("250")
	colourMidGrey := lipgloss.Color("246")
	colourDimGrey := lipgloss.Color("245")
	colourSubtleGrey := lipgloss.Color("242")
	colourTimestamp3 := lipgloss.Color("239")
	colourDarkGrey := lipgloss.Color("238")
	colourTimestamp1 := lipgloss.Color("236")

	colourBorderBlue := lipgloss.Color("12")

	colourWorkerAwaiting := lipgloss.Color("9")
	colourWorkerBusy := lipgloss.Color("22")
	colourWorkerPaused := lipgloss.Color("3")

	colourLogWorker := lipgloss.Color("33")
	colourLogError := lipgloss.Color("88")
	colourLogPermReq := lipgloss.Color("94")
	colourLogPermResp := lipgloss.Color("75")
	colourLogCommit := lipgloss.Color("74")
	colourLogClaim := lipgloss.Color("34")
	colourLogRelease := lipgloss.Color("21")

	colourPhaseImplement := lipgloss.Color("37")
	colourPhaseRefactor := lipgloss.Color("166")
	colourPhaseReview := lipgloss.Color("69")
	colourPhaseRespond := lipgloss.Color("135")
	colourPhaseBlocked := lipgloss.Color("124")

	colourDiffHunkHeader := lipgloss.Color("159")
	colourDiffAdded := lipgloss.Color("194")
	colourDiffRemoved := lipgloss.Color("224")
	colourDiffDeleted := lipgloss.Color("52")
	colourDiffRenamed := lipgloss.Color("18")

	// ── Base styles (used by derived styles via Inherit) ────────────────
	tabBaseStyle := lipgloss.NewStyle().
		Bold(true).
		Padding(0, 1)

	buttonBaseStyle := lipgloss.NewStyle().Padding(0, 2)

	workerStatusStyle := lipgloss.NewStyle().Bold(true)

	// ── Accent border ───────────────────────────────────────────────────
	accentBorder := lipgloss.DoubleBorder()
	accentBorder.Left = "▌"
	accentBorder.TopLeft = "╭"
	accentBorder.BottomLeft = "╰"

	unfocusedBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colourMuted)

	return &Theme{
		// ── Shared styles ───────────────────────────────────────────
		ViewPaneStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourBorderBlue),
		EmptyStateStyle: lipgloss.NewStyle().
			Foreground(colourSubtleGrey).
			Italic(true),
		HintKeyStyle:  lipgloss.NewStyle().Foreground(colourMuted).Bold(true),
		HintDescStyle: lipgloss.NewStyle().Foreground(colourMuted),

		// ── App chrome ──────────────────────────────────────────────
		HeaderStyle: lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1),
		TabBaseStyle: tabBaseStyle,
		ActiveTabStyle: lipgloss.NewStyle().
			Foreground(colourOnPrimary).
			Background(colourPrimary).
			Inherit(tabBaseStyle),
		InactiveTabStyle: lipgloss.NewStyle().
			Foreground(colourAccent).
			Inherit(tabBaseStyle),
		HelpHintStyle: lipgloss.NewStyle().Padding(0, 1),

		// ── Dialog ──────────────────────────────────────────────────
		DialogBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourPrimary).
			Padding(1, 2),
		DialogTitleStyle: lipgloss.NewStyle().
			Bold(true).
			Background(colourPrimary).
			Foreground(colourOnPrimary).
			Padding(0, 1).
			MarginBottom(1),
		ButtonBaseStyle: buttonBaseStyle,
		ButtonNormalStyle: lipgloss.NewStyle().
			Background(colourLightGrey).
			Foreground(colourBrightWhite).
			Inherit(buttonBaseStyle),
		ButtonFocusedStyle: lipgloss.NewStyle().
			Background(colourPrimary).
			Foreground(colourOnPrimary).
			Inherit(buttonBaseStyle),
		EditorWaitingStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourBorderBlue).
			BorderBackground(colourDarkGrey).
			Background(colourDarkGrey).
			Foreground(colourOnPrimary).
			Bold(true).
			Padding(0, 2),
		NotifStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourWarning).
			BorderBackground(colourDarkGrey).
			Background(colourDarkGrey).
			Foreground(colourOnPrimary).
			Bold(true).
			Padding(0, 2),

		// ── Permission dialog ───────────────────────────────────────
		PermOptionNormalStyle: lipgloss.NewStyle().Padding(0, 1),
		PermOptionSelectedStyle: lipgloss.NewStyle().
			Bold(true).
			Reverse(true).
			Padding(0, 1),

		// ── Quick response ──────────────────────────────────────────
		QuickResponseOutputStyle: lipgloss.NewStyle().
			Foreground(colourMuted),
		QuickResponseInputStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourPrimary).
			Padding(0, 1),

		// ── Ticket dialog ───────────────────────────────────────────
		TdSelectedStyle:   lipgloss.NewStyle().Background(colourPrimary).Foreground(colourOnPrimary),
		TdDismissedStyle:  lipgloss.NewStyle().Foreground(colourDimGrey),
		TdClosedStyle:     lipgloss.NewStyle().Foreground(colourWorkerBusy),
		TdSectionStyle:    lipgloss.NewStyle().Bold(true),
		HintInactiveStyle: lipgloss.NewStyle().Foreground(colourLightGrey),

		// ── Command view ────────────────────────────────────────────
		CmdSelectedStyle: lipgloss.NewStyle().
			Background(colourPrimary).
			Foreground(colourOnPrimary),
		CmdNeedsAttentionStyle: lipgloss.NewStyle().
			Foreground(colourWarning),
		CmdUserReviewStyle: lipgloss.NewStyle().
			Foreground(colourWorkerBusy),
		CmdErrorStyle: lipgloss.NewStyle().Foreground(colourDanger),

		// ── Project view ────────────────────────────────────────────
		AccentBorder: accentBorder,
		FocusedBorderStyle: lipgloss.NewStyle().
			Border(accentBorder).
			BorderForeground(colourBorderBlue),
		UnfocusedBorderStyle: unfocusedBorderStyle,
		StatusPaneStyle:      unfocusedBorderStyle,
		TreeBlockedStyle:     lipgloss.NewStyle().Foreground(colourMuted),
		TreeDoneStyle:        lipgloss.NewStyle().Underline(true),
		TreeDefaultStyle:     lipgloss.NewStyle(),
		TreeSelectedStyle: lipgloss.NewStyle().
			Background(colourPrimary).
			Foreground(colourOnPrimary),
		DetailLabelStyle:    lipgloss.NewStyle().Bold(true),
		ProgressFilledStyle: lipgloss.NewStyle().Foreground(colourSuccess),
		ProgressEmptyStyle:  lipgloss.NewStyle().Foreground(colourDarkGrey),
		RepoNameStyle:       lipgloss.NewStyle().Bold(true).Underline(true),

		PhaseBadgeStyles: map[models.TicketPhase]lipgloss.Style{
			models.PhaseImplement: lipgloss.NewStyle().Foreground(colourPhaseImplement),
			models.PhaseRefactor:  lipgloss.NewStyle().Foreground(colourPhaseRefactor),
			models.PhaseReview:    lipgloss.NewStyle().Foreground(colourPhaseReview),
			models.PhaseRespond:   lipgloss.NewStyle().Foreground(colourPhaseRespond),
			models.PhaseBlocked:   lipgloss.NewStyle().Foreground(colourPhaseBlocked),
			models.PhaseDone:      lipgloss.NewStyle().Foreground(colourSuccess),
		},

		// ── Worker view ─────────────────────────────────────────────
		WorkerStatusStyle:   workerStatusStyle,
		WorkerIdleStyle:     lipgloss.NewStyle().Foreground(colourMuted).Inherit(workerStatusStyle),
		WorkerAwaitingStyle: lipgloss.NewStyle().Foreground(colourWorkerAwaiting).Inherit(workerStatusStyle),
		WorkerBusyStyle:     lipgloss.NewStyle().Foreground(colourWorkerBusy).Inherit(workerStatusStyle),
		WorkerPausedStyle:   lipgloss.NewStyle().Foreground(colourWorkerPaused).Inherit(workerStatusStyle),
		WorkerOutputStyle: lipgloss.NewStyle().
			Foreground(colourDimGrey),
		WorkerNoOutputStyle: lipgloss.NewStyle().
			Foreground(colourLightGrey),
		WorkerNewLineStyle: lipgloss.NewStyle().
			Foreground(colourBrightWhite).Bold(true),

		// ── Log view ────────────────────────────────────────────────
		LogSelectedStyle: lipgloss.NewStyle().
			Background(colourPrimary).
			Foreground(colourOnPrimary),
		LogWorkerStyle: lipgloss.NewStyle().
			Foreground(colourLogWorker),

		// ── Diff view ───────────────────────────────────────────────
		DiffSelectedStyle: lipgloss.NewStyle().
			Background(colourPrimary).
			Foreground(colourOnPrimary),
		DiffRangeStyle: lipgloss.NewStyle().
			Background(colourAccent).
			Foreground(colourOnPrimary),
		DiffSeparatorStyle: lipgloss.NewStyle().
			Foreground(colourDimGrey),
		DiffStatusBarStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colourBorderBlue).
			BorderBottom(false),
		CommitHashStyle: lipgloss.NewStyle().Bold(true).Foreground(colourMuted),
		DiffLabelBold:   lipgloss.NewStyle().Bold(true),
		DiffErrorStyle:  lipgloss.NewStyle().Foreground(colourDanger),

		// ── Diff renderer ───────────────────────────────────────────
		DiffHunkHeaderStyle: lipgloss.NewStyle().Background(colourDiffHunkHeader),
		DiffAddedStyle:      lipgloss.NewStyle().Background(colourDiffAdded),
		DiffRemovedStyle:    lipgloss.NewStyle().Background(colourDiffRemoved),
		DiffFileHeaderStyle: lipgloss.NewStyle().Bold(true),
		DiffDeletedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(colourDiffDeleted),
		DiffRenamedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(colourDiffRenamed),

		// ── Dynamic functions ───────────────────────────────────────
		LogTimestampStyle: func(age time.Duration) lipgloss.Style {
			switch {
			case age < time.Minute:
				return lipgloss.NewStyle().Foreground(colourTimestamp1)
			case age < 5*time.Minute:
				return lipgloss.NewStyle().Foreground(colourTimestamp3)
			case age < 30*time.Minute:
				return lipgloss.NewStyle().Foreground(colourSubtleGrey)
			default:
				return lipgloss.NewStyle().Foreground(colourDimGrey)
			}
		},
		LogMessageColor: func(msg string) lipgloss.Color {
			switch {
			case strings.HasPrefix(msg, "[mock] error"):
				return colourLogError
			case strings.HasPrefix(msg, "[mock] asking user"):
				return colourLogPermReq
			case strings.HasPrefix(msg, "[mock] received response"):
				return colourLogPermResp
			case strings.HasPrefix(msg, "[mock] committed"):
				return colourLogCommit
			case strings.HasPrefix(msg, "claimed"):
				return colourLogClaim
			case strings.HasPrefix(msg, "released"),
				strings.HasPrefix(msg, "housekeeping: released"):
				return colourLogRelease
			case strings.HasPrefix(msg, "error"),
				strings.HasPrefix(msg, "ACP error"),
				strings.HasPrefix(msg, "housekeeping: error"):
				return colourLogError
			case strings.HasPrefix(msg, "permission request"):
				return colourLogPermReq
			case strings.HasPrefix(msg, "permission response"):
				return colourLogPermResp
			default:
				return colourMidGrey
			}
		},

		// ── Colour values ───────────────────────────────────────────
		DialogShadowColor: colourTimestamp1,
		MutedColor:        colourMuted,
		AccentColor:       colourAccent,
	}
}
