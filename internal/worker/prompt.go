package worker

import (
	"fmt"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

// BuildPrompt generates the appropriate prompt for the Claude agent based on
// the ticket's current phase. For the implement phase, it also appends context
// from the ticket's parent project hierarchy.
func BuildPrompt(ticket *models.WorkUnit, database *db.DB, ticketsDir string) (string, error) {
	identifier := ticket.Identifier
	worktreePath := storage.TicketWorktreePathIn(ticketsDir, identifier)

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
				"You may create intermediate commits if you need to, as long as you give them complete commit messages.",
			worktreePath, identifier, ticket.Description,
		)

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

	case models.PhaseRefactor:
		prompt = fmt.Sprintf("/cf-refactor on worktree `%s` for ticket `%s`", worktreePath, identifier)

	case models.PhaseReview:
		prompt = fmt.Sprintf("/cf-review on worktree `%s` for ticket `%s`", worktreePath, identifier)

	case models.PhaseRespond:
		prompt = fmt.Sprintf("/cf-respond on worktree `%s` for ticket `%s`", worktreePath, identifier)

	default:
		return "", fmt.Errorf("BuildPrompt: unsupported ticket phase %q", ticket.Phase)
	}

	return prompt, nil
}
