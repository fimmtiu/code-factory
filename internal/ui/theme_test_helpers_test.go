package ui

import (
	"testing"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// useTanTheme sets theme.Current() to a fresh Tan() for the duration of t,
// restoring the original theme on cleanup.
func useTanTheme(t *testing.T) {
	t.Helper()
	saved := theme.Current()
	t.Cleanup(func() { theme.SetCurrent(saved) })
	theme.SetCurrent(theme.Tan())
}

// assertThemeChangesOutput verifies that renderFunc produces different output
// when using a custom theme (set up by setupTheme) vs the default Tan theme.
// This is the standard pattern for testing that a render path actually reads
// from theme.Current() rather than hard-coding styles.
//
// setupTheme is called to install a custom theme (typically with structurally
// distinctive styles like extra padding) and register a t.Cleanup to restore
// the original. renderFunc should return the rendered string for whatever
// component is under test.
func assertThemeChangesOutput(t *testing.T, setupTheme func(t *testing.T), renderFunc func() string) {
	t.Helper()

	// Render with the default Tan theme.
	original := theme.Current()
	theme.SetCurrent(theme.Tan())
	defaultOutput := renderFunc()
	theme.SetCurrent(original)

	// Render with the custom theme.
	setupTheme(t)
	customOutput := renderFunc()

	if defaultOutput == customOutput {
		t.Error("expected render output to differ between default and custom themes")
	}
}
