package models

type TicketPhase string

const (
	PhaseBlocked   TicketPhase = "blocked"
	PhaseImplement TicketPhase = "implement"
	PhaseReview    TicketPhase = "review"
	PhaseRefactor  TicketPhase = "refactor"
	PhaseDone      TicketPhase = "done"
)

type TicketStatus string

const (
	StatusIdle           TicketStatus = "idle"
	StatusNeedsAttention TicketStatus = "needs-attention"
	StatusWorking        TicketStatus = "working"
	StatusResponding     TicketStatus = "responding"
	StatusUserReview     TicketStatus = "user-review"
)

func IsValidTicketPhase(s string) bool {
	switch TicketPhase(s) {
	case PhaseBlocked, PhaseImplement, PhaseReview, PhaseRefactor, PhaseDone:
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
	switch TicketStatus(s) {
	case StatusIdle, StatusNeedsAttention, StatusWorking, StatusResponding, StatusUserReview:
		return true
	}
	return false
}
