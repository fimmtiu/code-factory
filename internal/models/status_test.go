package models

import "testing"

func TestIsValidTicketStatus(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"blocked", true},
		{"open", true},
		{"in-progress", true},
		{"review-ready", true},
		{"in-review", true},
		{"done", true},
		{"", false},
		{"pending", false},
		{"OPEN", false},
		{"In-Progress", false},
		{"closed", false},
	}

	for _, tc := range tests {
		got := IsValidTicketStatus(tc.input)
		if got != tc.want {
			t.Errorf("IsValidTicketStatus(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestIsValidProjectStatus(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"blocked", true},
		{"open", true},
		{"in-progress", true},
		{"done", true},
		{"review-ready", false},
		{"in-review", false},
		{"", false},
		{"pending", false},
		{"OPEN", false},
		{"closed", false},
	}

	for _, tc := range tests {
		got := IsValidProjectStatus(tc.input)
		if got != tc.want {
			t.Errorf("IsValidProjectStatus(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestTicketStatusConstants(t *testing.T) {
	if StatusBlocked != "blocked" {
		t.Errorf("StatusBlocked = %q, want %q", StatusBlocked, "blocked")
	}
	if StatusOpen != "open" {
		t.Errorf("StatusOpen = %q, want %q", StatusOpen, "open")
	}
	if StatusInProgress != "in-progress" {
		t.Errorf("StatusInProgress = %q, want %q", StatusInProgress, "in-progress")
	}
	if StatusReviewReady != "review-ready" {
		t.Errorf("StatusReviewReady = %q, want %q", StatusReviewReady, "review-ready")
	}
	if StatusInReview != "in-review" {
		t.Errorf("StatusInReview = %q, want %q", StatusInReview, "in-review")
	}
	if StatusDone != "done" {
		t.Errorf("StatusDone = %q, want %q", StatusDone, "done")
	}
}

func TestProjectStatusConstants(t *testing.T) {
	if ProjectBlocked != "blocked" {
		t.Errorf("ProjectBlocked = %q, want %q", ProjectBlocked, "blocked")
	}
	if ProjectOpen != "open" {
		t.Errorf("ProjectOpen = %q, want %q", ProjectOpen, "open")
	}
	if ProjectInProgress != "in-progress" {
		t.Errorf("ProjectInProgress = %q, want %q", ProjectInProgress, "in-progress")
	}
	if ProjectDone != "done" {
		t.Errorf("ProjectDone = %q, want %q", ProjectDone, "done")
	}
}
