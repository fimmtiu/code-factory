package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]
	if subcommand == "--help" || subcommand == "-h" || subcommand == "help" {
		printUsage()
		os.Exit(0)
	}

	if err := runCommand(subcommand, os.Args[2:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: cf-memory <subcommand> [args]

Record and read repository-level memory: lessons, patterns, and notes that span
tickets and projects. A memory is injected into the prompts of tickets within
its scope.

Subcommands:
  add [flags] [text]   Record a memory (text from arguments, or stdin if omitted)
      --scope <id>     Identifier prefix it applies to (default: repository-wide)
      --kind <kind>    lesson | pattern | gotcha | note (default: lesson)
      --source <id>    Ticket identifier that authored it
  list [flags]         List memories as JSON
      --scope <id>     Show only the memories a ticket/project at <id> would receive
  rm <id>              Delete a memory by id
  prune [flags]        Curate the store: drop duplicates, aged-out, and over-cap entries
      --max-per-scope N  Keep at most N newest per scope (default 50; 0 disables)
      --max-age DAYS     Remove memories older than DAYS (0 disables)
      --dry-run          Report what would be removed without deleting`)
}
