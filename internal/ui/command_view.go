package ui

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/config"
	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
	"github.com/fimmtiu/code-factory/internal/util"
	"github.com/fimmtiu/code-factory/internal/worker"
	"github.com/fimmtiu/code-factory/internal/workflow"
)

// ── Styles ───────────────────────────────────────────────────────────────────

var (
	cmdSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230"))

	cmdNeedsAttentionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")) // orange

	cmdUserReviewStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("22")) // dark green
)

// ── Messages ─────────────────────────────────────────────────────────────────

type commandRefreshMsg struct {
	tickets []*models.WorkUnit
}

// ── listRow ───────────────────────────────────────────────────────────────────

// listRow represents one row in the command view list. If separator is true,
// the row is a blank non-selectable divider between the two status groups.
type listRow struct {
	wu        *models.WorkUnit
	separator bool
}

// ── CommandView ───────────────────────────────────────────────────────────────

// CommandView shows actionable tickets (needs-attention then user-review) and
// provides key bindings to respond (R), open terminal (T), open editor (E),
// and approve (A).
type CommandView struct {
	database *db.DB
	pool     *worker.Pool
	waitSecs int

	width  int
	height int

	rows     []listRow
	selected int // index into rows (never points at a separator)
	offset   int // first visible row

	errorMsg string // brief error shown in the status area
}

// NewCommandView creates a CommandView wired to the given database, worker
// pool, and poll interval.
func NewCommandView(database *db.DB, pool *worker.Pool, waitSecs int) CommandView {
	return CommandView{
		database: database,
		pool:     pool,
		waitSecs: waitSecs,
	}
}

// Init fetches initial data and schedules periodic refreshes.
func (v CommandView) Init() tea.Cmd {
	return tea.Batch(v.fetchCmd(), v.tickCmd())
}

// ── Commands ──────────────────────────────────────────────────────────────────

func (v CommandView) fetchCmd() tea.Cmd {
	return func() tea.Msg {
		tickets, err := v.database.ActionableTickets()
		if err != nil {
			tickets = nil
		}
		return commandRefreshMsg{tickets: tickets}
	}
}

func (v CommandView) tickCmd() tea.Cmd {
	d := time.Duration(v.waitSecs) * time.Second
	if d <= 0 {
		d = 5 * time.Second
	}
	return tea.Tick(d, func(time.Time) tea.Msg {
		return commandRefreshMsg{} // triggers fetch
	})
}

// ── Row building ──────────────────────────────────────────────────────────────

// buildRows converts the ordered ticket list into a slice of listRows with an
// optional separator between the two status groups.
func buildRows(tickets []*models.WorkUnit) []listRow {
	var rows []listRow
	var hasNA, hasUR bool
	for _, t := range tickets {
		if t.Status == models.StatusNeedsAttention {
			hasNA = true
		} else if t.Status == models.StatusUserReview {
			hasUR = true
		}
	}

	separatorInserted := false
	for _, t := range tickets {
		if !separatorInserted && hasNA && hasUR && t.Status == models.StatusUserReview {
			rows = append(rows, listRow{separator: true})
			separatorInserted = true
		}
		rows = append(rows, listRow{wu: t})
	}
	return rows
}

// ── Update ────────────────────────────────────────────────────────────────────

func (v CommandView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.clampScroll()
		return v, nil

	case commandRefreshMsg:
		if msg.tickets == nil {
			// Tick ping: fetch real data.
			return v, v.fetchCmd()
		}
		v.rows = buildRows(msg.tickets)
		// Clamp selection to a non-separator selectable row.
		v.clampSelected()
		v.clampScroll()
		return v, v.tickCmd()

	case approveResultMsg:
		if msg.err != nil {
			v.errorMsg = fmt.Sprintf("approve error: %s", msg.err)
			return v, nil
		}
		v.errorMsg = ""
		return v, v.fetchCmd()

	case tea.KeyMsg:
		return v.handleKey(msg)
	}

	return v, nil
}

func (v CommandView) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up":
		v.moveUp(1)
	case "down":
		v.moveDown(1)
	case "pgup":
		v.moveUp(v.listHeight())
	case "pgdown":
		v.moveDown(v.listHeight())
	case "enter":
		return v.openChangeRequestDialog()
	case "r", "R":
		return v.respondToAgent()
	case "t", "T":
		return v.openTerminal()
	case "e", "E":
		return v.openEditorNonblocking()
	case "a", "A":
		return v.approveTicket()
	case "d", "D":
		return v.debugTicket()
	}
	return v, nil
}

// ── Navigation ────────────────────────────────────────────────────────────────

func (v *CommandView) moveUp(n int) {
	for i := 0; i < n; i++ {
		v.selected--
		// Skip separators going upward.
		for v.selected >= 0 && v.rows[v.selected].separator {
			v.selected--
		}
		if v.selected < 0 {
			v.selected = 0
			// Land on first non-separator.
			for v.selected < len(v.rows) && v.rows[v.selected].separator {
				v.selected++
			}
			break
		}
	}
	v.clampScroll()
}

