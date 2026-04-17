package models

import "testing"

func TestIsValidTicketPhase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"blocked", true},
		{"plan", false},
		{"implement", true},
		{"review", true},
		{"respond", false},
		{"refactor", true},
		{"done", true},
		{"", false},
		{"open", false},
		{"working", false},
		{"PLAN", false},
		{"closed", false},
	}

	for _, tc := range tests {
		got := IsValidTicketPhase(tc.input)
		if got != tc.want {
			t.Errorf("IsValidTicketPhase(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestIsValidTicketStatus(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"idle", true},
		{"needs-attention", true},
		{"working", true},
		{"responding", true},
		{"user-review", true},
		{"", false},
		{"open", false},
		{"done", false},
		{"in-progress", false},
		{"IDLE", false},
	}

	for _, tc := range tests {
		got := IsValidTicketStatus(tc.input)
		if got != tc.want {
			t.Errorf("IsValidTicketStatus(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestTicketPhaseConstants(t *testing.T) {
	if PhaseBlocked != "blocked" {
		t.Errorf("PhaseBlocked = %q, want %q", PhaseBlocked, "blocked")
	}
	if PhaseImplement != "implement" {
		t.Errorf("PhaseImplement = %q, want %q", PhaseImplement, "implement")
	}
	if PhaseReview != "review" {
		t.Errorf("PhaseReview = %q, want %q", PhaseReview, "review")
	}
	if PhaseRefactor != "refactor" {
		t.Errorf("PhaseRefactor = %q, want %q", PhaseRefactor, "refactor")
	}
	if PhaseDone != "done" {
		t.Errorf("PhaseDone = %q, want %q", PhaseDone, "done")
	}
}

func TestTicketStatusConstants(t *testing.T) {
	if StatusIdle != "idle" {
		t.Errorf("StatusIdle = %q, want %q", StatusIdle, "idle")
	}
	if StatusNeedsAttention != "needs-attention" {
		t.Errorf("StatusNeedsAttention = %q, want %q", StatusNeedsAttention, "needs-attention")
	}
	if StatusWorking != "working" {
		t.Errorf("StatusWorking = %q, want %q", StatusWorking, "working")
	}
	if StatusResponding != "responding" {
		t.Errorf("StatusResponding = %q, want %q", StatusResponding, "responding")
	}
	if StatusUserReview != "user-review" {
		t.Errorf("StatusUserReview = %q, want %q", StatusUserReview, "user-review")
	}
}
