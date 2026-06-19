package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fimmtiu/code-factory/internal/db"
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
	d, err := openDB()
	if err != nil {
		return err
	}
	defer d.Close()

	switch subcommand {
	case "add":
		return runAdd(d, args, os.Stdin)
	case "list":
		return runList(d, args)
	case "rm":
		return runRemove(d, args)
	case "prune":
		return runPrune(d, args)
	default:
		return fmt.Errorf("unknown subcommand %q; run 'cf-memory --help' for usage", subcommand)
	}
}

func runAdd(d *db.DB, args []string, stdin io.Reader) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	scope := fs.String("scope", "", "identifier prefix the memory applies to")
	kind := fs.String("kind", "lesson", "memory kind: lesson | pattern | gotcha | note")
	source := fs.String("source", "", "ticket identifier that authored the memory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Text comes from the positional arguments, or stdin when none are given.
	text := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if text == "" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("add: read stdin: %w", err)
		}
		text = strings.TrimSpace(string(data))
	}
	if text == "" {
		return fmt.Errorf("add: no memory text provided (pass as arguments or on stdin)")
	}

	id, err := d.AddMemory(*scope, *kind, text, *source)
	if err != nil {
		return err
	}
	fmt.Printf("Recorded memory %d\n", id)
	return nil
}

func runList(d *db.DB, args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	scope := fs.String("scope", "", "show only the memories a ticket/project at this identifier would receive")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var (
		memories []db.Memory
		err      error
	)
	if *scope != "" {
		// Mirrors prompt injection: global + self + ancestors, capped.
		memories, err = d.MemoriesForIdentifier(*scope, 0)
	} else {
		memories, err = d.ListMemories()
	}
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(memories, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}

func runPrune(d *db.DB, args []string) error {
	fs := flag.NewFlagSet("prune", flag.ContinueOnError)
	maxPerScope := fs.Int("max-per-scope", db.DefaultMaxPerScope, "keep at most N newest memories per scope (0 disables)")
	maxAgeDays := fs.Int("max-age", 0, "remove memories older than this many days (0 disables)")
	dryRun := fs.Bool("dry-run", false, "report what would be removed without deleting")
	if err := fs.Parse(args); err != nil {
		return err
	}

	res, err := d.PruneMemories(*maxPerScope, time.Duration(*maxAgeDays)*24*time.Hour, *dryRun)
	if err != nil {
		return err
	}

	verb := "Removed"
	if *dryRun {
		verb = "Would remove"
	}
	total := res.Duplicates + res.AgedOut + res.OverCap
	fmt.Printf("%s %d memories: %d duplicate, %d aged-out, %d over-cap\n",
		verb, total, res.Duplicates, res.AgedOut, res.OverCap)
	return nil
}

func runRemove(d *db.DB, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("rm: expected exactly one memory id")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("rm: invalid id %q: %w", args[0], err)
	}
	if err := d.DeleteMemory(id); err != nil {
		return err
	}
	fmt.Printf("Deleted memory %d\n", id)
	return nil
}
