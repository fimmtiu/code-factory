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
				Background(colourPrimary).
				Foreground(colourOnPrimary)

	logWorkerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color("236"))
	case age < 5*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("239"))
	case age < 30*time.Minute:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	}
}

// Column widths for the three-column layout.
const (
	logTimestampWidth = 12 // "15:04:05" or "01/02 15:04"
	logWorkerWidth    = 2  // right-aligned worker number
	logWorkerGap      = 3  // space after worker number (before message)
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

	// Filter state
	filterText string
	filtering  bool
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
// When filtering is active, smart auto-scroll is skipped; only clamp.
func (v *LogView) adjustScrollAfterRefresh(oldCount, newCount int, wasOnLast bool) {
	if newCount == 0 {
		v.selected = 0
		v.offset = 0
		return
	}
	if v.filtering {
		// While filtering, just clamp to avoid out-of-bounds.
		filtered := v.filteredEntries()
		if v.selected >= len(filtered) {
			v.selected = max(0, len(filtered)-1)
		}
		v.clampScroll()
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

// filteredEntries returns the subset of entries matching filterText.
// When filterText is empty, the full slice is returned as-is.
func (v LogView) filteredEntries() []models.LogEntry {
	if v.filterText == "" {
		return v.entries
	}
	query := strings.ToLower(v.filterText)
	result := make([]models.LogEntry, 0, len(v.entries))
	for _, e := range v.entries {
		if strings.Contains(strings.ToLower(e.Message), query) {
			result = append(result, e)
		}
	}
	return result
}

// handleFilterInput processes a key press while filtering is active.
func (v LogView) handleFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	hadFilter := v.filterText != ""
	var cmd tea.Cmd

	switch msg.String() {
	case "esc":
		v.filtering = false
		v.filterText = ""
		v.selected = 0
		v.offset = 0
		if hadFilter {
			cmd = ShowNotification("Filter cleared")
		}
		return v, cmd
	case "backspace":
		runes := []rune(v.filterText)
		if len(runes) > 0 {
			v.filterText = string(runes[:len(runes)-1])
		}
	default:
		r := []rune(msg.String())
		if len(r) == 1 && r[0] >= 32 {
			v.filterText += string(r)
		}
	}

	v.selected = 0
	v.offset = 0

	if v.filterText != "" {
		cmd = ShowNotification(`Filtering to "` + v.filterText + `"`)
	}
	return v, cmd
}

func (v LogView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if v.filtering {
		return v.handleFilterInput(msg)
	}

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
	case "g":
		return v.terminalGitDiff()
	case "G":
		return v.githubCompare()
	case "/":
		v.filtering = true
		return v, nil
	}
	return v, nil
}

func (v *LogView) moveBy(delta int) {
	v.selected += delta
	if v.selected < 0 {
		v.selected = 0
	}
	if n := len(v.filteredEntries()); v.selected >= n && n > 0 {
		v.selected = n - 1
	}
	v.clampScroll()
}

func (v *LogView) clampScroll() {
	h := v.listHeight()
	entries := v.filteredEntries()
	if h <= 0 || len(entries) == 0 {
		v.offset = 0
		return
	}
	if v.selected < v.offset {
		v.offset = v.selected
	}
	if v.selected >= v.offset+h {
		v.offset = v.selected - h + 1
	}
	maxOffset := len(entries) - h
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
	entries := v.filteredEntries()
	if len(entries) == 0 || v.selected < 0 || v.selected >= len(entries) {
		return nil
	}
	return &entries[v.selected]
}

// openLogfile opens the selected entry's logfile in the blocking editor. The
// actual file path is opened directly (not a temp copy). Uses os/exec, not
// tea.ExecProcess.
func (v LogView) openLogfile() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}
	logfile := entry.Logfile
	return v, wrapEditorCmd(func() tea.Msg {
		_ = util.OpenFileInEditor(logfile)
		return nil
	})
}

func (v LogView) terminalGitDiff() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}
	identifier := identifierFromLogfile(entry.Logfile)
	if identifier == "" {
		return v, nil
	}
	openTerminalGitDiff(identifier)
	return v, nil
}

func (v LogView) githubCompare() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}
	identifier := identifierFromLogfile(entry.Logfile)
	if identifier == "" {
		return v, nil
	}
	openGitHubCompare(identifier)
	return v, nil
}

