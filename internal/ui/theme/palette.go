package theme

import "github.com/charmbracelet/lipgloss"

// Palette holds pure colour values for a theme variant. It contains no
// lipgloss imports beyond Color — all structural style assembly lives in
// buildTheme. Adding a new theme means defining a Palette constructor
// with zero duplicated layout logic.
type Palette struct {
	// Semantic colours.
	Primary   lipgloss.Color
	Accent    lipgloss.Color
	Success   lipgloss.Color
	Warning   lipgloss.Color
	Danger    lipgloss.Color
	Muted     lipgloss.Color
	OnPrimary lipgloss.Color

	// Grey scale (light to dark).
	StrongFg   lipgloss.Color // high-emphasis foreground (button text, new-line indicator)
	LightGrey  lipgloss.Color
	MidGrey    lipgloss.Color
	DimGrey    lipgloss.Color
	SubtleGrey lipgloss.Color
	DuskyGrey  lipgloss.Color
	DarkGrey   lipgloss.Color
	DeepGrey   lipgloss.Color

	// Panel overlay colours (editor-waiting, notifications).
	PanelBg lipgloss.Color
	PanelFg lipgloss.Color

	// Borders.
	BorderBlue lipgloss.Color

	// Worker status.
	WorkerAwaiting lipgloss.Color
	WorkerBusy     lipgloss.Color
	WorkerPaused   lipgloss.Color

	// Log message categories.
	LogError    lipgloss.Color
	LogPermReq  lipgloss.Color
	LogPermResp lipgloss.Color
	LogCommit   lipgloss.Color
	LogClaim    lipgloss.Color
	LogRelease  lipgloss.Color
	LogWorker   lipgloss.Color

	// Phase badges.
	PhaseImplement lipgloss.Color
	PhaseRefactor  lipgloss.Color
	PhaseReview    lipgloss.Color
	PhaseBlocked   lipgloss.Color

	// Timestamp tiers — ordered from most-recent to oldest.  Each palette
	// controls the brightness direction so light themes can go dark→light
	// and dark themes can go bright→dim.
	TimestampTier1 lipgloss.Color // < 1 min
	TimestampTier2 lipgloss.Color // 1–5 min
	TimestampTier3 lipgloss.Color // 5–30 min
	TimestampTier4 lipgloss.Color // > 30 min

	// Diff view.
	DiffHunkHeader lipgloss.Color
	DiffAdded      lipgloss.Color
	DiffRemoved    lipgloss.Color
	DiffDeleted    lipgloss.Color
	DiffRenamed    lipgloss.Color
}
