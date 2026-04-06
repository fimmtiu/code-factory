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

	workerIdleStyle     = lipgloss.NewStyle().Foreground(colourMuted).Inherit(workerStatusStyle)          // grey
	workerAwaitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Inherit(workerStatusStyle)  // red
	workerBusyStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("22")).Inherit(workerStatusStyle) // dark green
	workerPausedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Inherit(workerStatusStyle)  // yellow

	workerOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")) // dim grey for output lines

	workerNoOutputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("250")) // light grey

	workerNewLineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("255")).Bold(true) // bright white for newly arrived lines
)

// ── Messages ─────────────────────────────────────────────────────────────────

type workerTickMsg struct{}

// ── WorkerView ────────────────────────────────────────────────────────────────

// WorkerView is a read-only full-screen pane that shows each worker's status
// and the last three lines of agent output. It supports scrolling but has no
// selectable item.
type WorkerView struct {
	pool            *worker.Pool
	width           int
	height          int
	offset          int               // first visible line (scroll offset)
	prevOutput      map[int][]string  // last-seen output slice per worker number
	outputChangedAt map[int]time.Time // when each worker's output last changed
}

// NewWorkerView creates a WorkerView backed by the given worker pool.
func NewWorkerView(pool *worker.Pool) WorkerView {
	return WorkerView{
		pool:            pool,
		prevOutput:      make(map[int][]string),
		outputChangedAt: make(map[int]time.Time),
	}
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
		// Detect output changes and record timestamps for highlight flash.
		if v.pool != nil {
			for _, w := range v.pool.Workers {
				current := w.GetLastOutput()
				prev := v.prevOutput[w.Number]
				changed := len(current) != len(prev)
				if !changed && len(current) > 0 {
					changed = current[len(current)-1] != prev[len(prev)-1]
				}
				if changed {
					v.outputChangedAt[w.Number] = time.Now()
					snapshot := make([]string, len(current))
					copy(snapshot, current)
					v.prevOutput[w.Number] = snapshot
				}
			}
		}
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

// totalLines returns the total rendered line count for all workers, accounting
// for word-wrapped output lines whose height varies with content and pane width.
func (v WorkerView) totalLines() int {
	if v.pool == nil {
		return 0
	}
	return len(v.renderAllLines())
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v WorkerView) View() string {
	if v.pool == nil || len(v.pool.Workers) == 0 {
		return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.viewHeight()).Render(emptyStateStyle.Render("(no workers)"))
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

	content := strings.Join(all[start:end], "\n")
	return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(vh).Render(clipLines(content, vh))
}

// renderAllLines builds the full list of display lines for every worker.
func (v WorkerView) renderAllLines() []string {
	innerW := v.width - viewBorderOverhead
	if innerW <= 0 {
		innerW = 1
	}
	separator := lipgloss.NewStyle().Foreground(colourMuted).Render(strings.Repeat("╌", innerW))

	var lines []string
	for _, w := range v.pool.Workers {
		// Status line: "<N>: <status>"
		statusLine := v.renderStatusLine(w)
		lines = append(lines, statusLine)

		// Wrap all raw output lines and take the last 3 display lines so
		// each worker section is always exactly 3 output lines tall.
		output := w.GetLastOutput()
		highlight := time.Since(v.outputChangedAt[w.Number]) < 600*time.Millisecond

		var displayLines []string
		for _, rawLine := range output {
			for _, dl := range wrapLine(rawLine, innerW) {
				displayLines = append(displayLines, dl)
			}
		}
		if len(displayLines) > 3 {
			displayLines = displayLines[len(displayLines)-3:]
		}
		lastContentIdx := len(displayLines) - 1 // index of last real line; -1 if none
		for len(displayLines) < 3 {
			displayLines = append(displayLines, "")
		}

		for i, dl := range displayLines {
			switch {
			case dl == "" && i == 1 && len(output) == 0:
				lines = append(lines, workerNoOutputStyle.Render("(no output)"))
			case dl == "":
				lines = append(lines, "")
			case highlight && i == lastContentIdx:
				lines = append(lines, workerNewLineStyle.Render(dl))
			default:
				lines = append(lines, workerOutputStyle.Render(dl))
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

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v WorkerView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll up/down one line"},
		{Key: "PgUp/PgDn", Description: "Scroll up/down one page"},
	}
}
