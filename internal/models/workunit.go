package models

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var identifierSegmentRe = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$|^[a-z]$`)

type WorkUnit struct {
	Identifier   string    `json:"identifier"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Dependencies []string  `json:"dependencies"`
	LastUpdated  time.Time `json:"last_updated"`
	IsProject    bool      `json:"-"`
	Parent       string    `json:"parent,omitempty"`
}

func NewTicket(identifier, description string) *WorkUnit {
	return &WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Status:       StatusOpen,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    false,
	}
}

func NewProject(identifier, description string) *WorkUnit {
	return &WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Status:       ProjectOpen,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
		IsProject:    true,
	}
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
