package db_test

import (
	"testing"
)

func TestGetProjectContext_NoParent(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("root", "Root project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("root/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	// A top-level ticket's parent is "root" — one level of context.
	ctx, err := d.GetProjectContext("root/ticket")
	if err != nil {
		t.Fatalf("GetProjectContext: %v", err)
	}
	if len(ctx) != 1 {
		t.Fatalf("expected 1 context entry, got %d", len(ctx))
	}
	if ctx[0].Identifier != "root" {
		t.Errorf("expected identifier %q, got %q", "root", ctx[0].Identifier)
	}
	if ctx[0].Description != "Root project" {
		t.Errorf("expected description %q, got %q", "Root project", ctx[0].Description)
	}
}

func TestGetProjectContext_TwoLevels(t *testing.T) {
	d, _, _ := openTestDB(t)
	if err := d.CreateProject("gp", "Grandparent project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject gp: %v", err)
	}
	if err := d.CreateProject("gp/parent", "Parent project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	if err := d.CreateTicket("gp/parent/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	ctx, err := d.GetProjectContext("gp/parent/ticket")
	if err != nil {
		t.Fatalf("GetProjectContext: %v", err)
	}
	if len(ctx) != 2 {
		t.Fatalf("expected 2 context entries, got %d", len(ctx))
	}
	// First is the immediate parent.
	if ctx[0].Identifier != "gp/parent" {
		t.Errorf("ctx[0] identifier: got %q, want %q", ctx[0].Identifier, "gp/parent")
	}
	if ctx[0].Description != "Parent project" {
		t.Errorf("ctx[0] description: got %q, want %q", ctx[0].Description, "Parent project")
	}
	// Second is the grandparent.
	if ctx[1].Identifier != "gp" {
		t.Errorf("ctx[1] identifier: got %q, want %q", ctx[1].Identifier, "gp")
	}
	if ctx[1].Description != "Grandparent project" {
		t.Errorf("ctx[1] description: got %q, want %q", ctx[1].Description, "Grandparent project")
	}
}

func TestGetProjectContext_ThreeLevels(t *testing.T) {
	d, _, _ := openTestDB(t)
	for _, pair := range []struct{ id, desc string }{
		{"a", "Level A"},
		{"a/b", "Level B"},
		{"a/b/c", "Level C"},
	} {
		if err := d.CreateProject(pair.id, pair.desc, nil, "", nil); err != nil {
			t.Fatalf("CreateProject %q: %v", pair.id, err)
		}
	}
	if err := d.CreateTicket("a/b/c/ticket", "A ticket", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}

	ctx, err := d.GetProjectContext("a/b/c/ticket")
	if err != nil {
		t.Fatalf("GetProjectContext: %v", err)
	}
	if len(ctx) != 3 {
		t.Fatalf("expected 3 context entries, got %d", len(ctx))
	}
	expected := []struct{ id, desc string }{
		{"a/b/c", "Level C"},
		{"a/b", "Level B"},
		{"a", "Level A"},
	}
	for i, want := range expected {
		if ctx[i].Identifier != want.id {
			t.Errorf("ctx[%d] identifier: got %q, want %q", i, ctx[i].Identifier, want.id)
		}
		if ctx[i].Description != want.desc {
			t.Errorf("ctx[%d] description: got %q, want %q", i, ctx[i].Description, want.desc)
		}
	}
}

func TestGetProjectContext_TopLevelIdentifier(t *testing.T) {
	d, _, _ := openTestDB(t)
	// A top-level identifier with no slash has no parent.
	ctx, err := d.GetProjectContext("root")
	if err != nil {
		t.Fatalf("GetProjectContext: %v", err)
	}
	if len(ctx) != 0 {
		t.Errorf("expected 0 context entries for top-level identifier, got %d", len(ctx))
	}
}
