package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var identifierSegmentRe = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

const (
	ChangeRequestOpen      = "open"
	ChangeRequestClosed    = "closed"
	ChangeRequestDismissed = "dismissed"
)

// ChangeRequest records a requested code change at a specific location.
type ChangeRequest struct {
	ID           string    `json:"id"`
	CommitHash   string    `json:"commit_hash"`
	CodeLocation string    `json:"code_location"`
	Status       string    `json:"status"` // ChangeRequestOpen, ChangeRequestClosed, or ChangeRequestDismissed
	Date         time.Time `json:"date"`
	Author       string    `json:"author"`
	Description  string    `json:"description"`
}

type WorkUnit struct {
	Identifier     string          `json:"identifier"`
	Description    string          `json:"description"`
	Phase          TicketPhase     `json:"phase,omitempty"`
	Status         TicketStatus    `json:"status,omitempty"`
	Dependencies   []string        `json:"dependencies"`
	LastUpdated    time.Time       `json:"last_updated"`
	IsProject      bool            `json:"is_project,omitempty"`
	Parent         string          `json:"parent,omitempty"`
	ClaimedBy      string          `json:"claimed_by,omitempty"`
	ChangeRequests []ChangeRequest `json:"change_requests,omitempty"`
}

func NewTicket(identifier, description string) *WorkUnit {
	return &WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Phase:        PhaseImplement,
		Status:       StatusIdle,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    false,
	}
}

func NewProject(identifier, description string) *WorkUnit {
	return &WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    true,
	}
}

// ParentIdentifierOf returns the parent portion of a slash-separated identifier
// (e.g. "proj/ticket" → "proj", true) and whether one was found.
func ParentIdentifierOf(identifier string) (string, bool) {
	idx := strings.LastIndex(identifier, "/")
	if idx < 0 {
		return "", false
	}
	return identifier[:idx], true
}

func ValidateIdentifier(s string) error {
	if s == "" {
		return fmt.Errorf("identifier must not be empty")
	}
	segments := strings.Split(s, "/")
	for _, seg := range segments {
		if seg == "" {
			return fmt.Errorf("identifier %q has an empty segment", s)
		}
		if !identifierSegmentRe.MatchString(seg) {
			return fmt.Errorf("identifier segment %q is invalid: must match [a-z][a-z0-9-]*", seg)
		}
	}
	return nil
}
