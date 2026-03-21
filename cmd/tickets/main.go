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
	args := os.Args[2:]

	if err := runCommand(subcommand, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: tickets <subcommand> [args]

Subcommands (no daemon required):
  init                  Initialize .tickets/ directory in the current repo
  running               Check if the daemon is running
  exit                  Stop the daemon if running

Subcommands (auto-start daemon if needed):
  status                Show daemon status
  create-project <id>   Create a project (reads JSON description from stdin)
  create-ticket <id>    Create a ticket (reads JSON description from stdin)
  get-work              Get the next work item
  review-ready <id>     Mark a ticket as ready for review
  get-review            Get the next ticket to review
  done <id>             Mark a ticket as done`)
}
