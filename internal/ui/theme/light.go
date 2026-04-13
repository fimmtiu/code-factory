package theme

import "github.com/charmbracelet/lipgloss"

// LightPalette returns the colour palette for the "light" theme, optimised for
// white / light terminal backgrounds. Foreground colours use dark xterm-256
// values (0-100 range) for readability; backgrounds use subtle tints that
// contrast with white.
func LightPalette() Palette {
	return Palette{
		// Semantic colours — dark enough to read on white.
		Primary:   lipgloss.Color("25"),  // deep blue
		Accent:    lipgloss.Color("30"),  // teal
		Success:   lipgloss.Color("28"),  // dark green
		Warning:   lipgloss.Color("130"), // dark amber/brown
		Danger:    lipgloss.Color("124"), // dark red
		Muted:     lipgloss.Color("244"), // medium grey — visible on white
		OnPrimary: lipgloss.Color("255"), // bright white text on dark bg

		// Grey scale — inverted from tan: darkest values are "freshest",
		// lightest values are "most faded" but still readable on white.
		StrongFg:   lipgloss.Color("16"),  // black — high-emphasis foreground
		LightGrey:  lipgloss.Color("252"), // very light grey (button bg)
		MidGrey:    lipgloss.Color("247"), // mid grey
		DimGrey:    lipgloss.Color("102"), // dark-ish grey
		SubtleGrey: lipgloss.Color("243"), // subtle grey
		DuskyGrey:  lipgloss.Color("240"), // dusky grey
		DarkGrey:   lipgloss.Color("253"), // light grey for progress empty
		DeepGrey:   lipgloss.Color("236"), // deep grey for shadows

		// Panel overlay colours — dark bg + light fg for readability on light terminals.
		PanelBg: lipgloss.Color("238"), // dark grey background
		PanelFg: lipgloss.Color("255"), // bright white text

		// Borders.
		BorderBlue: lipgloss.Color("25"), // dark blue, visible on white

		// Worker status — saturated darks.
		WorkerAwaiting: lipgloss.Color("160"), // dark red
		WorkerBusy:     lipgloss.Color("22"),  // dark green
		WorkerPaused:   lipgloss.Color("94"),  // dark yellow/brown

		// Log message categories — dark/mid colours visible on white.
		LogError:    lipgloss.Color("124"), // dark red
		LogPermReq:  lipgloss.Color("94"),  // dark brown
		LogPermResp: lipgloss.Color("62"),  // medium purple
		LogCommit:   lipgloss.Color("30"),  // teal
		LogClaim:    lipgloss.Color("28"),  // dark green
		LogRelease:  lipgloss.Color("19"),  // dark blue

		// Phase badges — dark enough for white backgrounds.
		PhaseImplement: lipgloss.Color("30"),  // teal
		PhaseRefactor:  lipgloss.Color("130"), // dark amber
		PhaseReview:    lipgloss.Color("62"),  // medium purple
		PhaseRespond:   lipgloss.Color("91"),  // dark magenta
		PhaseBlocked:   lipgloss.Color("124"), // dark red

		// Diff view — subtle pastel backgrounds that show on white.
		DiffHunkHeader: lipgloss.Color("189"), // light lavender
		DiffAdded:      lipgloss.Color("194"), // light green tint
		DiffRemoved:    lipgloss.Color("224"), // light pink tint
		DiffDeleted:    lipgloss.Color("88"),  // dark red fg
		DiffRenamed:    lipgloss.Color("25"),  // dark blue fg
	}
}

// Light returns a fully assembled Theme using the light colour palette,
// optimised for readability on white / light terminal backgrounds.
func Light() *Theme {
	return buildTheme(LightPalette())
}
