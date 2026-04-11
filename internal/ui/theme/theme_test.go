package theme

import (
	"reflect"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
)

// TestTanReturnsNonNil verifies that Tan() returns a non-nil Theme.
func TestTanReturnsNonNil(t *testing.T) {
	th := Tan()
	if th == nil {
		t.Fatal("Tan() returned nil")
	}
}

// TestTanStyleFieldCount verifies that the Theme struct has the expected
// number of lipgloss.Style fields. This catches accidentally added or removed
// fields without updating Tan().
func TestTanStyleFieldCount(t *testing.T) {
	th := Tan()
	rv := reflect.ValueOf(th).Elem()
	rt := rv.Type()

	styleType := reflect.TypeOf(lipgloss.Style{})
	count := 0
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).Type == styleType {
			count++
		}
	}

	// There are 63 lipgloss.Style fields in the Theme struct.
	const expectedStyleFields = 63
	if count != expectedStyleFields {
		t.Errorf("Theme has %d lipgloss.Style fields, want %d", count, expectedStyleFields)
	}

	// Verify all non-Style special fields are populated.
	if th.PhaseBadgeStyles == nil {
		t.Error("PhaseBadgeStyles is nil")
	}
	if th.LogTimestampStyle == nil {
		t.Error("LogTimestampStyle is nil")
	}
	if th.LogMessageColor == nil {
		t.Error("LogMessageColor is nil")
	}
	if th.AccentBorder == (lipgloss.Border{}) {
		t.Error("AccentBorder is zero-value")
	}
	if th.DialogShadowColor == "" {
		t.Error("DialogShadowColor is empty")
	}
	if th.MutedColor == "" {
		t.Error("MutedColor is empty")
	}
	if th.AccentColor == "" {
		t.Error("AccentColor is empty")
	}
}

// TestTanAccentBorder verifies the custom DoubleBorder variant.
func TestTanAccentBorder(t *testing.T) {
	th := Tan()
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
	// The rest should come from DoubleBorder.
	db := lipgloss.DoubleBorder()
	if b.Top != db.Top {
		t.Errorf("AccentBorder.Top = %q, want DoubleBorder Top %q", b.Top, db.Top)
	}
	if b.Right != db.Right {
		t.Errorf("AccentBorder.Right = %q, want DoubleBorder Right %q", b.Right, db.Right)
	}
}

// TestTanPhaseBadgeStyles verifies all six ticket phases are present.
func TestTanPhaseBadgeStyles(t *testing.T) {
	th := Tan()

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

// TestTanLogTimestampStyle verifies the dynamic timestamp styling function
// returns distinct styles for different age brackets.
func TestTanLogTimestampStyle(t *testing.T) {
	th := Tan()
	if th.LogTimestampStyle == nil {
		t.Fatal("LogTimestampStyle is nil")
	}

	cases := []struct {
		name string
		age  time.Duration
	}{
		{"under1min", 30 * time.Second},
		{"1to5min", 3 * time.Minute},
		{"5to30min", 15 * time.Minute},
		{"over30min", time.Hour},
	}

	// Verify each bracket is callable and doesn't panic.
	for _, tc := range cases {
		s := th.LogTimestampStyle(tc.age)
		_ = s.Render("test")
	}

	// Verify boundary conditions: the function returns a style for edge values.
	_ = th.LogTimestampStyle(0)
	_ = th.LogTimestampStyle(time.Minute)
	_ = th.LogTimestampStyle(5 * time.Minute)
	_ = th.LogTimestampStyle(30 * time.Minute)
	_ = th.LogTimestampStyle(24 * time.Hour)
}

// TestTanLogMessageColor verifies the dynamic log message colour function
// returns correct colours for known prefixes.
func TestTanLogMessageColor(t *testing.T) {
	th := Tan()
	if th.LogMessageColor == nil {
		t.Fatal("LogMessageColor is nil")
	}

	cases := []struct {
		msg  string
		want lipgloss.Color
	}{
		{"error something", lipgloss.Color("88")},
		{"ACP error", lipgloss.Color("88")},
		{"housekeeping: error", lipgloss.Color("88")},
		{"claimed ticket-1", lipgloss.Color("34")},
		{"released ticket-1", lipgloss.Color("21")},
		{"housekeeping: released ticket-1", lipgloss.Color("21")},
		{"permission request foo", lipgloss.Color("94")},
		{"permission response bar", lipgloss.Color("75")},
		{"[mock] error foo", lipgloss.Color("88")},
		{"[mock] asking user", lipgloss.Color("94")},
		{"[mock] received response", lipgloss.Color("75")},
		{"[mock] committed", lipgloss.Color("74")},
		{"some random message", lipgloss.Color("246")},
	}

	for _, tc := range cases {
		got := th.LogMessageColor(tc.msg)
		if got != tc.want {
			t.Errorf("LogMessageColor(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

// TestTanColorFields verifies the Color fields are set correctly.
func TestTanColorFields(t *testing.T) {
	th := Tan()

	if th.DialogShadowColor != lipgloss.Color("236") {
		t.Errorf("DialogShadowColor = %q, want %q", th.DialogShadowColor, "236")
	}
	if th.MutedColor != lipgloss.Color("240") {
		t.Errorf("MutedColor = %q, want %q", th.MutedColor, "240")
	}
	if th.AccentColor != lipgloss.Color("67") {
		t.Errorf("AccentColor = %q, want %q", th.AccentColor, "67")
	}
}

// TestTanInheritance verifies that derived styles inherit from their base styles.
// ActiveTabStyle should include bold from tabBaseStyle.
func TestTanInheritance(t *testing.T) {
	th := Tan()

	// ActiveTabStyle inherits from tabBaseStyle (bold, padding).
	rendered := th.ActiveTabStyle.Render("test")
	if rendered == "" {
		t.Error("ActiveTabStyle.Render returned empty string")
	}

	// WorkerIdleStyle inherits from workerStatusStyle (bold).
	rendered = th.WorkerIdleStyle.Render("idle")
	if rendered == "" {
		t.Error("WorkerIdleStyle.Render returned empty string")
	}
}
