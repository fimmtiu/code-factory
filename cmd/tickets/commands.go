package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/fimmtiu/code-factory/internal/db"
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
		return runCreateTicket(d, args, os.Stdin)
	case "set-status":
		return runSetStatus(d, args)
	case "claim":
		return runClaim(d, args)
	case "release":
		return runRelease(d, args)
	case "add-change-request":
		return runAddChangeRequest(d, args, os.Stdin)
	case "close-change-request":
		return runCloseChangeRequest(d, args)
	case "dismiss-change-request":
		return runDismissChangeRequest(d, args)
	case "open-change-requests":
		return runOpenChangeRequests(d, args)
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
	return d.CreateProject(args[0], input.Description, input.Dependencies)
}

func runCreateTicket(d *db.DB, args []string, stdin io.Reader) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets create-ticket <identifier>")
	}
	input, err := parseStdinInput("create-ticket", stdin)
	if err != nil {
		return err
	}
	return d.CreateTicket(args[0], input.Description, input.Dependencies)
}

func runSetStatus(d *db.DB, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: tickets set-status <identifier> <phase> [<status>]")
	}
	status := models.StatusIdle
	if len(args) >= 3 {
		status = args[2]
	}
	return d.SetStatus(args[0], args[1], status)
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
		return fmt.Errorf("usage: tickets add-change-request <identifier> <code-location> <author>")
	}
	text, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("add-change-request: read stdin: %w", err)
	}
	return d.AddChangeRequest(args[0], args[1], args[2], string(text))
}

func runCloseChangeRequest(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets close-change-request <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("close-change-request: invalid id %q: %w", args[0], err)
	}
	return d.CloseChangeRequest(id)
}

func runOpenChangeRequests(d *db.DB, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: tickets open-change-requests <identifier>")
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
		return fmt.Errorf("usage: tickets dismiss-change-request <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("dismiss-change-request: invalid id %q: %w", args[0], err)
	}
	return d.DismissChangeRequest(id)
}
