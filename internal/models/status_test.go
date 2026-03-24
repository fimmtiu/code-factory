package models

import "testing"

func TestIsValidTicketPhase(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"blocked", true},
		{"plan", true},
		{"implement", true},
		{"review", true},
		{"respond", true},
		{"refactor", true},
		{"done", true},
		{"", false},
		{"open", false},
		{"in-progress", false},
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
		{"in-progress", true},
		{"user-review", true},
		{"", false},
		{"open", false},
		{"done", false},
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
	if PhasePlan != "plan" {
		t.Errorf("PhasePlan = %q, want %q", PhasePlan, "plan")
	}
	if PhaseImplement != "implement" {
		t.Errorf("PhaseImplement = %q, want %q", PhaseImplement, "implement")
	}
	if PhaseReview != "review" {
		t.Errorf("PhaseReview = %q, want %q", PhaseReview, "review")
	}
	if PhaseRespond != "respond" {
		t.Errorf("PhaseRespond = %q, want %q", PhaseRespond, "respond")
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
	if StatusInProgress != "in-progress" {
		t.Errorf("StatusInProgress = %q, want %q", StatusInProgress, "in-progress")
	}
	if StatusUserReview != "user-review" {
		t.Errorf("StatusUserReview = %q, want %q", StatusUserReview, "user-review")
	}
}