// copyLogfilePath copies the selected entry's logfile path to the clipboard.
func (v LogView) copyLogfilePath() (tea.Model, tea.Cmd) {
	entry := v.selectedEntry()
	if entry == nil || entry.Logfile == "" {
		return v, nil
	}
	_ = util.CopyToClipboard(entry.Logfile)
	return v, ShowNotification("Copied path to clipboard")
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
	used := logTimestampWidth + logColGap + logWorkerWidth + logWorkerGap + viewBorderOverhead
	w := v.width - used
	if w < 1 {
		w = 1
	}
	return w
}

// ── View ──────────────────────────────────────────────────────────────────────

func (v LogView) View() string {
	paneW := v.width - viewBorderOverhead
	entries := v.filteredEntries()

	if len(entries) == 0 {
		emptyMsg := "No log entries"
		if v.filterText != "" {
			emptyMsg = "No matching entries"
		}
		return viewPaneStyle.Width(paneW).Height(v.listHeight()).
			Render(lipgloss.Place(paneW, v.listHeight(), lipgloss.Center, lipgloss.Center, emptyStateStyle.Render(emptyMsg)))
	}

	h := v.listHeight()
	end := v.offset + h
	if end > len(entries) {
		end = len(entries)
	}

	var sb strings.Builder
	for i := v.offset; i < end; i++ {
		line := v.renderRow(&entries[i], i == v.selected)
		sb.WriteString(line)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return viewPaneStyle.Width(paneW).Height(h).Render(clipLines(sb.String(), h))
}

// renderRow formats one log entry as a three-column row.
func (v LogView) renderRow(e *models.LogEntry, selected bool) string {
	now := time.Now()
	ts := formatLogTimestamp(e.Timestamp, now)
	var workerStr string
	if e.WorkerNumber > 0 {
		workerStr = fmt.Sprintf("%*d", logWorkerWidth, e.WorkerNumber)
	} else {
		workerStr = strings.Repeat(" ", logWorkerWidth)
	}
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

	gap := strings.Repeat(" ", logWorkerGap)
	line := fmt.Sprintf("%-*s %s%s%s",
		logTimestampWidth, ts,
		workerStr,
		gap,
		msg,
	)

	if selected {
		return logSelectedStyle.Width(v.width - viewBorderOverhead).Render(line)
	}

	// For non-selected rows, colour the timestamp by age and the message
	// segment by message type for quick visual scanning.
	age := now.Sub(e.Timestamp)
	tsStyled := logTimestampStyle(age).Render(fmt.Sprintf("%-*s", logTimestampWidth, ts))
	rest := fmt.Sprintf(" %s%s%s", workerStr, gap, msg)
	msgStyle := lipgloss.NewStyle().Foreground(logMessageColor(e.Message))
	return tsStyled + msgStyle.Render(rest)
}

// logMessageColor returns the foreground colour for a log message based on its
// content, allowing quick visual scanning of the log by message type.
func logMessageColor(msg string) lipgloss.Color {
	switch {
	// [mock] variants first so they don't fall through to the general cases.
	case strings.HasPrefix(msg, "[mock] error"):
		return lipgloss.Color("88") // error — dark red
	case strings.HasPrefix(msg, "[mock] asking user"):
		return lipgloss.Color("94") // permission request — orange-brown
	case strings.HasPrefix(msg, "[mock] received response"):
		return lipgloss.Color("75") // permission response — soft blue
	case strings.HasPrefix(msg, "[mock] committed"):
		return lipgloss.Color("74") // commit — teal
	case strings.HasPrefix(msg, "claimed"):
		return lipgloss.Color("34") // claim — green
	case strings.HasPrefix(msg, "released"),
		strings.HasPrefix(msg, "housekeeping: released"):
		return lipgloss.Color("21") // release — blue
	case strings.HasPrefix(msg, "error"),
		strings.HasPrefix(msg, "ACP error"),
		strings.HasPrefix(msg, "housekeeping: error"):
		return lipgloss.Color("88") // error — dark red
	case strings.HasPrefix(msg, "permission request"):
		return lipgloss.Color("94") // permission request — orange-brown
	case strings.HasPrefix(msg, "permission response"):
		return lipgloss.Color("75") // permission response — soft blue
	default:
		return lipgloss.Color("246") // agent output — mid grey
	}
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
		{Key: "g", Description: "Open git diff in terminal"},
		{Key: "G", Description: "Open GitHub compare page", Hidden: !isGitHubRepo()},
		{Key: "/", Description: "Filter entries by substring"},
	}
}

func (v LogView) Label() string { return "F4:Log" }
