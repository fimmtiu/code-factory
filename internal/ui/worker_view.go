package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/ui/theme"
	"github.com/fimmtiu/code-factory/internal/worker"
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
		return theme.Current().ViewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.viewHeight()).Render(theme.Current().EmptyStateStyle.Render("(no workers)"))
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
	return theme.Current().ViewPaneStyle.Width(v.width - viewBorderOverhead).Height(vh).Render(clipLines(content, vh))
}

// renderAllLines builds the full list of display lines for every worker.
func (v WorkerView) renderAllLines() []string {
	innerW := v.width - viewBorderOverhead
	if innerW <= 0 {
		innerW = 1
	}
	separator := theme.Current().SeparatorStyle.Render(strings.Repeat("─", innerW))

	var lines []string
	for _, w := range v.pool.Workers {
		// Status line: "<N>: <status>"
		statusLine := v.renderStatusLine(w)
		lines = append(lines, statusLine)

		// Wrap all raw output lines and take the last OutputLines display lines so
		// each worker section is always exactly OutputLines output lines tall.
		output := w.GetLastOutput()
		highlight := time.Since(v.outputChangedAt[w.Number]) < 600*time.Millisecond

		var displayLines []string
		for _, rawLine := range output {
			for _, dl := range wrapLine(rawLine, innerW) {
				displayLines = append(displayLines, dl)
			}
		}
		if len(displayLines) > worker.OutputLines {
			displayLines = displayLines[len(displayLines)-worker.OutputLines:]
		}
		lastContentIdx := len(displayLines) - 1 // index of last real line; -1 if none
		for len(displayLines) < worker.OutputLines {
			displayLines = append(displayLines, "")
		}

		for i, dl := range displayLines {
			switch {
			case dl == "" && i == 1 && len(output) == 0:
				lines = append(lines, theme.Current().WorkerNoOutputStyle.Render("(no output)"))
			case dl == "":
				lines = append(lines, "")
			case highlight && i == lastContentIdx:
				lines = append(lines, theme.Current().WorkerNewLineStyle.Render(dl))
			default:
				lines = append(lines, theme.Current().WorkerOutputStyle.Render(dl))
			}
		}

		// Separator line.
		lines = append(lines, separator)
	}
	return lines
}

// lastActivityThreshold is the minimum inactivity duration before the "Xs ago"
// label appears in the Workers view. Very recent timestamps are hidden to
// avoid visual noise when the agent is actively streaming.
const lastActivityThreshold = 10 * time.Second

// formatLastActivity formats a duration as a compact relative timestamp.
func formatLastActivity(since time.Duration) string {
	switch {
	case since < time.Minute:
		return fmt.Sprintf("%ds ago", int(since.Seconds()))
	case since < time.Hour:
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	}
}

// renderStatusLine returns the styled status line for a worker.
func (v WorkerView) renderStatusLine(w *worker.Worker) string {
	var text string
	switch {
	case w.Paused:
		text = fmt.Sprintf("Worker %d: paused", w.Number)
	case w.GetCurrentTicket() != "":
		text = fmt.Sprintf("Worker %d: %s", w.Number, w.GetCurrentTicket())
	default:
		text = fmt.Sprintf("Worker %d: %s", w.Number, w.Status.String())
	}

	// Build an activity/timing suffix for non-idle, non-paused workers.
	var suffix string
	if w.Status != worker.StatusIdle && !w.Paused {
		if activity := w.GetActivity(); activity != "" {
			suffix += " · " + activity
		}
		if lastActivity := w.GetLastActivityAt(); !lastActivity.IsZero() {
			if since := time.Since(lastActivity); since >= lastActivityThreshold {
				suffix += " · " + formatLastActivity(since)
			}
		}
	}

	var styled string
	switch {
	case w.Paused:
		styled = theme.Current().WorkerPausedStyle.Render(text)
	case w.Status == worker.StatusIdle:
		styled = theme.Current().WorkerIdleStyle.Render(text)
	case w.Status == worker.StatusAwaitingResponse:
		styled = theme.Current().WorkerAwaitingStyle.Render(text)
	case w.Status == worker.StatusBusy:
		styled = theme.Current().WorkerBusyStyle.Render(text)
	default:
		styled = theme.Current().WorkerIdleStyle.Render(text)
	}

	if suffix != "" {
		styled += theme.Current().WorkerOutputStyle.Render(suffix)
	}
	return styled
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v WorkerView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Scroll up/down one line"},
		{Key: "PgUp/PgDn", Description: "Scroll up/down one page"},
	}
}

func (v WorkerView) Label() string { return "F3:Workers" }
