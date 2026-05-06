package worker_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// buildPromptTicket creates a ticket work unit for prompt tests.
func buildPromptTicket(identifier, description string, phase models.TicketPhase) *models.WorkUnit {
	return &models.WorkUnit{
		Identifier:   identifier,
		Description:  description,
		Phase:        phase,
		Status:       models.StatusIdle,
		IsProject:    false,
		Dependencies: []string{},
	}
}

func TestBuildPrompt_Implement(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "Do the thing", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "Do the thing") {
		t.Error("expected prompt to contain ticket description")
	}
	if !strings.Contains(prompt, "proj/ticket-1") {
		t.Error("expected prompt to contain ticket identifier")
	}
	worktreePath := filepath.Join(ticketsDir, "proj", "ticket-1", "worktree")
	if !strings.Contains(prompt, worktreePath) {
		t.Errorf("expected prompt to contain worktree path %q", worktreePath)
	}
	if !strings.Contains(prompt, "experienced staff software developer") {
		t.Error("expected implement preamble in prompt")
	}
	if !strings.Contains(prompt, "writing new tests") {
		t.Error("expected spec requirement in implement prompt")
	}
}

func TestBuildPrompt_ImplementWithParentContext(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("proj", "Parent project description", nil, ""); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "Implement feature X", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "Additional context from project `proj`") {
		t.Error("expected parent context heading")
	}
	if !strings.Contains(prompt, "Parent project description") {
		t.Error("expected parent project description in prompt")
	}
}

func TestBuildPrompt_ImplementWithNestedProjectContext(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("grandparent", "Top-level project", nil, ""); err != nil {
		t.Fatalf("CreateProject grandparent: %v", err)
	}
	if err := d.CreateProject("grandparent/parent", "Mid-level project", nil, ""); err != nil {
		t.Fatalf("CreateProject parent: %v", err)
	}
	createTicket(t, d, "grandparent/parent/ticket-1")

	ticket := buildPromptTicket("grandparent/parent/ticket-1", "Do nested work", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	// Both parent and grandparent contexts should appear.
	if !strings.Contains(prompt, "Additional context from project `grandparent/parent`") {
		t.Error("expected direct parent context")
	}
	if !strings.Contains(prompt, "Mid-level project") {
		t.Error("expected parent description")
	}
	if !strings.Contains(prompt, "Additional context from project `grandparent`") {
		t.Error("expected grandparent context")
	}
	if !strings.Contains(prompt, "Top-level project") {
		t.Error("expected grandparent description")
	}
}

func TestBuildPrompt_Refactor(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "Some description", models.PhaseRefactor)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "/cf-refactor") {
		t.Error("expected /cf-refactor in refactor prompt")
	}
	if !strings.Contains(prompt, "proj/ticket-1") {
		t.Error("expected ticket identifier in refactor prompt")
	}
}

func TestBuildPrompt_Review(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "Some description", models.PhaseReview)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "/cf-review") {
		t.Error("expected /cf-review in review prompt")
	}
	if !strings.Contains(prompt, "proj/ticket-1") {
		t.Error("expected ticket identifier in review prompt")
	}
}

func TestBuildPrompt_RespondingStatus(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	// Status "responding" overrides the phase and invokes /cf-respond.
	ticket := buildPromptTicket("proj/ticket-1", "Some description", models.PhaseImplement)
	ticket.Status = models.StatusResponding
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "/cf-respond") {
		t.Error("expected /cf-respond in responding-status prompt")
	}
	if !strings.Contains(prompt, "proj/ticket-1") {
		t.Error("expected ticket identifier in responding-status prompt")
	}
}

func TestBuildPrompt_UnsupportedPhase(t *testing.T) {
	d, ticketsDir := openTestDB(t)

	ticket := buildPromptTicket("some-ticket", "desc", models.PhaseBlocked)
	_, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err == nil {
		t.Error("expected error for unsupported phase, got nil")
	}
}

// ===== BuildMergingPrompt =====

func TestBuildMergingPrompt_IncludesTicketDescription(t *testing.T) {
	ticket := buildPromptTicket("proj/t1", "Add rate-limiting middleware", models.PhaseMerging)
	prompt := worker.BuildMergingPrompt(ticket, nil, nil)

	if !strings.Contains(prompt, "Add rate-limiting middleware") {
		t.Error("expected prompt to contain ticket description")
	}
	if !strings.Contains(prompt, "proj/t1") {
		t.Error("expected prompt to contain ticket identifier")
	}
}

func TestBuildMergingPrompt_IncludesSiblingDescriptions(t *testing.T) {
	ticket := buildPromptTicket("proj/t1", "Add rate-limiting middleware", models.PhaseMerging)
	siblings := []db.WorkUnitSummary{
		{Identifier: "proj/t2", Description: "Refactor the request pipeline"},
		{Identifier: "proj/t3", Description: "Add caching layer"},
	}
	prompt := worker.BuildMergingPrompt(ticket, siblings, nil)

	if !strings.Contains(prompt, "Refactor the request pipeline") {
		t.Error("expected prompt to contain sibling t2 description")
	}
	if !strings.Contains(prompt, "Add caching layer") {
		t.Error("expected prompt to contain sibling t3 description")
	}
	if !strings.Contains(prompt, "proj/t2") {
		t.Error("expected prompt to contain sibling t2 identifier")
	}
	if !strings.Contains(prompt, "proj/t3") {
		t.Error("expected prompt to contain sibling t3 identifier")
	}
}

func TestBuildMergingPrompt_IncludesParentContext(t *testing.T) {
	ticket := buildPromptTicket("proj/t1", "Add rate-limiting", models.PhaseMerging)
	contexts := []db.ProjectContext{
		{Identifier: "proj", Description: "Build a web server with auth and rate limiting"},
	}
	prompt := worker.BuildMergingPrompt(ticket, nil, contexts)

	if !strings.Contains(prompt, "Build a web server with auth and rate limiting") {
		t.Error("expected prompt to contain parent project description")
	}
}

func TestBuildMergingPrompt_ContainsRebaseInstructions(t *testing.T) {
	ticket := buildPromptTicket("proj/t1", "A ticket", models.PhaseMerging)
	prompt := worker.BuildMergingPrompt(ticket, nil, nil)

	if !strings.Contains(prompt, "rebase") {
		t.Error("expected prompt to contain rebase instructions")
	}
	if !strings.Contains(prompt, "linting and tests") {
		t.Error("expected prompt to contain linting/test requirement")
	}
}

func TestBuildMergingPrompt_NoSiblingsStillWorks(t *testing.T) {
	ticket := buildPromptTicket("proj/only", "The only ticket", models.PhaseMerging)
	prompt := worker.BuildMergingPrompt(ticket, nil, nil)

	if !strings.Contains(prompt, "The only ticket") {
		t.Error("expected prompt to contain ticket description even without siblings")
	}
	if strings.Contains(prompt, "Sibling") {
		t.Error("expected no sibling section when there are no siblings")
	}
}

func TestBuildMergingPrompt_TopLevelTicket(t *testing.T) {
	ticket := buildPromptTicket("standalone", "A standalone ticket", models.PhaseMerging)
	prompt := worker.BuildMergingPrompt(ticket, nil, nil)

	if !strings.Contains(prompt, "A standalone ticket") {
		t.Error("expected prompt to contain ticket description")
	}
}
