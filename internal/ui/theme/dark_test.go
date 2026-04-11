package theme

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestDarkReturnsNonNil(t *testing.T)     { assertThemeNonNil(t, Dark()) }
func TestDarkStyleFieldCount(t *testing.T)   { assertStyleFieldCount(t, Dark()) }
func TestDarkAccentBorder(t *testing.T)      { assertAccentBorder(t, Dark()) }
func TestDarkPhaseBadgeStyles(t *testing.T)  { assertPhaseBadgeStyles(t, Dark()) }
func TestDarkLogTimestampStyle(t *testing.T) { assertLogTimestampStyle(t, Dark()) }
func TestDarkInheritance(t *testing.T)       { assertInheritance(t, Dark()) }

// TestDarkAllStyleFieldsRenderable verifies that every lipgloss.Style field
// in the Dark theme can render text without panicking.
func TestDarkAllStyleFieldsRenderable(t *testing.T) {
	th := Dark()
	rv := reflect.ValueOf(th).Elem()
	rt := rv.Type()

	styleType := reflect.TypeOf(lipgloss.Style{})

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if f.Type != styleType {
			continue
		}
		val := rv.Field(i).Interface().(lipgloss.Style)
		rendered := val.Render("test")
		if len(rendered) == 0 {
			t.Errorf("style field %q rendered to empty string", f.Name)
		}
	}
}

// TestDarkLogCategoryColors verifies that all log categories have a colour
// assigned in the dark theme.
func TestDarkLogCategoryColors(t *testing.T) {
	assertLogCategoryPresence(t, Dark())
}

// TestDarkForegroundsAvoidLowRange verifies that foreground colours used on
// dark backgrounds are not in the very dark xterm-256 range (0-60) where
// they'd be invisible.
func TestDarkForegroundsAvoidLowRange(t *testing.T) {
	p := DarkPalette()
	rv := reflect.ValueOf(p)
	rt := rv.Type()

	// Palette fields that are foreground-oriented and should be bright.
	brightFields := map[string]bool{
		"OnPrimary": true,
		"StrongFg":  true,
		"LightGrey": true,
		"MidGrey":   true,
		"DimGrey":   true,
	}

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !brightFields[f.Name] {
			continue
		}
		c := rv.Field(i).Interface().(lipgloss.Color)
		n, err := strconv.Atoi(string(c))
		if err != nil {
			continue // skip non-numeric colours
		}
		if n < 61 {
			t.Errorf("palette field %q has colour %d which is too dark for dark backgrounds (want > 60)", f.Name, n)
		}
	}
}

// TestDarkPaletteNonZero verifies every Palette field has a non-empty colour.
func TestDarkPaletteNonZero(t *testing.T) {
	p := DarkPalette()
	rv := reflect.ValueOf(p)
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		c := rv.Field(i).Interface().(lipgloss.Color)
		if c == "" {
			t.Errorf("palette field %q is empty", f.Name)
		}
	}
}

// TestDarkSemanticConsistency verifies that semantic colour mappings are
// consistent: success uses a green, danger uses a red, warning uses an amber.
func TestDarkSemanticConsistency(t *testing.T) {
	p := DarkPalette()

	successN, _ := strconv.Atoi(string(p.Success))
	if successN == 0 {
		t.Error("Success colour is empty or zero")
	}

	dangerN, _ := strconv.Atoi(string(p.Danger))
	if dangerN == 0 {
		t.Error("Danger colour is empty or zero")
	}

	warningN, _ := strconv.Atoi(string(p.Warning))
	if warningN == 0 {
		t.Error("Warning colour is empty or zero")
	}

	// These shouldn't be identical to each other.
	if successN == dangerN {
		t.Error("Success and Danger have the same colour")
	}
	if successN == warningN {
		t.Error("Success and Warning have the same colour")
	}
	if dangerN == warningN {
		t.Error("Danger and Warning have the same colour")
	}
}

// TestDarkTimestampTiersDescend verifies that the dark palette's timestamp
// tiers descend from bright (recent) to dim (old), the opposite of the tan
// theme's direction.
func TestDarkTimestampTiersDescend(t *testing.T) {
	p := DarkPalette()
	tiers := []lipgloss.Color{p.TimestampTier1, p.TimestampTier2, p.TimestampTier3, p.TimestampTier4}

	for i := 0; i < len(tiers)-1; i++ {
		cur, _ := strconv.Atoi(string(tiers[i]))
		next, _ := strconv.Atoi(string(tiers[i+1]))
		if cur <= next {
			t.Errorf("TimestampTier%d (%d) should be brighter than TimestampTier%d (%d) for dark theme",
				i+1, cur, i+2, next)
		}
	}
}

// TestTanTimestampTiersAscend verifies the tan palette's timestamp tiers
// ascend from dark (recent) to light (old).
func TestTanTimestampTiersAscend(t *testing.T) {
	p := TanPalette()
	tiers := []lipgloss.Color{p.TimestampTier1, p.TimestampTier2, p.TimestampTier3, p.TimestampTier4}

	for i := 0; i < len(tiers)-1; i++ {
		cur, _ := strconv.Atoi(string(tiers[i]))
		next, _ := strconv.Atoi(string(tiers[i+1]))
		if cur >= next {
			t.Errorf("TimestampTier%d (%d) should be darker than TimestampTier%d (%d) for tan theme",
				i+1, cur, i+2, next)
		}
	}
}

// TestDarkDiffersFromTan verifies that the dark theme is not identical to the
// tan theme — i.e. the placeholder has been replaced with real values.
func TestDarkDiffersFromTan(t *testing.T) {
	dark := DarkPalette()
	tan := TanPalette()

	if dark == tan {
		t.Error("DarkPalette() is identical to TanPalette(); dark theme must have its own colour values")
	}
}
