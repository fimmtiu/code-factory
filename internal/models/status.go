package models

type TicketPhase = string

const (
	PhaseBlocked   TicketPhase = "blocked"
	PhaseImplement TicketPhase = "implement"
	PhaseReview    TicketPhase = "review"
	PhaseRespond   TicketPhase = "respond"
	PhaseRefactor  TicketPhase = "refactor"
	PhaseDone      TicketPhase = "done"
)

type TicketStatus = string

const (
	StatusIdle           TicketStatus = "idle"
	StatusNeedsAttention TicketStatus = "needs-attention"
	StatusInProgress     TicketStatus = "in-progress"
	StatusUserReview     TicketStatus = "user-review"
)

func IsValidTicketPhase(s string) bool {
	switch s {
	case PhaseBlocked, PhaseImplement, PhaseReview, PhaseRespond, PhaseRefactor, PhaseDone:
		return true
	}
	return false
}

const (
	ProjectPhaseOpen = "open"
	ProjectPhaseDone = "done"
)

func IsValidProjectPhase(s string) bool {
	return s == ProjectPhaseOpen || s == ProjectPhaseDone
}

func IsValidTicketStatus(s string) bool {
	switch s {
	case StatusIdle, StatusNeedsAttention, StatusInProgress, StatusUserReview:
		return true
	}
	return false
}
