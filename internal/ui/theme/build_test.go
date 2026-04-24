package theme

import (
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// themeConstructors lists every theme variant for shared build-level tests.
var themeConstructors = map[string]func() *Theme{
	"tan":   Tan,
	"light": Light,
}

// TestBuildThemeReturnsNonNil verifies that every theme constructor returns a
// non-nil Theme.
func TestBuildThemeReturnsNonNil(t *testing.T) {
	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			if ctor() == nil {
				t.Fatalf("%s() returned nil", name)
			}
		})
	}
}

// TestBuildThemeStyleFieldCount verifies that every theme populates the
// expected number of lipgloss.Style fields and all special fields.
func TestBuildThemeStyleFieldCount(t *testing.T) {
	const expectedStyleFields = 66

	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			rv := reflect.ValueOf(th).Elem()
			rt := rv.Type()

			styleType := reflect.TypeOf(lipgloss.Style{})
			count := 0
			for i := 0; i < rt.NumField(); i++ {
				if rt.Field(i).Type == styleType {
					count++
				}
			}
			if count != expectedStyleFields {
				t.Errorf("Theme has %d lipgloss.Style fields, want %d", count, expectedStyleFields)
			}

			if th.PhaseBadgeStyles == nil {
				t.Error("PhaseBadgeStyles is nil")
			}
			if th.LogTimestampStyle == nil {
				t.Error("LogTimestampStyle is nil")
			}
			if th.LogCategoryColors == nil {
				t.Error("LogCategoryColors is nil")
			}
			if th.AccentBorder == (lipgloss.Border{}) {
				t.Error("AccentBorder is zero-value")
			}
			if reflect.DeepEqual(th.DialogShadowStyle, lipgloss.Style{}) {
				t.Error("DialogShadowStyle is zero-value")
			}
		})
	}
}

// TestBuildThemeAccentBorder verifies the custom DoubleBorder variant matches
// the shared border shape from buildTheme for every theme.
func TestBuildThemeAccentBorder(t *testing.T) {
	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			b := ctor().AccentBorder
			if b.Left != "▌" {
				t.Errorf("AccentBorder.Left = %q, want %q", b.Left, "▌")
			}
			if b.TopLeft != "╭" {
				t.Errorf("AccentBorder.TopLeft = %q, want %q", b.TopLeft, "╭")
			}
			if b.BottomLeft != "╰" {
				t.Errorf("AccentBorder.BottomLeft = %q, want %q", b.BottomLeft, "╰")
			}
			db := lipgloss.DoubleBorder()
			if b.Top != db.Top {
				t.Errorf("AccentBorder.Top = %q, want DoubleBorder Top %q", b.Top, db.Top)
			}
			if b.Right != db.Right {
				t.Errorf("AccentBorder.Right = %q, want DoubleBorder Right %q", b.Right, db.Right)
			}
		})
	}
}

// TestBuildThemePhaseBadgeStyles verifies all six ticket phases are present in
// every theme.
func TestBuildThemePhaseBadgeStyles(t *testing.T) {
	phases := []models.TicketPhase{
		models.PhaseImplement,
		models.PhaseRefactor,
		models.PhaseReview,
		models.PhaseBlocked,
		models.PhaseDone,
	}

	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			for _, phase := range phases {
				if _, ok := th.PhaseBadgeStyles[phase]; !ok {
					t.Errorf("PhaseBadgeStyles missing key %q", phase)
				}
			}
			if len(th.PhaseBadgeStyles) != len(phases) {
				t.Errorf("PhaseBadgeStyles has %d entries, want %d", len(th.PhaseBadgeStyles), len(phases))
			}
		})
	}
}

// TestBuildThemeLogTimestampStyle verifies the dynamic timestamp styling
// function returns styles for all age brackets without panicking.
func TestBuildThemeLogTimestampStyle(t *testing.T) {
	ages := []time.Duration{
		0,
		30 * time.Second,
		time.Minute,
		3 * time.Minute,
		5 * time.Minute,
		15 * time.Minute,
		30 * time.Minute,
		time.Hour,
		24 * time.Hour,
	}

	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			if th.LogTimestampStyle == nil {
				t.Fatal("LogTimestampStyle is nil")
			}
			for _, age := range ages {
				_ = th.LogTimestampStyle(age).Render("test")
			}
		})
	}
}

// TestBuildThemeLogCategoryColors verifies that all log categories have a
// colour assigned in every theme.
func TestBuildThemeLogCategoryColors(t *testing.T) {
	categories := []models.LogCategory{
		models.LogCategoryError,
		models.LogCategoryPermReq,
		models.LogCategoryPermResp,
		models.LogCategoryCommit,
		models.LogCategoryClaim,
		models.LogCategoryRelease,
		models.LogCategoryDefault,
	}

	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			if th.LogCategoryColors == nil {
				t.Fatal("LogCategoryColors is nil")
			}
			for _, cat := range categories {
				if _, ok := th.LogCategoryColors[cat]; !ok {
					t.Errorf("LogCategoryColors missing key %q", cat)
				}
			}
			if len(th.LogCategoryColors) != len(categories) {
				t.Errorf("LogCategoryColors has %d entries, want %d", len(th.LogCategoryColors), len(categories))
			}
		})
	}
}

// TestBuildThemeInheritance verifies that derived styles inherit from their
// base styles and render without panicking in every theme.
func TestBuildThemeInheritance(t *testing.T) {
	for name, ctor := range themeConstructors {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			if rendered := th.ActiveTabStyle.Render("test"); rendered == "" {
				t.Error("ActiveTabStyle.Render returned empty string")
			}
			if rendered := th.WorkerIdleStyle.Render("idle"); rendered == "" {
				t.Error("WorkerIdleStyle.Render returned empty string")
			}
		})
	}
}
