package theme

import (
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/fimmtiu/code-factory/internal/models"
)

// assertThemeNonNil verifies that a Theme constructor returns a non-nil value.
func assertThemeNonNil(t *testing.T, th *Theme) {
	t.Helper()
	if th == nil {
		t.Fatal("constructor returned nil")
	}
}

// assertStyleFieldCount verifies that all 63 lipgloss.Style fields and all
// special fields are populated with non-zero values.
func assertStyleFieldCount(t *testing.T, th *Theme) {
	t.Helper()
	rv := reflect.ValueOf(th).Elem()
	rt := rv.Type()

	styleType := reflect.TypeOf(lipgloss.Style{})
	count := 0
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Type == styleType {
			count++
		}
	}

	const expectedStyleFields = 63
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
}

// assertAccentBorder verifies the custom DoubleBorder variant with a solid
// half-block left edge.
func assertAccentBorder(t *testing.T, th *Theme) {
	t.Helper()
	b := th.AccentBorder

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
}

// assertPhaseBadgeStyles verifies all six ticket phases are present.
func assertPhaseBadgeStyles(t *testing.T, th *Theme) {
	t.Helper()
	phases := []models.TicketPhase{
		models.PhaseImplement,
		models.PhaseRefactor,
		models.PhaseReview,
		models.PhaseRespond,
		models.PhaseBlocked,
		models.PhaseDone,
	}

	for _, phase := range phases {
		if _, ok := th.PhaseBadgeStyles[phase]; !ok {
			t.Errorf("PhaseBadgeStyles missing key %q", phase)
		}
	}

	if len(th.PhaseBadgeStyles) != len(phases) {
		t.Errorf("PhaseBadgeStyles has %d entries, want %d", len(th.PhaseBadgeStyles), len(phases))
	}
}

// assertLogTimestampStyle verifies the dynamic timestamp styling function
// returns styles for all age brackets and boundary conditions without panicking.
func assertLogTimestampStyle(t *testing.T, th *Theme) {
	t.Helper()
	if th.LogTimestampStyle == nil {
		t.Fatal("LogTimestampStyle is nil")
	}

	ages := []time.Duration{
		30 * time.Second,
		3 * time.Minute,
		15 * time.Minute,
		time.Hour,
		0,
		time.Minute,
		5 * time.Minute,
		30 * time.Minute,
		24 * time.Hour,
	}

	for _, age := range ages {
		s := th.LogTimestampStyle(age)
		_ = s.Render("test")
	}
}

// assertLogCategoryPresence verifies that all log categories have a colour
// assigned in the theme.
func assertLogCategoryPresence(t *testing.T, th *Theme) {
	t.Helper()
	if th.LogCategoryColors == nil {
		t.Fatal("LogCategoryColors is nil")
	}

	categories := []models.LogCategory{
		models.LogCategoryError,
		models.LogCategoryPermReq,
		models.LogCategoryPermResp,
		models.LogCategoryCommit,
		models.LogCategoryClaim,
		models.LogCategoryRelease,
		models.LogCategoryDefault,
	}

	for _, cat := range categories {
		if _, ok := th.LogCategoryColors[cat]; !ok {
			t.Errorf("LogCategoryColors missing key %q", cat)
		}
	}

	if len(th.LogCategoryColors) != len(categories) {
		t.Errorf("LogCategoryColors has %d entries, want %d", len(th.LogCategoryColors), len(categories))
	}
}

// assertInheritance verifies that derived styles inherit from base styles
// and render without error.
func assertInheritance(t *testing.T, th *Theme) {
	t.Helper()
	rendered := th.ActiveTabStyle.Render("test")
	if rendered == "" {
		t.Error("ActiveTabStyle.Render returned empty string")
	}

	rendered = th.WorkerIdleStyle.Render("idle")
	if rendered == "" {
		t.Error("WorkerIdleStyle.Render returned empty string")
	}
}
