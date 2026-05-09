package worker

import (
	"fmt"
	"strings"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

// BuildPrompt generates the appropriate prompt for the Claude agent based on
// the ticket's current status and phase. A status of "responding" runs the
// /cf-respond skill regardless of phase; otherwise the phase drives the
// prompt. For the implement phase, it also appends context from the ticket's
// parent project hierarchy.
func BuildPrompt(ticket *models.WorkUnit, database *db.DB, ticketsDir string) (string, error) {
	identifier := ticket.Identifier
	worktreePath := storage.TicketWorktreePathIn(ticketsDir, identifier)

	if ticket.Status == models.StatusResponding {
		env := DetectWorktreeEnv(worktreePath)
		return env.FormatEnvBlock() +
			fmt.Sprintf("/cf-respond on worktree `%s` for ticket `%s`", worktreePath, identifier), nil
	}

	var prompt string
	switch ticket.Phase {
	case models.PhaseImplement:
		prompt = fmt.Sprintf(
			"You are an experienced staff software developer with a keen eye for abstraction and good code design. "+
				"Implement the following work in the git worktree `%s` for ticket `%s`:\n\n%s\n\n"+
				"For all of your changes, you MUST begin by writing new tests or editing existing relevant tests to cover your changes before you start coding. "+
				"Tests that cover your planned changes MUST exist before you begin changing the implementation.\n\n"+
				"Before you commit, you MUST run linting and tests to ensure your changes are working as expected.\n\n"+
				"When you're done, commit your changes to the worktree with a commit message that explains what you did and why you did it. "+
				"Your commit message must not be prefixed with `cf-respond:` or `refactor:`. "+
				"You may create intermediate commits if you need to, as long as you give them complete commit messages.",
			worktreePath, identifier, ticket.Description,
		)

		// Append write_scope constraint if set. The scope is already
		// populated on the WorkUnit at claim time, so no DB call needed.
		if len(ticket.WriteScope) > 0 {
			prompt += "\n\n### write_scope\n\nThis ticket's declared write_scope is:\n"
			for _, s := range ticket.WriteScope {
				prompt += fmt.Sprintf("- `%s`\n", s)
			}
			prompt += "\nYou MUST NOT create or modify files outside these paths. " +
				"If you believe changes outside the write_scope are necessary, " +
				"note them in your commit message but do not make them."
		}

		// Append parent project context recursively.
		contexts, err := database.GetProjectContext(identifier)
		if err != nil {
			return "", fmt.Errorf("BuildPrompt: get project context: %w", err)
		}
		for _, ctx := range contexts {
			prompt += fmt.Sprintf(
				"\n\n### Additional context from project `%s`\n\n%s",
				ctx.Identifier, ctx.Description,
			)
		}

		// Append prerequisite descriptions. The dependencies are already
		// merged into this worktree, so their public APIs exist on disk —
		// the agent should grep for and consume them rather than invent
		// parallel stubs (which is how the gem-upgrader refresh.NewLoop
		// regression slipped through).
		deps, err := database.GetDependencyContext(identifier)
		if err != nil {
			return "", fmt.Errorf("BuildPrompt: get dependency context: %w", err)
		}
		if len(deps) > 0 {
			prompt += "\n\n### Prerequisites already merged into this worktree\n\n" +
				"Each item below is a ticket or project this one depends on. Their " +
				"code is already present in the worktree. Before adding new types, " +
				"functions, or files, search the existing tree for the symbols these " +
				"prerequisites describe and consume them directly — do not write " +
				"parallel stubs of an API that already exists."
			for _, dep := range deps {
				prompt += fmt.Sprintf(
					"\n\n#### `%s`\n\n%s",
					dep.Identifier, dep.Description,
				)
			}
		}

	case models.PhaseRefactor:
		env := DetectWorktreeEnv(worktreePath)
		prompt = env.FormatEnvBlock() +
			fmt.Sprintf("/cf-refactor on worktree `%s` for ticket `%s`", worktreePath, identifier)

	case models.PhaseReview:
		env := DetectWorktreeEnv(worktreePath)
		prompt = env.FormatEnvBlock() +
			fmt.Sprintf("/cf-review on worktree `%s` for ticket `%s`", worktreePath, identifier)

	default:
		return "", fmt.Errorf("BuildPrompt: unsupported ticket phase %q", ticket.Phase)
	}

	return prompt, nil
}

// BuildMergingPrompt constructs an intent-aware prompt for the merging agent.
// It includes the ticket's own description (what it was trying to do), the
// descriptions of sibling tickets/subprojects under the same parent (whose
// merged commits are the likely source of the conflict), and parent project
// context. This gives the agent enough semantic context to resolve conflicts
// intelligently instead of staring at raw conflict markers.
//
// The caller is responsible for fetching siblings and contexts from the
// database; this function is pure string formatting.
func BuildMergingPrompt(ticket *models.WorkUnit, siblings []db.WorkUnitSummary, contexts []db.ProjectContext) string {
	var b strings.Builder

	b.WriteString("Complete this rebase, fixing the current conflict and any further conflicts that arise when applying remaining commits. ")
	b.WriteString("After staging each resolved file with `git add`, run `git rerere` before `git rebase --continue` so the resolution is recorded for future reuse. ")
	b.WriteString("You must run linting and tests before committing.\n\n")

	b.WriteString(fmt.Sprintf("### This ticket: `%s`\n\n%s\n", ticket.Identifier, ticket.Description))

	if len(siblings) > 0 {
		b.WriteString("\n### Sibling tickets whose changes may conflict\n")
		for _, s := range siblings {
			b.WriteString(fmt.Sprintf("\n**`%s`**: %s\n", s.Identifier, s.Description))
		}
	}

	for _, ctx := range contexts {
		b.WriteString(fmt.Sprintf(
			"\n### Additional context from project `%s`\n\n%s\n",
			ctx.Identifier, ctx.Description,
		))
	}

	return b.String()
}
