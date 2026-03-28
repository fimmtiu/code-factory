package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/tickets/internal/config"
	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/storage"
	"github.com/fimmtiu/tickets/internal/ui"
	"github.com/fimmtiu/tickets/internal/worker"
)

func main() {
	fs := flag.NewFlagSet("code-factory", flag.ContinueOnError)
	poolSize := fs.Int("pool", 4, "worker pool size")
	waitSecs := fs.Int("wait", 5, "poll interval in seconds")
	mock := fs.Bool("mock", false, "use mock workers instead of real ACP subprocesses")

	// Support short flags -p and -w as aliases
	fs.IntVar(poolSize, "p", 4, "worker pool size (shorthand)")
	fs.IntVar(waitSecs, "w", 5, "poll interval in seconds (shorthand)")

	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, `Usage: code-factory [-p <pool>] [-w <wait>] [--mock]

Options:
  -p, --pool <N>   Worker pool size (default 4)
  -w, --wait <N>   Poll interval in seconds (default 5)
      --mock       Use mock workers for UI testing (no real Claude subprocess)
  -h, --help       Show this help message`)
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		if err == flag.ErrHelp {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	if *poolSize <= 0 {
		fmt.Fprintln(os.Stderr, "error: pool size must be a positive integer")
		os.Exit(1)
	}
	if *waitSecs <= 0 {
		fmt.Fprintln(os.Stderr, "error: wait interval must be a positive integer")
		os.Exit(1)
	}

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: cannot determine current directory:", err)
		os.Exit(1)
	}

	repoRoot, err := storage.FindRepoRoot(cwd)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: not inside a git repository")
		os.Exit(1)
	}

	ticketsDir := filepath.Join(repoRoot, ".tickets")
	info, err := os.Stat(ticketsDir)
	if err != nil || !info.IsDir() {
		fmt.Fprintln(os.Stderr, "error: .tickets/ directory not found; run 'tickets init' first")
		os.Exit(1)
	}

	if err := config.Init(ticketsDir); err != nil {
		fmt.Fprintln(os.Stderr, "error: loading settings:", err)
		os.Exit(1)
	}

	database, err := db.Open(ticketsDir, repoRoot)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: opening database:", err)
		os.Exit(1)
	}
	defer database.Close()

	// NewPool defaults WorkFn to the real ACP subprocess; --mock overrides it.
	pool := worker.NewPool(*poolSize, *waitSecs)
	if *mock {
		pool.WorkFn = worker.MockWorkFn
	}
	pool.Start(database, ticketsDir)
	pool.StartHousekeeping(database)
	pool.StartLogDrainer(database)

	// Redirect the standard logger to a file so library log output doesn't
	// corrupt the bubbletea terminal display.
	if logFile, err := os.OpenFile(filepath.Join(ticketsDir, "code-factory.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		log.SetOutput(logFile)
		defer logFile.Close()
	}

	model := ui.NewModel(pool, database, *waitSecs)
	prog := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := prog.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error: TUI exited with error:", err)
	}

	pool.Stop()
}
