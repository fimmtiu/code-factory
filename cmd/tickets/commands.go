package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
)

func openDB() (*db.DB, error) {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return nil, err
	}
	return db.Open(storage.TicketsDirPath(repoRoot), repoRoot)
}

func runCommand(subcommand string, args []string) error {
	if subcommand == "init" {
		return runInit()
	}

	d, err := openDB()
	if err != nil {
		return err
	}
	defer d.Close()

	switch subcommand {
	case "status":
		return runStatus(d)
	case "create-project":
		return runCreateProject(d, args, os.Stdin)
	case "create-ticket":
		if len(args) == 0 && isatty.IsTerminal(os.Stdin.Fd()) {
			return runWizard(d, "ticket")
		}
		return runCreateTicket(d, args, os.Stdin)
	case "set-status":
		return runSetStatus(d, args)
	case "claim":
		return runClaim(d, args)
	case "release":
		return runRelease(d, args)
	case "create-cr":
		return runAddChangeRequest(d, args, os.Stdin)
	case "close-cr":
		return runCloseChangeRequest(d, args)
	case "dismiss-cr":
		return runDismissChangeRequest(d, args)
	case "list-crs":
		return runOpenChangeRequests(d, args)
	case "reset":
		return runReset(d, args)
	case "new":
		return runWizard(d, "ticket")
	case "new-project":
		return runWizard(d, "project")
	default:
		return fmt.Errorf("unknown subcommand %q; run 'tickets' for usage", subcommand)
	}
}

func runInit() error {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return err
	}
	if err := storage.InitTicketsDir(repoRoot); err != nil {
		return err
	}
	fmt.Printf("Initialized .tickets/ in %s\n", repoRoot)
	return nil
}

func runStatus(d *db.DB) error {
	units, err := d.Status()
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(units, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

type stdinInput struct {
	Description  string   `json:"description"`
	Dependencies []string `json:"dependencies"`
	ParentBranch string   `json:"parent_branch"`
}

func parseStdinInput(cmdName string, r io.Reader) (stdinInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return stdinInput{}, fmt.Errorf("%s: read stdin: %w", cmdName, err)
	}
	var input stdinInput
	if err := json.Unmarshal(data, &input); err != nil {
		return stdinInput{}, fmt.Errorf("%s: parse stdin JSON: %w", cmdName, err)
	}
	if input.Description == "" {
		return stdinInput{}, fmt.Errorf("%s: stdin JSON must contain a 'description' field", cmdName)
	}
	return input, nil
}

func runCreateProject(d *db.DB, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-project <identifier>")
	}
	input, err := parseStdinInput("create-project", stdin)
	if err != nil {
		return err
	}
	return d.CreateProject(args[0], input.Description, input.Dependencies, input.ParentBranch)
}

func runCreateTicket(d *db.DB, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-ticket <identifier>")
	}
	input, err := parseStdinInput("create-ticket", stdin)
	if err != nil {
		return err
	}
	return d.CreateTicket(args[0], input.Description, input.Dependencies, input.ParentBranch)
}

func runSetStatus(d *db.DB, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tickets set-status <identifier> <phase> [<status>]")
	}
	status := models.StatusIdle
	if len(args) >= 3 {
		status = models.TicketStatus(args[2])
	}
	return d.SetStatus(args[0], models.TicketPhase(args[1]), status)
}

func runClaim(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets claim <pid>")
	}
	pid, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("claim: invalid pid %q: %w", args[0], err)
	}
	wu, err := d.Claim(pid)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(wu, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runRelease(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets release <identifier>")
	}
	return d.Release(args[0])
}

func runAddChangeRequest(d *db.DB, args []string, stdin io.Reader) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: tickets create-cr <identifier> <code-location> <author>")
	}
	text, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("create-cr: read stdin: %w", err)
	}
	return d.AddChangeRequest(args[0], args[1], args[2], string(text))
}

func runCloseChangeRequest(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets close-cr <id> [<explanation>]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("close-cr: invalid id %q: %w", args[0], err)
	}
	if len(args) >= 2 {
		if err := d.AppendChangeRequestDescription(id, args[1]); err != nil {
			return fmt.Errorf("close-cr: append description: %w", err)
		}
	}
	return d.CloseChangeRequest(id)
}

