package ui

import "testing"

// ── phaseFromLogfile tests ───────────────────────────────────────────────────

func TestPhaseFromLogfile_Standard(t *testing.T) {
	got := phaseFromLogfile("/some/repo/.code-factory/proj/ticket/implement.log")
	if got != "implement" {
		t.Errorf("phaseFromLogfile: got %q, want %q", got, "implement")
	}
}

func TestPhaseFromLogfile_Review(t *testing.T) {
	got := phaseFromLogfile("/repo/.code-factory/my-proj/my-ticket/review.log")
	if got != "review" {
		t.Errorf("phaseFromLogfile: got %q, want %q", got, "review")
	}
}

func TestPhaseFromLogfile_Numbered(t *testing.T) {
	// Numbered logfiles like implement.log.1 should still return "implement".
	got := phaseFromLogfile("/repo/.code-factory/proj/ticket/implement.log.1")
	if got != "implement" {
		t.Errorf("phaseFromLogfile numbered: got %q, want %q", got, "implement")
	}
}

func TestPhaseFromLogfile_Respond(t *testing.T) {
	got := phaseFromLogfile("/repo/.code-factory/proj/ticket/respond.log")
	if got != "respond" {
		t.Errorf("phaseFromLogfile: got %q, want %q", got, "respond")
	}
}

func TestPhaseFromLogfile_Empty(t *testing.T) {
	got := phaseFromLogfile("")
	if got != "" {
		t.Errorf("phaseFromLogfile empty: got %q, want %q", got, "")
	}
}

func TestPhaseFromLogfile_NoTicketsDir(t *testing.T) {
	got := phaseFromLogfile("/some/random/path/implement.log")
	if got != "" {
		t.Errorf("phaseFromLogfile no .code-factory: got %q, want %q", got, "")
	}
}

func TestPhaseFromLogfile_TooShortPath(t *testing.T) {
	got := phaseFromLogfile(".code-factory/proj/implement.log")
	if got != "" {
		t.Errorf("phaseFromLogfile too short: got %q, want %q", got, "")
	}
}

// ── identifierFromLogfile tests (existing function, verify preservation) ─────

func TestIdentifierFromLogfile_Standard(t *testing.T) {
	got := identifierFromLogfile("/repo/.code-factory/proj/ticket/implement.log")
	if got != "proj/ticket" {
		t.Errorf("identifierFromLogfile: got %q, want %q", got, "proj/ticket")
	}
}

func TestIdentifierFromLogfile_Empty(t *testing.T) {
	got := identifierFromLogfile("")
	if got != "" {
		t.Errorf("identifierFromLogfile empty: got %q, want %q", got, "")
	}
}
