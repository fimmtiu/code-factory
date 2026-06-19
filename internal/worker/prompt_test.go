package worker_test

import (
	"os"
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

// installFakeSkill writes a SKILL.md under a tempdir HOME so BuildPrompt can
// inline it. The body is wrapped in YAML frontmatter so the frontmatter
// stripper has something to strip.
func installFakeSkill(t *testing.T, name, body string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll %s: %v", dir, err)
	}
	content := "---\nname: " + name + "\ndescription: fake\n---\n\n" + body
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile SKILL.md: %v", err)
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
	if err := d.CreateProject("proj", "Parent project description", nil, "", nil); err != nil {
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
	if err := d.CreateProject("grandparent", "Top-level project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject grandparent: %v", err)
	}
	if err := d.CreateProject("grandparent/parent", "Mid-level project", nil, "", nil); err != nil {
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

func TestBuildPrompt_ImplementWithDependencyContext(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("proj", "Top-level project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("proj/loop", "Refresh loop. Exposes Refresh, RefreshJob, and Loop methods on *Runner.", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket dep: %v", err)
	}
	if err := d.CreateTicket("proj/main", "Wires startup, the worker, refresh, and the TUI together.", []string{"proj/loop"}, "", nil); err != nil {
		t.Fatalf("CreateTicket dependent: %v", err)
	}

	ticket := buildPromptTicket("proj/main", "Wires startup, the worker, refresh, and the TUI together.", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "Prerequisites already merged into this worktree") {
		t.Error("expected prerequisites heading in prompt")
	}
	if !strings.Contains(prompt, "`proj/loop`") {
		t.Error("expected dependency identifier in prompt")
	}
	if !strings.Contains(prompt, "Refresh, RefreshJob, and Loop methods") {
		t.Error("expected dependency description in prompt so the agent learns the public API names")
	}
	if !strings.Contains(prompt, "do not write parallel stubs") {
		t.Error("expected the consume-existing-API instruction in prompt")
	}
}

func TestBuildPrompt_ImplementWithoutDependenciesOmitsSection(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("proj", "P", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	createTicket(t, d, "proj/loner")

	ticket := buildPromptTicket("proj/loner", "no deps", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if strings.Contains(prompt, "Prerequisites already merged") {
		t.Error("did not expect prerequisites heading when ticket has no dependencies")
	}
}

func TestBuildPrompt_Refactor(t *testing.T) {
	installFakeSkill(t, "cf-refactor", "REFACTOR_BODY_MARKER")
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
	if !strings.Contains(prompt, "REFACTOR_BODY_MARKER") {
		t.Error("expected inlined cf-refactor skill body in refactor prompt")
	}
	if strings.Contains(prompt, "name: cf-refactor") {
		t.Error("expected YAML frontmatter to be stripped from inlined skill body")
	}
}

func TestBuildPrompt_Review(t *testing.T) {
	installFakeSkill(t, "cf-review", "REVIEW_BODY_MARKER")
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
	if !strings.Contains(prompt, "REVIEW_BODY_MARKER") {
		t.Error("expected inlined cf-review skill body in review prompt")
	}
}

func TestBuildPrompt_ReviewMissingSkillIsError(t *testing.T) {
	// Point HOME at a tempdir with no skills installed and confirm we surface
	// the missing skill rather than silently shipping a degraded prompt.
	t.Setenv("HOME", t.TempDir())
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "desc", models.PhaseReview)
	_, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err == nil {
		t.Fatal("expected error when cf-review skill is not installed")
	}
	if !strings.Contains(err.Error(), "cf-review") {
		t.Errorf("expected error to mention cf-review, got %v", err)
	}
}

func TestBuildPrompt_RespondingStatus(t *testing.T) {
	installFakeSkill(t, "cf-respond", "RESPOND_BODY_MARKER")
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
	if !strings.Contains(prompt, "RESPOND_BODY_MARKER") {
		t.Error("expected inlined cf-respond skill body in responding-status prompt")
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

// ===== write_scope in implement prompt =====

func TestBuildPrompt_ImplementIncludesWriteScope(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("proj/scoped", "Do scoped work", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	ticket := buildPromptTicket("proj/scoped", "Do scoped work", models.PhaseImplement)
	ticket.WriteScope = []string{"internal/db/", "cmd/cf-tickets/main.go"}
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "internal/db/") {
		t.Error("expected prompt to contain write_scope path internal/db/")
	}
	if !strings.Contains(prompt, "cmd/cf-tickets/main.go") {
		t.Error("expected prompt to contain write_scope path cmd/cf-tickets/main.go")
	}
	if !strings.Contains(prompt, "write_scope") || !strings.Contains(prompt, "MUST NOT") {
		t.Error("expected prompt to contain write_scope constraint language")
	}
}

func TestBuildPrompt_ImplementOmitsWriteScopeWhenEmpty(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/noscope")

	ticket := buildPromptTicket("proj/noscope", "Do unscoped work", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if strings.Contains(prompt, "write_scope") {
		t.Error("expected prompt to NOT contain write_scope when none is set")
	}
}

func TestBuildPrompt_RefactorDoesNotIncludeWriteScope(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	if err := d.CreateProject("proj", "A project", nil, "", nil); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := d.CreateTicket("proj/refac", "Refactor something", nil, "", nil); err != nil {
		t.Fatalf("CreateTicket: %v", err)
	}
	ticket := buildPromptTicket("proj/refac", "Refactor something", models.PhaseRefactor)
	ticket.WriteScope = []string{"internal/db/"}
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	// Refactor uses /cf-refactor skill, not the raw implement prompt.
	// write_scope should not appear.
	if strings.Contains(prompt, "write_scope") {
		t.Error("expected refactor prompt to NOT contain write_scope")
	}
}

func TestBuildPrompt_ImplementIncludesScopedMemory(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	if _, err := d.AddMemory("proj", "gotcha", "rake compile needs Linux/Docker", ""); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}
	if _, err := d.AddMemory("other", "lesson", "should not appear", ""); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}

	ticket := buildPromptTicket("proj/ticket-1", "Do the thing", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "Repository memory") {
		t.Error("expected memory section heading in implement prompt")
	}
	if !strings.Contains(prompt, "rake compile needs Linux/Docker") {
		t.Error("expected in-scope memory text in prompt")
	}
	if strings.Contains(prompt, "should not appear") {
		t.Error("out-of-scope memory leaked into prompt")
	}
}

func TestBuildPrompt_SkillPhaseIncludesMemory(t *testing.T) {
	installFakeSkill(t, "cf-refactor", "Refactor body")
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/refac")

	if _, err := d.AddMemory("", "pattern", "prefer table-driven tests here", ""); err != nil {
		t.Fatalf("AddMemory: %v", err)
	}

	ticket := buildPromptTicket("proj/refac", "Refactor something", models.PhaseRefactor)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}

	if !strings.Contains(prompt, "Repository memory") {
		t.Error("expected memory section in refactor (skill) prompt")
	}
	if !strings.Contains(prompt, "prefer table-driven tests here") {
		t.Error("expected global memory text in skill prompt")
	}
}

func TestBuildPrompt_NoMemorySectionWhenEmpty(t *testing.T) {
	d, ticketsDir := openTestDB(t)
	createProject(t, d, "proj")
	createTicket(t, d, "proj/ticket-1")

	ticket := buildPromptTicket("proj/ticket-1", "Do the thing", models.PhaseImplement)
	prompt, err := worker.BuildPrompt(ticket, d, ticketsDir)
	if err != nil {
		t.Fatalf("BuildPrompt: %v", err)
	}
	if strings.Contains(prompt, "Repository memory") {
		t.Error("did not expect a memory section when no memories exist")
	}
}
