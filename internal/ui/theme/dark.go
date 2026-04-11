package theme

import "github.com/charmbracelet/lipgloss"

// DarkPalette returns the colour palette for the "dark" theme, optimised for
// readability on black/dark terminal backgrounds. Foreground colours use high
// xterm-256 numbers (150-255) for visibility; backgrounds use subtle dark
// tints that still contrast with pure black.
func DarkPalette() Palette {
	return Palette{
		// Semantic colours — bright variants visible on black.
		Primary:   lipgloss.Color("63"),  // brighter purple
		Accent:    lipgloss.Color("111"), // bright blue-purple
		Success:   lipgloss.Color("78"),  // bright green
		Warning:   lipgloss.Color("214"), // bright amber
		Danger:    lipgloss.Color("203"), // bright red
		Muted:     lipgloss.Color("245"), // medium grey, visible on black
		OnPrimary: lipgloss.Color("255"), // bright white for text on coloured bg

		// Grey scale (bright to dim — all visible on black).
		StrongFg:   lipgloss.Color("255"),
		LightGrey:  lipgloss.Color("252"),
		MidGrey:    lipgloss.Color("249"),
		DimGrey:    lipgloss.Color("246"),
		SubtleGrey: lipgloss.Color("243"),
		DuskyGrey:  lipgloss.Color("240"),
		DarkGrey:   lipgloss.Color("237"),
		DeepGrey:   lipgloss.Color("235"),

		// Panel overlay colours.
		PanelBg: lipgloss.Color("236"), // dark charcoal, distinct from pure black
		PanelFg: lipgloss.Color("252"), // near-white for panel text

		// Borders.
		BorderBlue: lipgloss.Color("75"), // bright blue, visible on dark

		// Worker status — bright colours for readability.
		WorkerAwaiting: lipgloss.Color("204"), // bright pink-red
		WorkerBusy:     lipgloss.Color("77"),  // bright green
		WorkerPaused:   lipgloss.Color("220"), // bright yellow

		// Log message categories — brighter than tan for dark bg.
		LogError:    lipgloss.Color("203"), // bright red
		LogPermReq:  lipgloss.Color("177"), // bright purple
		LogPermResp: lipgloss.Color("117"), // bright cyan-blue
		LogCommit:   lipgloss.Color("116"), // bright teal
		LogClaim:    lipgloss.Color("114"), // bright green
		LogRelease:  lipgloss.Color("69"),  // bright blue

		// Phase badges — bright enough for dark bg.
		PhaseImplement: lipgloss.Color("80"),  // bright cyan
		PhaseRefactor:  lipgloss.Color("214"), // bright orange
		PhaseReview:    lipgloss.Color("111"), // bright blue
		PhaseRespond:   lipgloss.Color("177"), // bright purple
		PhaseBlocked:   lipgloss.Color("203"), // bright red

		// Timestamp tiers (recent = bright, old = dim — visible on dark bg).
		TimestampTier1: lipgloss.Color("252"), // < 1 min  (brightest, most prominent on dark bg)
		TimestampTier2: lipgloss.Color("246"), // 1–5 min
		TimestampTier3: lipgloss.Color("243"), // 5–30 min
		TimestampTier4: lipgloss.Color("240"), // > 30 min (dimmest, least prominent)

		// Diff view — dark-tinted backgrounds that contrast with black.
		DiffHunkHeader: lipgloss.Color("24"),  // dark blue tint
		DiffAdded:      lipgloss.Color("22"),  // dark green tint
		DiffRemoved:    lipgloss.Color("52"),  // dark red tint
		DiffDeleted:    lipgloss.Color("167"), // bright red-orange fg
		DiffRenamed:    lipgloss.Color("75"),  // bright blue fg
	}
}

// Dark returns a fully assembled Theme using the dark colour palette,
// optimised for black/dark terminal backgrounds.
func Dark() *Theme {
	return buildTheme(DarkPalette())
}
