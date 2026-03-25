package main

import (
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/fimmtiu/tickets/internal/db"
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
	d, err := db.Open(ticketsDir, repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: could not open database:", err)
		os.Exit(1)
	}
	defer d.Close()

	repoName := filepath.Base(repoRoot)
	model := ui.NewModel(d, repoName)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error running UI:", err)
		os.Exit(1)
	}
}