func (v *CommandView) moveDown(n int) {
	last := len(v.rows) - 1
	for i := 0; i < n; i++ {
		v.selected++
		// Skip separators going downward.
		for v.selected <= last && v.rows[v.selected].separator {
			v.selected++
		}
		if v.selected > last {
			// Land on last non-separator.
			v.selected = last
			for v.selected >= 0 && v.rows[v.selected].separator {
				v.selected--
			}
			if v.selected < 0 {
				v.selected = 0
			}
			break
		}
	}
	v.clampScroll()
}

func (v *CommandView) clampSelected() {
	if len(v.rows) == 0 {
		v.selected = 0
		return
	}
	if v.selected >= len(v.rows) {
		v.selected = len(v.rows) - 1
	}
	if v.selected < 0 {
		v.selected = 0
	}
	// If we land on a separator, move forward.
	for v.selected < len(v.rows) && v.rows[v.selected].separator {
		v.selected++
	}
	// If no non-separator found, search backward.
	if v.selected >= len(v.rows) {
		v.selected = len(v.rows) - 1
		for v.selected >= 0 && v.rows[v.selected].separator {
			v.selected--
		}
	}
	if v.selected < 0 {
		v.selected = 0
	}
}

func (v *CommandView) clampScroll() {
	h := v.listHeight()
	if h <= 0 || len(v.rows) == 0 {
		return
	}
	if v.selected < v.offset {
		v.offset = v.selected
	}
	if v.selected >= v.offset+h {
		v.offset = v.selected - h + 1
	}
	maxOffset := len(v.rows) - h
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

func (v CommandView) selectedTicket() *models.WorkUnit {
	if len(v.rows) == 0 || v.selected >= len(v.rows) {
		return nil
	}
	row := v.rows[v.selected]
	if row.separator {
		return nil
	}
	return row.wu
}

func (v CommandView) respondToAgent() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil || wu.Status != models.StatusNeedsAttention {
		return v, nil
	}

	response, err := util.EditText("")
	if err != nil || strings.TrimSpace(response) == "" {
		return v, nil
	}

	workerNum, err := strconv.Atoi(wu.ClaimedBy)
	if err != nil {
		return v, nil
	}
	w := v.pool.GetWorker(workerNum)
	if w == nil {
		return v, nil
	}

	w.ToWorker <- worker.MainToWorkerMessage{
		Kind:    worker.MsgResponse,
		Payload: response,
	}

	return v, v.fetchCmd()
}

func (v CommandView) openTerminal() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil {
		return v, nil
	}
	dir, err := storage.WorktreePathForIdentifier(wu.Identifier)
	if err != nil {
		return v, nil
	}
	_ = util.OpenTerminal(dir)
	return v, nil
}

func (v CommandView) openEditorNonblocking() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil {
		return v, nil
	}
	dir, err := storage.WorktreePathForIdentifier(wu.Identifier)
	if err != nil {
		return v, nil
	}
	_ = exec.Command(config.Current.NonblockingEditorCommand, dir).Start()
	return v, nil
}

func (v CommandView) openChangeRequestDialog() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil {
		return v, nil
	}
	// Fetch fresh data to get change requests populated, then only open the
	// dialog if there is at least one change request.
	database := v.database
	identifier := wu.Identifier
	return v, func() tea.Msg {
		units, err := database.Status()
		if err != nil {
			return nil
		}
		for _, u := range units {
			if u.Identifier == identifier && !u.IsProject && len(u.ChangeRequests) > 0 {
				return openChangeRequestDialogMsg{wu: u}
			}
		}
		return nil
	}
}

type approveResultMsg struct {
	err error
}

func (v CommandView) approveTicket() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil || wu.Status != models.StatusUserReview {
		return v, nil
	}

	database := v.database
	identifier := wu.Identifier
	return v, func() tea.Msg {
		err := workflow.Approve(database, identifier)
		return approveResultMsg{err: err}
	}
}

// ── Dimension helpers ─────────────────────────────────────────────────────────

// listHeight returns the number of visible rows in the list body.
func (v CommandView) listHeight() int {
	h := v.height - chromeHeight - viewBorderOverhead
	if h < 1 {
		h = 1
	}
	return h
}

// ── View ──────────────────────────────────────────────────────────────────────

var cmdErrorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")) // bright red

func (v CommandView) View() string {
	var sb strings.Builder

	if v.errorMsg != "" {
		sb.WriteString(cmdErrorStyle.Render(v.errorMsg))
		sb.WriteString("\n")
	}

	if len(v.rows) == 0 {
		return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.listHeight()).
			Render(lipgloss.Place(v.width-viewBorderOverhead, v.listHeight(), lipgloss.Center, lipgloss.Center, "No actionable tickets"))
	}

	h := v.listHeight()
	end := v.offset + h
	if end > len(v.rows) {
		end = len(v.rows)
	}

	for i := v.offset; i < end; i++ {
		row := v.rows[i]
		if row.separator {
			sb.WriteString("\n")
			continue
		}

		line := v.renderRow(row.wu, i == v.selected)
		sb.WriteString(line)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return viewPaneStyle.Width(v.width - viewBorderOverhead).Height(v.listHeight()).Render(sb.String())
}

