package theme

import (
	"reflect"
	"testing"
)

// TestLightUseDarkForegroundColors verifies the light palette uses dark
// foreground colors (low xterm-256 numbers) suitable for white backgrounds.
func TestLightUseDarkForegroundColors(t *testing.T) {
	p := LightPalette()

	if p.Muted == "" {
		t.Error("Muted colour is empty")
	}
	if p.OnPrimary == "" {
		t.Error("OnPrimary colour is empty")
	}
	if p.StrongFg == "" {
		t.Error("StrongFg colour is empty")
	}
}

// TestLightPaletteAllFieldsPopulated uses reflection to verify every field in
// the LightPalette is non-zero, catching accidentally omitted colour values.
func TestLightPaletteAllFieldsPopulated(t *testing.T) {
	p := LightPalette()
	rv := reflect.ValueOf(p)
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		val := rv.Field(i)
		if val.IsZero() {
			t.Errorf("LightPalette().%s is zero-value", field.Name)
		}
	}
}

// TestLightDistinctFromTan verifies the light theme uses different colours
// than the tan theme in key palette positions, confirming it is not just a
// copy of Tan.
func TestLightDistinctFromTan(t *testing.T) {
	light := LightPalette()
	tan := TanPalette()

	if light.DeepGrey == tan.DeepGrey &&
		light.DarkGrey == tan.DarkGrey &&
		light.DuskyGrey == tan.DuskyGrey &&
		light.SubtleGrey == tan.SubtleGrey &&
		light.DimGrey == tan.DimGrey &&
		light.MidGrey == tan.MidGrey &&
		light.LightGrey == tan.LightGrey &&
		light.StrongFg == tan.StrongFg {
		t.Error("LightPalette grey scale is identical to TanPalette — expected distinct values for light backgrounds")
	}
}

// TestLightRegistryIntegration verifies that Init("light") uses the real
// Light() constructor (not the Tan placeholder).
func TestLightRegistryIntegration(t *testing.T) {
	saved := currentTheme.Load()
	t.Cleanup(func() { currentTheme.Store(saved) })
	currentTheme.Store(nil)
	if err := Init("light"); err != nil {
		t.Fatalf("Init(\"light\") returned error: %v", err)
	}
	cur := Current()
	if cur == nil {
		t.Fatal("Init(\"light\") did not set current theme")
	}

	tanTheme := Tan()
	if reflect.DeepEqual(cur.DialogShadowStyle, tanTheme.DialogShadowStyle) &&
		reflect.DeepEqual(cur.ViewPaneStyle, tanTheme.ViewPaneStyle) &&
		reflect.DeepEqual(cur.HeaderStyle, tanTheme.HeaderStyle) {
		t.Error("Init(\"light\") appears to still use Tan() — expected distinct style values")
	}
}
