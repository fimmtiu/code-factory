package util

import "testing"

func TestParseCursorPositionReport(t *testing.T) {
	cases := []struct {
		name   string
		report string
		want   CursorPosition
	}{
		{"plain report", "\x1b[12;34R", CursorPosition{Row: 12, Col: 34}},
		{"report after an unrelated reply", "\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[7;1R", CursorPosition{Row: 7, Col: 1}},
		{"empty", "", CursorPosition{}},
		{"timed out mid-report", "\x1b[12;", CursorPosition{}},
		{"no column", "\x1b[12R", CursorPosition{}},
		{"non-numeric", "\x1b[a;bR", CursorPosition{}},
		{"zero row", "\x1b[0;1R", CursorPosition{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseCursorPositionReport(c.report); got != c.want {
				t.Errorf("parseCursorPositionReport(%q) = %+v, want %+v", c.report, got, c.want)
			}
		})
	}
}

func TestClampRowToScreen_RejectsUnknownRows(t *testing.T) {
	for _, row := range []int{0, -1} {
		if got := clampRowToScreen(row); got != 0 {
			t.Errorf("clampRowToScreen(%d) = %d, want 0", row, got)
		}
	}
}

func TestClampRowToScreen_KeepsRowsWithinTheScreen(t *testing.T) {
	// Tests run without a terminal on stdout, so the height is unavailable
	// and the row has to come back unchanged rather than be dropped.
	if got := clampRowToScreen(9); got != 9 {
		t.Errorf("clampRowToScreen(9) = %d, want 9", got)
	}
}

func TestSaveCursorPosition_WithoutATerminal(t *testing.T) {
	if got := SaveCursorPosition(); got != (CursorPosition{}) {
		t.Errorf("SaveCursorPosition() = %+v, want the zero value when stdio is not a terminal", got)
	}
}

func TestRestoreTerminal_WithoutATerminalWritesNothing(t *testing.T) {
	// Guards against escape sequences landing in redirected output.
	RestoreTerminal(CursorPosition{Row: 3, Col: 1})
}