// renderRow formats one ticket row in tabular style:
//
//	<identifier>  <status>  <N>m
//
// The identifier expands to fill available width; status and time are
// right-aligned.
func (v CommandView) renderRow(wu *models.WorkUnit, selected bool) string {
	// Right-hand side: "  <status>  <N>m"
	mins := int(time.Since(wu.LastUpdated).Minutes())
	if mins < 0 {
		mins = 0
	}
	right := fmt.Sprintf("  %s  %dm", wu.Status, mins)

	// Available width for identifier (subtract border overhead).
	availW := v.width - viewBorderOverhead - lipgloss.Width(right)
	if availW < 1 {
		availW = 1
	}

	// Truncate identifier if needed.
	id := wu.Identifier
	idRunes := []rune(id)
	if len(idRunes) > availW {
		if availW > 1 {
			id = string(idRunes[:availW-1]) + "…"
		} else {
			id = string(idRunes[:availW])
		}
	}

	// Pad identifier to fill available width.
	idW := lipgloss.Width(id)
	if idW < availW {
		id += strings.Repeat(" ", availW-idW)
	}

	line := id + right

	if selected {
		return cmdSelectedStyle.Width(v.width - viewBorderOverhead).Render(line)
	}
	switch wu.Status {
	case models.StatusNeedsAttention:
		return cmdNeedsAttentionStyle.Render(line)
	case models.StatusUserReview:
		return cmdUserReviewStyle.Render(line)
	}
	return line
}

func (v CommandView) debugTicket() (tea.Model, tea.Cmd) {
	wu := v.selectedTicket()
	if wu == nil {
		return v, nil
	}

	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return v, nil
	}
	ticketsDir := storage.TicketsDirPath(repoRoot)
	worktreePath := storage.TicketWorktreePathIn(ticketsDir, wu.Identifier)
	logfilePath := worker.LatestLogfilePath(ticketsDir, wu.Identifier, string(wu.Phase))

	template := buildDebugTemplate(wu, worktreePath, logfilePath)

	_, tmpPath, err := util.EditTextKeepFile(template)
	if err != nil || strings.TrimSpace(tmpPath) == "" {
		return v, nil
	}

	script := fmt.Sprintf(`tell application "iTerm2"
	tell current window
		set myNewTab to create tab with default profile
		tell current session of myNewTab
			write text "claude < %s"
		end tell
	end tell
end tell`, tmpPath)
	cmd := exec.Command("osascript")
	cmd.Stdin = strings.NewReader(script)
	_ = cmd.Start()

	return v, nil
}

// buildDebugTemplate returns a pre-filled debug prompt for the given ticket.
func buildDebugTemplate(wu *models.WorkUnit, worktreePath, logfilePath string) string {
	type phaseInfo struct {
		intro  string
		adjust string
	}
	info := map[models.TicketPhase]phaseInfo{
		models.PhaseImplement: {
			intro:  "a prompt to implement this ticket",
			adjust: "either the prompt or the ticket description",
		},
		models.PhaseRefactor: {
			intro:  "the /cf-refactor skill to refactor the recent changes on this ticket",
			adjust: "either the skill or the ticket description",
		},
		models.PhaseReview: {
			intro:  "the /cf-review skill to review the recent changes on this ticket",
			adjust: "either the skill or the ticket description",
		},
		models.PhaseRespond: {
			intro:  "the /cf-respond skill to apply change requests to this ticket",
			adjust: "either the skill or the ticket description",
		},
	}
	pi, ok := info[wu.Phase]
	if !ok {
		pi = info[models.PhaseImplement]
	}

	logRef := "(no logfile found)"
	if logfilePath != "" {
		logRef = "`" + logfilePath + "`"
	}

	return fmt.Sprintf("I ran %s:\n\nName: %s\nGit worktree: %s\nDescription:\n%s\n\nWhat I expected was ...\n\nWhat I got was ...\n\nRead the full prompt and agent output from %s, then tell me how I could adjust %s to make an agent more likely to do what I expect.\n",
		pi.intro, wu.Identifier, worktreePath, wu.Description, logRef, pi.adjust)
}

// ── KeyBindings ───────────────────────────────────────────────────────────────

func (v CommandView) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "↑/↓", Description: "Navigate list"},
		{Key: "PgUp/PgDn", Description: "Page navigate"},
		{Key: "Enter", Description: "Open change request dialog"},
		{Key: "R", Description: "Respond to agent (needs-attention tickets)"},
		{Key: "T", Description: "Open terminal in worktree"},
		{Key: "E", Description: "Open worktree in Cursor"},
		{Key: "A", Description: "Approve ticket (user-review tickets)"},
		{Key: "D", Description: "Debug prompt: open template in $EDITOR then launch claude"},
	}
}
