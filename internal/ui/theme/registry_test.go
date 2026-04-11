package theme

import (
	"strings"
	"testing"
)

// TestInitTanSetsCurrent verifies that Init("tan") populates Current.
func TestInitTanSetsCurrent(t *testing.T) {
	Current = nil
	if err := Init("tan"); err != nil {
		t.Fatalf("Init(\"tan\") returned error: %v", err)
	}
	if Current == nil {
		t.Fatal("Init(\"tan\") did not set Current")
	}
}

// TestInitDarkSetsCurrent verifies that Init("dark") populates Current
// (placeholder returns Tan for now).
func TestInitDarkSetsCurrent(t *testing.T) {
	Current = nil
	if err := Init("dark"); err != nil {
		t.Fatalf("Init(\"dark\") returned error: %v", err)
	}
	if Current == nil {
		t.Fatal("Init(\"dark\") did not set Current")
	}
}

// TestInitLightSetsCurrent verifies that Init("light") populates Current
// (placeholder returns Tan for now).
func TestInitLightSetsCurrent(t *testing.T) {
	Current = nil
	if err := Init("light"); err != nil {
		t.Fatalf("Init(\"light\") returned error: %v", err)
	}
	if Current == nil {
		t.Fatal("Init(\"light\") did not set Current")
	}
}

// TestInitInvalidReturnsError verifies that Init rejects unknown theme names
// with a helpful error message.
func TestInitInvalidReturnsError(t *testing.T) {
	Current = nil
	err := Init("neon")
	if err == nil {
		t.Fatal("Init(\"neon\") should return an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "neon") {
		t.Errorf("error should mention the invalid name, got: %s", msg)
	}
	if !strings.Contains(msg, "tan") || !strings.Contains(msg, "dark") || !strings.Contains(msg, "light") {
		t.Errorf("error should list valid themes, got: %s", msg)
	}
	if Current != nil {
		t.Error("Current should remain nil after failed Init")
	}
}

// TestInitEmptyStringReturnsError verifies that empty string is rejected.
func TestInitEmptyStringReturnsError(t *testing.T) {
	Current = nil
	err := Init("")
	if err == nil {
		t.Fatal("Init(\"\") should return an error")
	}
}
