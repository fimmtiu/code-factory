package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/worker"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	workerStatusStyle = lipgloss.NewStyle().Bold(true)

	workerIdleStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Inherit(workerStatusStyle) // grey
	workerAwaitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Inherit(workerStatusStyle)   // red
	workerBusyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Inherit(workerStatusStyle)  // dark green
	workerPausedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Inherit(workerStatusStyle)   // yellow

	workerOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")) // dim grey for output lines

	workerNoOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")) // light grey
)

// linesPerWorker is the number of rendered lines each worker section occupies:
// 1 status line + 3 output lines + 1 separator.
const linesPerWorker = 5

// ── Messages ─────────────────────────────────────────────────────────────────

type workerTickMsg struct{}

// ── WorkerView ────────────────────────────────────────────────────────────────

// WorkerView is a read-only full-screen pane that shows each worker's status
// and the last three lines of agent output. It supports scrolling but has no
// selectable item.
type WorkerView struct {
	pool   *worker.Pool
	width  int
	height int
	offset int // first visible line (scroll offset)
}

// NewWorkerView creates a WorkerView backed by the given worker pool.
func NewWorkerView(pool *worker.Pool) WorkerView {
	return WorkerView{pool: pool}
}

// Init schedules the first periodic refresh tick.
func (v WorkerView) Init() tea.Cmd {
	return v.tickCmd()
}

// tickCmd schedules a refresh tick at 500 ms intervals.
func (v WorkerView) tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return workerTickMsg{}
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func (v WorkerView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case workerTickMsg:
		// Re-render on each tick; schedule the next tick.
		return v, v.tickCmd()

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

func (v WorkerView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		v.scrollBy(-1)
	case "down":
		v.scrollBy(1)
	case "pgup":
		v.scrollBy(-v.viewHeight())
	case "pgdown":
		v.scrollBy(v.viewHeight())
	}
	return v, nil
}

func (v *WorkerView) scrollBy(delta int) {
	v.offset += delta
	v.clampScroll()
}

func (v *WorkerView) clampScroll() {
	if v.offset < 0 {
		v.offset = 0
	}
	total := v.totalLines()
	vh := v.viewHeight()
	max := total - vh
	if max < 0 {
		max = 0
	}
	if v.offset > max {
		v.offset = max
	}
}

// ── Dimension helpers ─────────────────────────────────────────────────────────

// viewHeight returns the number of lines available for the list body.
func (v WorkerView) viewHeight() int {
	h := v.height - chromeHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// totalLines returns the total rendered line count for all workers.
func (v WorkerView) totalLines() int {
	if v.pool == nil {
		return 0
	}
	return len(v.pool.Workers) * linesPerWorker
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v WorkerView) View() string {
	if v.pool == nil || len(v.pool.Workers) == 0 {
		return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.viewHeight()).Render("(no workers)")
	}

	// Build all lines for all workers.
	all := v.renderAllLines()

	vh := v.viewHeight()
	start := v.offset
	if start >= len(all) {
		start = len(all) - 1
		if start < 0 {
			start = 0
		}
	}
	end := start + vh
	if end > len(all) {
		end = len(all)
	}

	return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.viewHeight()).Render(strings.Join(all[start:end], "\n"))
}

// renderAllLines builds the full list of display lines for every worker.
func (v WorkerView) renderAllLines() []string {
	innerW := v.width - viewBorderOverhead
	if innerW <= 0 {
		innerW = 1
	}
	separator := strings.Repeat("─", innerW)

	var lines []string
	for _, w := range v.pool.Workers {
		// Status line: "<N>: <status>"
		statusLine := v.renderStatusLine(w)
		lines = append(lines, statusLine)

		// Last three lines of agent output. When there is no output at all,
		// show "(no output)" in light grey on the middle line only.
		output := w.GetLastOutput()
		for i := 0; i < 3; i++ {
			if i < len(output) {
				line := truncateLine(output[i], innerW)
				lines = append(lines, workerOutputStyle.Render(line))
			} else if len(output) == 0 && i == 1 {
				lines = append(lines, workerNoOutputStyle.Render("(no output)"))
			} else {
				lines = append(lines, "")
			}
		}

		// Separator line.
		lines = append(lines, separator)
	}
	return lines
}

// renderStatusLine returns the styled "Worker <N>: <status>" line for a worker.
func (v WorkerView) renderStatusLine(w *worker.Worker) string {
	text := fmt.Sprintf("Worker %d: %s", w.Number, w.Status.String())
	if w.Paused {
		text = fmt.Sprintf("Worker %d: paused", w.Number)
		return workerPausedStyle.Render(text)
	}
	switch w.Status {
	case worker.StatusIdle:
		return workerIdleStyle.Render(text)
	case worker.StatusAwaitingResponse:
		return workerAwaitingStyle.Render(text)
	case worker.StatusBusy:
		return workerBusyStyle.Render(text)
	default:
		return workerIdleStyle.Render(text)
	}
}

// truncateLine truncates a string to at most maxWidth visible characters,
// appending an ellipsis if truncation occurred.
func truncateLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth > 1 {
		return string(runes[:maxWidth-1]) + "…"
	}
	return string(runes[:maxWidth])
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v WorkerView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll up/down one line"},
		{Key: "PgUp/PgDn", Description: "Scroll up/down one page"},
	}
}
