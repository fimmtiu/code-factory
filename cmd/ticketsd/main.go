package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fimmtiu/tickets/internal/daemon"
	"github.com/fimmtiu/tickets/internal/storage"
)

func main() {
	// Determine repo root from argument or auto-detect.
	var repoRoot string
	var err error
	if len(os.Args) > 1 {
		repoRoot = os.Args[1]
	} else {
		repoRoot, err = storage.FindRepoRoot(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: could not find repository root:", err)
			os.Exit(1)
		}
	}

	ticketsDir := storage.TicketsDirPath(repoRoot)
	socketPath := filepath.Join(ticketsDir, ".daemon.sock")

	// If another daemon is already running, report and exit cleanly.
	running, _ := daemon.IsRunning(socketPath)
	if running {
		fmt.Println("Daemon is already running.")
		os.Exit(0)
	}

	d := daemon.NewDaemon(socketPath, ticketsDir)
	if err := d.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "error starting daemon:", err)
		os.Exit(1)
	}

	// Block until the daemon stops (via signal or explicit Stop call).
	d.Wait()
}
