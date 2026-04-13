package theme

import (
	"strings"
	"testing"
)

func TestDefaultThemeIsNonNil(t *testing.T) {
	// Current() should be usable even before Init is called.
	if Current() == nil {
		t.Fatal("Current() should default to a non-nil theme")
	}
}

func TestInitValidThemes(t *testing.T) {
	original := Current()
	t.Cleanup(func() { SetCurrent(original) })

	for _, name := range []string{"tan", "dark", "light"} {
		t.Run(name, func(t *testing.T) {
			currentTheme.Store(nil)
			t.Cleanup(func() { currentTheme.Store(nil) })
			if err := Init(name); err != nil {
				t.Fatalf("Init(%q) returned error: %v", name, err)
			}
			if Current() == nil {
				t.Fatalf("Init(%q) did not set current theme", name)
			}
		})
	}
}

func TestInitEmptyNameDefaultsToTan(t *testing.T) {
	original := Current()
	t.Cleanup(func() { SetCurrent(original) })

	currentTheme.Store(nil)
	if err := Init(""); err != nil {
		t.Fatalf("Init(\"\") returned error: %v", err)
	}
	if Current() == nil {
		t.Fatal("Init(\"\") did not set current theme")
	}
}

func TestInitInvalidThemes(t *testing.T) {
	for _, name := range []string{"neon", "retro", "matrix"} {
		t.Run(name, func(t *testing.T) {
			before := Current()
			err := Init(name)
			if err == nil {
				t.Fatalf("Init(%q) should return an error", name)
			}
			if Current() != before {
				t.Errorf("current theme should remain unchanged after failed Init(%q)", name)
			}
		})
	}
}

func TestInitInvalidErrorMessage(t *testing.T) {
	err := Init("neon")
	if err == nil {
		t.Fatal("Init(\"neon\") should return an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "neon") {
		t.Errorf("error should mention the invalid name, got: %s", msg)
	}
	for name := range themes {
		if !strings.Contains(msg, name) {
			t.Errorf("error should list valid theme %q, got: %s", name, msg)
		}
	}
}
