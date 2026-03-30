package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/util"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	logSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230"))

	logWorkerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	logMessageStyle = lipgloss.NewStyle()

	logFileIndicatorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")) // orange — indicates logfile is present
)

// logTimestampStyle returns a style that fades the timestamp colour based on
// how long ago the entry was created:
//
//	< 1 min  → bright white ("255")
//	1–5 min  → normal grey  ("252")
//	5–30 min → dimmer grey  ("245")
//	> 30 min → very dim grey ("238")
func logTimestampStyle(age time.Duration) lipgloss.Style {
	switch {
	case age < time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	case age < 5*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	case age < 30*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	}
}

// Column widths for the three-column layout.
const (
	logTimestampWidth = 14 // "15:04:05" or "01/02 15:04"
	logWorkerWidth    = 4  // right-aligned worker number
	logColGap         = 1  // space between columns
)

// ── Messages ─────────────────────────────────────────────────────────────────

// logRefreshMsg carries freshly-fetched log entries. When entries is nil, it
// is a tick ping that triggers a real fetch.
type logRefreshMsg struct {
	entries []models.LogEntry
}

// logActivatedMsg is sent by the root model when the user switches to the log
// view, triggering an immediate refresh.
type logActivatedMsg struct{}

// ── LogView ──────────────────────────────────────────────────────────────────

// LogView is a full-screen selectable list of log entries. It auto-refreshes
// every 3 seconds while active and implements smart auto-scroll.
type LogView struct {
	database *db.DB

	width  int
	height int

	entries  []models.LogEntry
	selected int // index into entries
	offset   int // first visible row
}

// NewLogView creates a LogView backed by the given database.
func NewLogView(database *db.DB) LogView {
	return LogView{database: database}
}

// Init schedules the first periodic refresh tick.
func (v LogView) Init() tea.Cmd {
	return tea.Batch(v.fetchCmd(), v.tickCmd())
}

// ── Commands ──────────────────────────────────────────────────────────────────

// fetchCmd loads log entries from the database.
func (v LogView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		entries, err := v.database.GetLogs()
		if err != nil {
			entries = nil
		}
		return logRefreshMsg{entries: entries}
	}
}

// tickCmd schedules a refresh tick every 3 seconds.
func (v LogView) tickCmd() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return logRefreshMsg{} // nil entries: trigger fetch
	})
}

// ── Update ────────────────────────────────────────────────────────────────────

func (v LogView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case logActivatedMsg:
		// Immediate refresh when the user switches to this view.
		return v, v.fetchCmd()

	case logRefreshMsg:
		if msg.entries == nil {
			// Tick ping: fetch real data.
			return v, v.fetchCmd()
		}
		oldCount := len(v.entries)
		wasOnLast := oldCount == 0 || v.selected == oldCount-1
		v.entries = msg.entries
		v.adjustScrollAfterRefresh(oldCount, len(v.entries), wasOnLast)
		// Schedule the next tick.
		return v, v.tickCmd()

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

// adjustScrollAfterRefresh implements the smart auto-scroll logic. If the user
// was on the last entry before the refresh, move the selection to the new last
// entry. Otherwise, preserve selection and scroll position.
func (v *LogView) adjustScrollAfterRefresh(oldCount, newCount int, wasOnLast bool) {
	if newCount == 0 {
		v.selected = 0
		v.offset = 0
		return
	}
	if wasOnLast {
		// Follow new entries: move to the new last entry.
		v.selected = newCount - 1
		v.clampScroll()
	} else {
		// Preserve the user's position: clamp selection in case entries shrank.
		if v.selected >= newCount {
			v.selected = newCount - 1
		}
		v.clampScroll()
	}
}

func (v LogView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		v.moveBy(-1)
	case "down":
		v.moveBy(1)
	case "pgup":
		v.moveBy(-v.listHeight())
	case "pgdown":
		v.moveBy(v.listHeight())
	case "e", "E":
		return v.openLogfile()
	case "c", "C":
		return v.copyLogfilePath()
	}
	return v, nil
}

func (v *LogView) moveBy(delta int) {
	v.selected += delta
	if v.selected < 0 {
		v.selected = 0
	}
	if n := len(v.entries); v.selected >= n && n > 0 {
		v.selected = n - 1
	}
	v.clampScroll()
}

