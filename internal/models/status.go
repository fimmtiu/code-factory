package models

type TicketStatus = string

const (
	StatusBlocked     TicketStatus = "blocked"
	StatusOpen        TicketStatus = "open"
	StatusInProgress  TicketStatus = "in-progress"
	StatusReviewReady TicketStatus = "review-ready"
	StatusInReview    TicketStatus = "in-review"
	StatusDone        TicketStatus = "done"
)

type ProjectStatus = string

const (
	ProjectBlocked    ProjectStatus = "blocked"
	ProjectOpen       ProjectStatus = "open"
	ProjectInProgress ProjectStatus = "in-progress"
	ProjectDone       ProjectStatus = "done"
)

func IsValidTicketStatus(s string) bool {
	switch s {
	case StatusBlocked, StatusOpen, StatusInProgress, StatusReviewReady, StatusInReview, StatusDone:
		return true
	}
	return false
}

func IsValidProjectStatus(s string) bool {
	switch s {
	case ProjectBlocked, ProjectOpen, ProjectInProgress, ProjectDone:
		return true
	}
	return false
}
