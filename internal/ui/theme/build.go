package theme

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// buildTheme assembles a complete Theme from a Palette. All structural layout
// decisions (bold, padding, border shapes, Inherit chains) live here so that
// theme variants only need to supply colour values.
func buildTheme(p Palette) *Theme {
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
		BorderForeground(p.Muted)

	return &Theme{
		// ── Shared styles ───────────────────────────────────────────
		ViewPaneStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderBlue),
		EmptyStateStyle: lipgloss.NewStyle().
			Foreground(p.SubtleGrey).
			Italic(true),
		HintKeyStyle:  lipgloss.NewStyle().Foreground(p.Muted).Bold(true),
		HintDescStyle: lipgloss.NewStyle().Foreground(p.Muted),

		// ── App chrome ──────────────────────────────────────────────
		HeaderStyle: lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1),
		ActiveTabStyle: lipgloss.NewStyle().
			Foreground(p.OnPrimary).
			Background(p.Primary).
			Inherit(tabBaseStyle),
		InactiveTabStyle: lipgloss.NewStyle().
			Foreground(p.Accent).
			Inherit(tabBaseStyle),
		HelpHintStyle: lipgloss.NewStyle().Padding(0, 1),

		// ── Dialog ──────────────────────────────────────────────────
		DialogBoxStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Primary).
			Padding(1, 2),
		DialogTitleStyle: lipgloss.NewStyle().
			Bold(true).
			Background(p.Primary).
			Foreground(p.OnPrimary).
			Padding(0, 1).
			MarginBottom(1),
		ButtonNormalStyle: lipgloss.NewStyle().
			Background(p.LightGrey).
			Foreground(p.StrongFg).
			Inherit(buttonBaseStyle),
		ButtonFocusedStyle: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.OnPrimary).
			Inherit(buttonBaseStyle),
		EditorWaitingStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderBlue).
			BorderBackground(p.PanelBg).
			Background(p.PanelBg).
			Foreground(p.PanelFg).
			Bold(true).
			Padding(0, 2),
		NotifStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Warning).
			BorderBackground(p.PanelBg).
			Background(p.PanelBg).
			Foreground(p.PanelFg).
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
			Foreground(p.Muted),
		QuickResponseInputStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.Primary).
			Padding(0, 1),

		// ── Ticket dialog ───────────────────────────────────────────
		TdSelectedStyle:   lipgloss.NewStyle().Background(p.Primary).Foreground(p.OnPrimary),
		TdDismissedStyle:  lipgloss.NewStyle().Foreground(p.DimGrey),
		TdClosedStyle:     lipgloss.NewStyle().Foreground(p.WorkerBusy),
		TdSectionStyle:    lipgloss.NewStyle().Bold(true),
		HintInactiveStyle: lipgloss.NewStyle().Foreground(p.LightGrey),

		// ── Command view ────────────────────────────────────────────
		CmdSelectedStyle: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.OnPrimary),
		CmdSelectedUnfocusedStyle: lipgloss.NewStyle().
			Foreground(p.Accent),
		CmdNeedsAttentionStyle: lipgloss.NewStyle().
			Foreground(p.Warning),
		CmdUserReviewStyle: lipgloss.NewStyle().
			Foreground(p.WorkerBusy),
		CmdErrorStyle: lipgloss.NewStyle().Foreground(p.Danger),

		// ── Project view ────────────────────────────────────────────
		AccentBorder: accentBorder,
		FocusedBorderStyle: lipgloss.NewStyle().
			Border(accentBorder).
			BorderForeground(p.BorderBlue),
		UnfocusedBorderStyle: unfocusedBorderStyle,
		StatusPaneStyle:      unfocusedBorderStyle,
		TreeBlockedStyle:     lipgloss.NewStyle().Foreground(p.Muted),
		TreeDoneStyle:        lipgloss.NewStyle().Underline(true),
		TreeDefaultStyle:     lipgloss.NewStyle(),
		TreeSelectedStyle: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.OnPrimary),
		DetailLabelStyle:    lipgloss.NewStyle().Bold(true),
		ProgressFilledStyle: lipgloss.NewStyle().Foreground(p.Success),
		ProgressEmptyStyle:  lipgloss.NewStyle().Foreground(p.DarkGrey),
		RepoNameStyle:       lipgloss.NewStyle().Bold(true).Underline(true),

		PhaseBadgeStyles: map[models.TicketPhase]lipgloss.Style{
			models.PhaseImplement: lipgloss.NewStyle().Foreground(p.PhaseImplement),
			models.PhaseRefactor:  lipgloss.NewStyle().Foreground(p.PhaseRefactor),
			models.PhaseReview:    lipgloss.NewStyle().Foreground(p.PhaseReview),
			models.PhaseRespond:   lipgloss.NewStyle().Foreground(p.PhaseRespond),
			models.PhaseBlocked:   lipgloss.NewStyle().Foreground(p.PhaseBlocked),
			models.PhaseDone:      lipgloss.NewStyle().Foreground(p.Success),
		},

		// ── Worker view ─────────────────────────────────────────────
		WorkerIdleStyle:     lipgloss.NewStyle().Foreground(p.Muted).Inherit(workerStatusStyle),
		WorkerAwaitingStyle: lipgloss.NewStyle().Foreground(p.WorkerAwaiting).Inherit(workerStatusStyle),
		WorkerBusyStyle:     lipgloss.NewStyle().Foreground(p.WorkerBusy).Inherit(workerStatusStyle),
		WorkerPausedStyle:   lipgloss.NewStyle().Foreground(p.WorkerPaused).Inherit(workerStatusStyle),
		WorkerOutputStyle: lipgloss.NewStyle().
			Foreground(p.DimGrey),
		WorkerNoOutputStyle: lipgloss.NewStyle().
			Foreground(p.LightGrey),
		WorkerNewLineStyle: lipgloss.NewStyle().
			Foreground(p.StrongFg).Bold(true),

		// ── Log view ────────────────────────────────────────────────
		LogSelectedStyle: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.OnPrimary),
		LogWorkerStyle: lipgloss.NewStyle().
			Foreground(p.LogWorker),

		// ── Diff view ───────────────────────────────────────────────
		DiffSelectedStyle: lipgloss.NewStyle().
			Background(p.Primary).
			Foreground(p.OnPrimary),
		DiffRangeStyle: lipgloss.NewStyle().
			Background(p.Accent).
			Foreground(p.OnPrimary),
		DiffSeparatorStyle: lipgloss.NewStyle().
			Foreground(p.DimGrey),
		DiffStatusBarStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(p.BorderBlue).
			BorderBottom(false),
		CommitHashStyle: lipgloss.NewStyle().Bold(true).Foreground(p.Muted),
		DiffLabelBold:   lipgloss.NewStyle().Bold(true),
		DiffErrorStyle:  lipgloss.NewStyle().Foreground(p.Danger),

		// ── Diff renderer ───────────────────────────────────────────
		DiffHunkHeaderStyle: lipgloss.NewStyle().Background(p.DiffHunkHeader),
		DiffAddedStyle:      lipgloss.NewStyle().Background(p.DiffAdded),
		DiffRemovedStyle:    lipgloss.NewStyle().Background(p.DiffRemoved),
		DiffFileHeaderStyle: lipgloss.NewStyle().Bold(true),
		DiffDeletedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(p.DiffDeleted),
		DiffRenamedMsgStyle: lipgloss.NewStyle().Bold(true).Foreground(p.DiffRenamed),

		// ── Dynamic functions ───────────────────────────────────────
		LogTimestampStyle: func() func(time.Duration) lipgloss.Style {
			fresh := lipgloss.NewStyle().Foreground(p.TimestampTier1)
			recent := lipgloss.NewStyle().Foreground(p.TimestampTier2)
			aging := lipgloss.NewStyle().Foreground(p.TimestampTier3)
			old := lipgloss.NewStyle().Foreground(p.TimestampTier4)
			return func(age time.Duration) lipgloss.Style {
				switch {
				case age < time.Minute:
					return fresh
				case age < 5*time.Minute:
					return recent
				case age < 30*time.Minute:
					return aging
				default:
					return old
				}
			}
		}(),
		LogCategoryColors: map[models.LogCategory]lipgloss.Color{
			models.LogCategoryError:    p.LogError,
			models.LogCategoryPermReq:  p.LogPermReq,
			models.LogCategoryPermResp: p.LogPermResp,
			models.LogCategoryCommit:   p.LogCommit,
			models.LogCategoryClaim:    p.LogClaim,
			models.LogCategoryRelease:  p.LogRelease,
			models.LogCategoryDefault:  p.MidGrey,
		},

		// ── Inline styles ───────────────────────────────────────────
		DialogShadowStyle: lipgloss.NewStyle().Background(p.DeepGrey),
		SeparatorStyle:    lipgloss.NewStyle().Foreground(p.Muted),
	}
}
