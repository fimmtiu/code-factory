package theme

import (
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// Theme captures every semantic style used across the application UI. Views
// reference fields by name instead of hard-coded colour/style variables.
type Theme struct {
	// ── Shared styles ───────────────────────────────────────────────────
	ViewPaneStyle   lipgloss.Style
	EmptyStateStyle lipgloss.Style
	HintKeyStyle    lipgloss.Style
	HintDescStyle   lipgloss.Style

	// ── App chrome ──────────────────────────────────────────────────────
	HeaderStyle      lipgloss.Style
	ActiveTabStyle   lipgloss.Style
	InactiveTabStyle lipgloss.Style
	HelpHintStyle    lipgloss.Style

	// ── Dialog ──────────────────────────────────────────────────────────
	DialogBoxStyle     lipgloss.Style
	DialogTitleStyle   lipgloss.Style
	ButtonNormalStyle  lipgloss.Style
	ButtonFocusedStyle lipgloss.Style
	EditorWaitingStyle lipgloss.Style
	NotifStyle         lipgloss.Style

	// ── Permission dialog ───────────────────────────────────────────────
	PermOptionNormalStyle   lipgloss.Style
	PermOptionSelectedStyle lipgloss.Style

	// ── Quick response ──────────────────────────────────────────────────
	QuickResponseOutputStyle lipgloss.Style
	QuickResponseInputStyle  lipgloss.Style

	// ── Ticket dialog ───────────────────────────────────────────────────
	TdSelectedStyle   lipgloss.Style
	TdDismissedStyle  lipgloss.Style
	TdClosedStyle     lipgloss.Style
	TdSectionStyle    lipgloss.Style
	HintInactiveStyle lipgloss.Style

	// ── Command view ────────────────────────────────────────────────────
	CmdSelectedStyle          lipgloss.Style
	CmdSelectedUnfocusedStyle lipgloss.Style
	CmdNeedsAttentionStyle    lipgloss.Style
	CmdUserReviewStyle        lipgloss.Style
	CmdErrorStyle             lipgloss.Style

	// ── Project view ────────────────────────────────────────────────────
	FocusedBorderStyle   lipgloss.Style
	UnfocusedBorderStyle lipgloss.Style
	StatusPaneStyle      lipgloss.Style
	TreeBlockedStyle     lipgloss.Style
	TreeDoneStyle        lipgloss.Style
	TreeDefaultStyle     lipgloss.Style
	TreeSelectedStyle    lipgloss.Style
	DetailLabelStyle     lipgloss.Style
	ProgressFilledStyle  lipgloss.Style
	ProgressEmptyStyle   lipgloss.Style
	RepoNameStyle        lipgloss.Style

	// ── Worker view ─────────────────────────────────────────────────────
	WorkerIdleStyle     lipgloss.Style
	WorkerAwaitingStyle lipgloss.Style
	WorkerBusyStyle     lipgloss.Style
	WorkerPausedStyle   lipgloss.Style
	WorkerOutputStyle   lipgloss.Style
	WorkerNoOutputStyle lipgloss.Style
	WorkerNewLineStyle  lipgloss.Style

	// ── Log view ────────────────────────────────────────────────────────
	LogSelectedStyle lipgloss.Style
	LogWorkerStyle   lipgloss.Style

	// ── Diff view ───────────────────────────────────────────────────────
	DiffSelectedStyle   lipgloss.Style
	DiffRangeStyle      lipgloss.Style
	DiffLineSelectStyle lipgloss.Style
	DiffSeparatorStyle  lipgloss.Style
	DiffStatusBarStyle  lipgloss.Style
	CommitHashStyle     lipgloss.Style
	DiffLabelBold       lipgloss.Style
	DiffErrorStyle      lipgloss.Style
	DiffStatAddStyle    lipgloss.Style
	DiffStatRemoveStyle lipgloss.Style

	// ── Diff renderer ───────────────────────────────────────────────────
	DiffHunkHeaderStyle lipgloss.Style
	DiffAddedStyle      lipgloss.Style
	DiffRemovedStyle    lipgloss.Style
	DiffFileHeaderStyle lipgloss.Style
	DiffDeletedMsgStyle lipgloss.Style
	DiffRenamedMsgStyle lipgloss.Style

	// ── Inline styles ───────────────────────────────────────────────────

	// DialogShadowStyle renders the shadow block behind dialogs.
	DialogShadowStyle lipgloss.Style

	// SeparatorStyle renders horizontal separator lines (e.g. between workers).
	SeparatorStyle lipgloss.Style

	// ── Special fields ──────────────────────────────────────────────────

	// AccentBorder is a custom DoubleBorder variant with a solid half-block
	// left edge, used for the focused pane border.
	AccentBorder lipgloss.Border

	// PhaseBadgeStyles maps each ticket phase to its badge style.
	PhaseBadgeStyles map[models.TicketPhase]lipgloss.Style

	// LogTimestampStyle returns a style that fades based on log entry age.
	LogTimestampStyle func(time.Duration) lipgloss.Style

	// LogCategoryColors maps semantic log categories to their display colours.
	// The log view classifies messages via logCategory() and looks up the colour here.
	LogCategoryColors map[models.LogCategory]lipgloss.Color
}
