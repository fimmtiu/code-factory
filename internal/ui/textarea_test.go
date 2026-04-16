package ui

import "testing"

func TestTextArea_SetValue_Empty(t *testing.T) {
	ta := NewTextArea(40, 5)
	ta.SetValue("")
	if got := ta.Value(); got != "" {
		t.Errorf("SetValue(%q): Value() = %q, want %q", "", got, "")
	}
}

func TestTextArea_SetValue_SingleLine(t *testing.T) {
	ta := NewTextArea(40, 5)
	ta.SetValue("hello world")
	if got := ta.Value(); got != "hello world" {
		t.Errorf("SetValue: Value() = %q, want %q", got, "hello world")
	}
}

func TestTextArea_SetValue_MultiLine(t *testing.T) {
	ta := NewTextArea(40, 5)
	ta.SetValue("line one\nline two\nline three")
	if got := ta.Value(); got != "line one\nline two\nline three" {
		t.Errorf("SetValue: Value() = %q, want %q", got, "line one\nline two\nline three")
	}
}

func TestTextArea_SetValue_OverwritesPreviousContent(t *testing.T) {
	ta := NewTextArea(40, 5)
	ta.SetValue("first")
	ta.SetValue("second")
	if got := ta.Value(); got != "second" {
		t.Errorf("SetValue: Value() = %q, want %q", got, "second")
	}
}

func TestTextArea_SetValue_CursorAtEnd(t *testing.T) {
	ta := NewTextArea(40, 5)
	ta.SetValue("abc\ndef")
	// Cursor should be at end of last line.
	if ta.row != 1 {
		t.Errorf("SetValue: row = %d, want 1", ta.row)
	}
	if ta.col != 3 {
		t.Errorf("SetValue: col = %d, want 3", ta.col)
	}
}