func runOpenChangeRequests(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets list-crs <identifier>")
	}
	crs, err := d.OpenChangeRequests(args[0])
	if err != nil {
		return err
	}
	if crs == nil {
		crs = []models.ChangeRequest{}
	}
	out, err := json.MarshalIndent(crs, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runDismissChangeRequest(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets dismiss-cr <id> [<reason>]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("dismiss-cr: invalid id %q: %w", args[0], err)
	}
	if len(args) >= 2 {
		if err := d.AppendChangeRequestDescription(id, args[1]); err != nil {
			return err
		}
	}
	return d.DismissChangeRequest(id)
}

// allProjects returns all work units that are projects.
func allProjects(d *db.DB) ([]*models.WorkUnit, error) {
	units, err := d.Status()
	if err != nil {
		return nil, err
	}
	var projects []*models.WorkUnit
	for _, u := range units {
		if u.IsProject {
			projects = append(projects, u)
		}
	}
	return projects, nil
}

func runWizard(d *db.DB, kind string) error {
	projects, err := allProjects(d)
	if err != nil {
		return err
	}
	prog := tea.NewProgram(newWizard(kind, projects), tea.WithAltScreen())
	result, err := prog.Run()
	if err != nil {
		return err
	}
	final := result.(wizardModel)
	if final.cancelled {
		fmt.Fprintln(os.Stderr, "Cancelled.")
		return nil
	}
	desc := strings.TrimSpace(final.descText)
	if kind == "project" {
		return d.CreateProject(final.fullIdentifier(), desc, []string{}, "")
	}
	return d.CreateTicket(final.fullIdentifier(), desc, []string{}, "")
}

func runReset(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets reset <ticket-identifier>")
	}
	identifier := args[0]

	// Look up the ticket and guard against blocked/done phase.
	units, err := d.Status()
	if err != nil {
		return err
	}
	var found *models.WorkUnit
	for _, u := range units {
		if u.Identifier == identifier {
			found = u
			break
		}
	}
	if found == nil {
		return fmt.Errorf("ticket %q not found", identifier)
	}
	if found.Phase == models.PhaseBlocked || found.Phase == models.PhaseDone {
		return fmt.Errorf("cannot reset ticket %q in %q phase", identifier, found.Phase)
	}

	// Resolve the target HEAD: parent worktree HEAD or default branch HEAD.
	worktreePath, err := storage.WorktreePathForIdentifier(identifier)
	if err != nil {
		return fmt.Errorf("reset: resolve worktree path: %w", err)
	}

	var targetHEAD string
	if parentID, ok := models.ParentIdentifierOf(identifier); ok {
		parentPath, err := storage.WorktreePathForIdentifier(parentID)
		if err != nil {
			return fmt.Errorf("reset: resolve parent worktree: %w", err)
		}
		targetHEAD, err = git.Output(parentPath, "rev-parse", "HEAD")
		if err != nil {
			return fmt.Errorf("reset: get parent HEAD: %w", err)
		}
	} else {
		branch := git.DetectDefaultBranch(worktreePath)
		targetHEAD, err = git.Output(worktreePath, "rev-parse", branch)
		if err != nil {
			return fmt.Errorf("reset: get default branch HEAD: %w", err)
		}
	}

	// Clean uncommitted work and reset to target HEAD.
	if _, err := git.Output(worktreePath, "checkout", "--", "."); err != nil {
		return fmt.Errorf("reset: discard changes: %w", err)
	}
	if _, err := git.Output(worktreePath, "clean", "-fd"); err != nil {
		return fmt.Errorf("reset: clean untracked files: %w", err)
	}
	if _, err := git.Output(worktreePath, "reset", "--hard", targetHEAD); err != nil {
		return fmt.Errorf("reset: reset HEAD: %w", err)
	}

	// Delete all change requests.
	if err := d.DeleteChangeRequestsForTicket(identifier); err != nil {
		return fmt.Errorf("reset: delete change requests: %w", err)
	}

	// Set ticket to implement/idle.
	if err := d.SetStatus(identifier, models.PhaseImplement, models.StatusIdle); err != nil {
		return fmt.Errorf("reset: set status: %w", err)
	}

	fmt.Printf("Reset ticket %q to implement/idle\n", identifier)
	return nil
}
