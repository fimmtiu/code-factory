package theme

import "github.com/charmbracelet/lipgloss"

// TanPalette returns the colour palette for the "tan" theme, reproducing the
// original hard-coded colours from internal/ui/styles.go.
func TanPalette() Palette {
	return Palette{
		// Semantic colours.
		Primary:   lipgloss.Color("62"),
		Accent:    lipgloss.Color("67"),
		Success:   lipgloss.Color("28"),
		Warning:   lipgloss.Color("166"),
		Danger:    lipgloss.Color("196"),
		Muted:     lipgloss.Color("240"),
		OnPrimary: lipgloss.Color("230"),

		// Grey scale (light to dark).
		StrongFg:   lipgloss.Color("255"),
		LightGrey:  lipgloss.Color("250"),
		MidGrey:    lipgloss.Color("246"),
		DimGrey:    lipgloss.Color("245"),
		SubtleGrey: lipgloss.Color("242"),
		DuskyGrey:  lipgloss.Color("239"),
		DarkGrey:   lipgloss.Color("238"),
		DeepGrey:   lipgloss.Color("236"),

		// Panel overlay colours.
		PanelBg: lipgloss.Color("238"),
		PanelFg: lipgloss.Color("230"),

		// Borders.
		BorderBlue: lipgloss.Color("12"),

		// Worker status.
		WorkerAwaiting: lipgloss.Color("9"),
		WorkerBusy:     lipgloss.Color("22"),
		WorkerPaused:   lipgloss.Color("3"),

		// Log message categories.
		LogError:    lipgloss.Color("88"),
		LogPermReq:  lipgloss.Color("94"),
		LogPermResp: lipgloss.Color("75"),
		LogCommit:   lipgloss.Color("74"),
		LogClaim:    lipgloss.Color("34"),
		LogRelease:  lipgloss.Color("21"),

		// Phase badges.
		PhaseImplement: lipgloss.Color("37"),
		PhaseRefactor:  lipgloss.Color("166"),
		PhaseReview:    lipgloss.Color("69"),
		PhaseRespond:   lipgloss.Color("135"),
		PhaseBlocked:   lipgloss.Color("124"),

		// Timestamp tiers (recent = dark, old = light — visible on tan bg).
		TimestampTier1: lipgloss.Color("236"), // < 1 min  (darkest, most prominent on light bg)
		TimestampTier2: lipgloss.Color("239"), // 1–5 min
		TimestampTier3: lipgloss.Color("242"), // 5–30 min
		TimestampTier4: lipgloss.Color("245"), // > 30 min (lightest, least prominent)

		// Diff view.
		DiffHunkHeader: lipgloss.Color("159"),
		DiffAdded:      lipgloss.Color("194"),
		DiffRemoved:    lipgloss.Color("224"),
		DiffDeleted:    lipgloss.Color("52"),
		DiffRenamed:    lipgloss.Color("18"),
	}
}

// Tan returns a fully assembled Theme using the tan colour palette.
func Tan() *Theme {
	return buildTheme(TanPalette())
}
