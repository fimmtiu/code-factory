package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fimmtiu/tickets/internal/client"
	"github.com/fimmtiu/tickets/internal/storage"
	"github.com/fimmtiu/tickets/internal/ui"
)

func main() {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not find repository root:", err)
		os.Exit(1)
	}

	ticketsDir := storage.TicketsDirPath(repoRoot)
	socketPath := filepath.Join(ticketsDir, ".daemon.sock")

	if err := client.EnsureRunning(socketPath, repoRoot); err != nil {
		fmt.Fprintln(os.Stderr, "error: could not start daemon:", err)
		os.Exit(1)
	}

	model := ui.NewModel(socketPath)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running UI:", err)
		os.Exit(1)
	}
}
