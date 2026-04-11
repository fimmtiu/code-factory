package theme

import (
	"reflect"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

func TestTanReturnsNonNil(t *testing.T)     { assertThemeNonNil(t, Tan()) }
func TestTanStyleFieldCount(t *testing.T)   { assertStyleFieldCount(t, Tan()) }
func TestTanAccentBorder(t *testing.T)      { assertAccentBorder(t, Tan()) }
func TestTanPhaseBadgeStyles(t *testing.T)  { assertPhaseBadgeStyles(t, Tan()) }
func TestTanLogTimestampStyle(t *testing.T) { assertLogTimestampStyle(t, Tan()) }
func TestTanInheritance(t *testing.T)       { assertInheritance(t, Tan()) }

// TestTanLogCategoryColors verifies that all log categories have the correct
// colour values for the tan theme.
func TestTanLogCategoryColors(t *testing.T) {
	th := Tan()
	assertLogCategoryPresence(t, th)

	wantColours := map[models.LogCategory]lipgloss.Color{
		models.LogCategoryError:    lipgloss.Color("88"),
		models.LogCategoryPermReq:  lipgloss.Color("94"),
		models.LogCategoryPermResp: lipgloss.Color("75"),
		models.LogCategoryCommit:   lipgloss.Color("74"),
		models.LogCategoryClaim:    lipgloss.Color("34"),
		models.LogCategoryRelease:  lipgloss.Color("21"),
		models.LogCategoryDefault:  lipgloss.Color("246"),
	}
	for cat, want := range wantColours {
		got := th.LogCategoryColors[cat]
		if got != want {
			t.Errorf("LogCategoryColors[%q] = %q, want %q", cat, got, want)
		}
	}
}

// TestTanDialogShadowStyle verifies the DialogShadowStyle is set to a
// non-zero value.
func TestTanDialogShadowStyle(t *testing.T) {
	th := Tan()

	if reflect.DeepEqual(th.DialogShadowStyle, lipgloss.Style{}) {
		t.Error("DialogShadowStyle is zero-value")
	}
}
