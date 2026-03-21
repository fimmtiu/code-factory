package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewTicket(t *testing.T) {
	wu := NewTicket("fix-bug", "Fix the rounding bug")
	if wu.Identifier != "fix-bug" {
		t.Errorf("Identifier = %q, want %q", wu.Identifier, "fix-bug")
	}
	if wu.Description != "Fix the rounding bug" {
		t.Errorf("Description = %q, want %q", wu.Description, "Fix the rounding bug")
	}
	if wu.Status != StatusOpen {
		t.Errorf("Status = %q, want %q", wu.Status, StatusOpen)
	}
	if wu.IsProject {
		t.Error("IsProject should be false for ticket")
	}
	if wu.Dependencies == nil {
		t.Error("Dependencies should not be nil")
	}
	if len(wu.Dependencies) != 0 {
		t.Errorf("Dependencies len = %d, want 0", len(wu.Dependencies))
	}
	if wu.LastUpdated.IsZero() {
		t.Error("LastUpdated should not be zero")
	}
}

func TestNewProject(t *testing.T) {
	wu := NewProject("my-project", "My Project description")
	if wu.Identifier != "my-project" {
		t.Errorf("Identifier = %q, want %q", wu.Identifier, "my-project")
	}
	if wu.Description != "My Project description" {
		t.Errorf("Description = %q, want %q", wu.Description, "My Project description")
	}
	if wu.Status != ProjectOpen {
		t.Errorf("Status = %q, want %q", wu.Status, ProjectOpen)
	}
	if !wu.IsProject {
		t.Error("IsProject should be true for project")
	}
	if wu.Dependencies == nil {
		t.Error("Dependencies should not be nil")
	}
	if len(wu.Dependencies) != 0 {
		t.Errorf("Dependencies len = %d, want 0", len(wu.Dependencies))
	}
	if wu.LastUpdated.IsZero() {
		t.Error("LastUpdated should not be zero")
	}
}

func TestWorkUnitJSONRoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	original := &WorkUnit{
		Identifier:   "my-project/fix-bug",
		Description:  "Fix the bug in the widget",
		Status:       StatusInProgress,
		Dependencies: []string{"my-project/setup-env"},
		LastUpdated:  now,
		IsProject:    false,
		Parent:       "my-project",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded WorkUnit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Identifier != original.Identifier {
		t.Errorf("Identifier: got %q, want %q", decoded.Identifier, original.Identifier)
	}
	if decoded.Description != original.Description {
		t.Errorf("Description: got %q, want %q", decoded.Description, original.Description)
	}
	if decoded.Status != original.Status {
		t.Errorf("Status: got %q, want %q", decoded.Status, original.Status)
	}
	if len(decoded.Dependencies) != len(original.Dependencies) {
		t.Errorf("Dependencies len: got %d, want %d", len(decoded.Dependencies), len(original.Dependencies))
	} else if decoded.Dependencies[0] != original.Dependencies[0] {
		t.Errorf("Dependencies[0]: got %q, want %q", decoded.Dependencies[0], original.Dependencies[0])
	}
	if !decoded.LastUpdated.Equal(original.LastUpdated) {
		t.Errorf("LastUpdated: got %v, want %v", decoded.LastUpdated, original.LastUpdated)
	}
	if decoded.Parent != original.Parent {
		t.Errorf("Parent: got %q, want %q", decoded.Parent, original.Parent)
	}
}

func TestWorkUnitJSONFieldNames(t *testing.T) {
	wu := &WorkUnit{
		Identifier:   "fix-bug",
		Description:  "desc",
		Status:       StatusOpen,
		Dependencies: []string{},
		LastUpdated:  time.Now().UTC(),
	}

	data, err := json.Marshal(wu)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedKeys := []string{"dependencies", "description", "identifier", "last_updated", "status"}
	for _, key := range expectedKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("expected JSON key %q not found in output", key)
		}
	}
}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"fix-bug", false},
		{"colorize-output", false},
		{"react-upgrade-19", false},
		{"my-project/fix-bug", false},
		{"project/subproject/ticket", false},
		{"a", false},
		{"a1", false},
		{"a-b-c", false},
		{"", true},
		{"Fix-Bug", true},
		{"fix_bug", true},
		{"fix bug", true},
		{"-fix-bug", true},
		{"fix-bug-", true},
		{"fix--bug", false},
		{"/fix-bug", true},
		{"fix-bug/", true},
		{"fix-bug//ticket", true},
		{"1fix-bug", true},
	}

	for _, tc := range tests {
		err := ValidateIdentifier(tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("ValidateIdentifier(%q) error = %v, wantErr = %v", tc.input, err, tc.wantErr)
		}
	}
}