func (v *LogView) clampScroll() {
	h := v.listHeight()
	if h <= 0 || len(v.entries) == 0 {
		v.offset = 0
		return
	}
	if v.selected < v.offset {
		v.offset = v.selected
	}
	if v.selected >= v.offset+h {
		v.offset = v.selected - h + 1
	}
	maxOffset := len(v.entries) - h
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.offset > maxOffset {
		v.offset = maxOffset
	}
	if v.offset < 0 {
		v.offset = 0
	}
}

// ── Action handlers ───────────────────────────────────────────────────────────

func (v LogView) selectedEntry() *models.LogEntry {
	if len(v.entries) == 0 || v.selected < 0 || v.selected >= len(v.entries) {
		return nil
	}
	return &v.entries[v.selected]
}

// openLogfile opens the selected entry's logfile in the blocking editor. The
// actual file path is opened directly (not a temp copy). Uses os/exec, not
// tea.ExecProcess.
func (v LogView) openLogfile() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}

	_ = util.OpenFileInEditor(entry.Logfile)

	return v, nil
}

// copyLogfilePath copies the selected entry's logfile path to the clipboard.
func (v LogView) copyLogfilePath() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}
	_ = util.CopyToClipboard(entry.Logfile)
	return v, nil
}

// ── Dimension helpers ─────────────────────────────────────────────────────────

// listHeight returns the number of visible rows in the list body.
func (v LogView) listHeight() int {
	h := v.height - chromeHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// messageWidth returns the width available for the message column.
func (v LogView) messageWidth() int {
	used := logTimestampWidth + logColGap + logWorkerWidth + logColGap + viewBorderOverhead
	w := v.width - used
	if w < 1 {
		w = 1
	}
	return w
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v LogView) View() string {
	paneW := v.width - viewBorderOverhead

	if len(v.entries) == 0 {
		return viewPaneStyle.Width(paneW).Height(v.listHeight()).
			Render(lipgloss.Place(paneW, v.listHeight(), lipgloss.Center, lipgloss.Center, "No log entries"))
	}

	h := v.listHeight()
	end := v.offset + h
	if end > len(v.entries) {
		end = len(v.entries)
	}

	var sb strings.Builder
	for i := v.offset; i < end; i++ {
		line := v.renderRow(&v.entries[i], i == v.selected)
		sb.WriteString(line)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return viewPaneStyle.Width(paneW).Height(v.listHeight()).Render(sb.String())
}

// renderRow formats one log entry as a three-column row.
func (v LogView) renderRow(e *models.LogEntry, selected bool) string {
	now := time.Now()
	ts := formatLogTimestamp(e.Timestamp, now)
	workerStr := fmt.Sprintf("%*d", logWorkerWidth, e.WorkerNumber)
	msgW := v.messageWidth()

	// Build the message segment. If the entry has a logfile, prepend a subtle marker.
	msg := e.Message
	if e.Logfile != "" {
		marker := "◆ "
		markerRunes := []rune(marker)
		msgW -= len(markerRunes)
		if msgW < 0 {
			msgW = 0
		}
		msg = marker + truncateLine(msg, msgW)
	} else {
		msg = truncateLine(msg, msgW)
	}

	line := fmt.Sprintf("%-*s %s %s",
		logTimestampWidth, ts,
		workerStr,
		msg,
	)

	if selected {
		return logSelectedStyle.Width(v.width - viewBorderOverhead).Render(line)
	}

	// For non-selected rows, colour the timestamp by age and compose with the
	// rest of the line (which may carry the logfile indicator style).
	age := now.Sub(e.Timestamp)
	tsStyled := logTimestampStyle(age).Render(fmt.Sprintf("%-*s", logTimestampWidth, ts))
	rest := fmt.Sprintf(" %s %s", workerStr, msg)
	if e.Logfile != "" {
		return tsStyled + logFileIndicatorStyle.Render(rest)
	}
	return tsStyled + rest
}

// formatLogTimestamp formats a timestamp compactly: "15:04:05" for today,
// "01/02 15:04" for older entries.
func formatLogTimestamp(ts, now time.Time) string {
	if ts.Year() == now.Year() && ts.YearDay() == now.YearDay() {
		return ts.Format("15:04:05")
	}
	return ts.Format("01/02 15:04")
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v LogView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate list"},
		{Key: "PgUp/PgDn", Description: "Page navigate"},
		{Key: "E", Description: "Open logfile in $EDITOR"},
		{Key: "C", Description: "Copy logfile path to clipboard"},
	}
}
