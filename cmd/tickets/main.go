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
  status                          Show daemon status
  create-project <id>             Create a project (reads JSON description from stdin)
  create-ticket <id>              Create a ticket (reads JSON description from stdin)
  set-status <id> <status>        Set a ticket's status (use "done" to merge and cascade)
  claim <pid>                     Claim the next available ticket for the given process ID
  release <id>                    Release the claim on a ticket
  add-comment <id> <loc> <author> <text>
                                  Add a comment to a work unit at a code location
  close-thread <thread-id>        Close a comment thread`)
}
