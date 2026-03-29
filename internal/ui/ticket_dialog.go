package ui

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
	"github.com/fimmtiu/code-factory/internal/util"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// openTicketDialogMsg requests the root model to open the unified ticket dialog.
type openTicketDialogMsg struct {
	wu *models.WorkUnit
}

// crStatusChangedMsg is sent when a change request's status is updated.
type crStatusChangedMsg struct {
	idx    int
	status string
}

// crDescriptionUpdatedMsg is sent when a change request's description is updated.
type crDescriptionUpdatedMsg struct {
	idx         int
	description string
}

// phaseOrder defines the display order and labels for ticket phases.
var phaseOrder = []struct {
	phase models.TicketPhase
	label string
}{
	{models.PhaseImplement, "Implement"},
	{models.PhaseRefactor, "Refactor"},
	{models.PhaseReview, "Review"},
	{models.PhaseRespond, "Respond"},
}

// wrapLine splits a single line into display lines of at most width runes,
// preferring to break at spaces, tabs, or after dashes. Falls back to a hard
// break only when no such boundary exists in the current segment.
func wrapLine(line string, width int) []string {
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	var result []string
	for len(runes) > width {
		cut, skipOne := width, false
		for i := width - 1; i >= 0; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				cut = i
				skipOne = true
				break
			}
			if runes[i] == '-' {
				cut = i + 1
				break
			}
		}
		result = append(result, string(runes[:cut]))
		runes = runes[cut:]
		if skipOne && len(runes) > 0 {
			runes = runes[1:]
		}
	}
	return append(result, string(runes))
}

// ── Item model ────────────────────────────────────────────────────────────────

type tdItemKind int

const (
	tdItemHeader    tdItemKind = iota // bold non-selectable section header
	tdItemSeparator                   // blank non-selectable row
	tdItemCR                          // selectable change-request entry
	tdItemLog                         // selectable logfile entry
)

type tdItem struct {
	kind    tdItemKind
	label   string // display text
	dataIdx int    // index into changeRequests or logEntries
}

func (it tdItem) selectable() bool {
	return it.kind == tdItemCR || it.kind == tdItemLog
}

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	tdSelectedStyle  = lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	tdDismissedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	tdClosedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("22"))
	tdSectionStyle   = lipgloss.NewStyle().Bold(true)
)

// ── TicketDialog ──────────────────────────────────────────────────────────────

type tdFocus int

const (
	tdListFocus    tdFocus = iota
	tdContentFocus tdFocus = iota
)

type tdLogEntry struct {
	label string
	phase models.TicketPhase
	path  string
	lines []string
}

// TicketDialog is a unified two-pane modal showing a ticket's change requests
// and logfiles. Only non-empty sections are shown.
type TicketDialog struct {
	wu             *models.WorkUnit
	database       *db.DB
	worktree       string
	changeRequests []models.ChangeRequest
	logEntries     []tdLogEntry

	items       []tdItem
	selectedIdx int // index into items, always points to a selectable row

	listOffset    int
	contentOffset int
	focus         tdFocus

	width, height      int
	listW, listH       int
	contentW, contentH int
}

func NewTicketDialog(database *db.DB, wu *models.WorkUnit, width, height int) *TicketDialog {
	// Sort CRs: most recent first.
	crs := make([]models.ChangeRequest, len(wu.ChangeRequests))
	copy(crs, wu.ChangeRequests)
	sort.Slice(crs, func(i, j int) bool { return crs[i].Date.After(crs[j].Date) })

	// Discover logfiles.
	var logs []tdLogEntry
	if repoRoot, err := storage.FindRepoRoot("."); err == nil {
		ticketsDir := storage.TicketsDirPath(repoRoot)
		for _, p := range phaseOrder {
			path := worker.LatestLogfilePath(ticketsDir, wu.Identifier, p.phase)
			if path == "" {
				continue
			}
			var lines []string
			if data, err := os.ReadFile(path); err == nil {
				lines = strings.Split(string(data), "\n")
			}
			logs = append(logs, tdLogEntry{label: p.label, phase: p.phase, path: path, lines: lines})
		}
	}

	worktree, _ := storage.WorktreePathForIdentifier(wu.Identifier)

	d := &TicketDialog{
		wu:             wu,
		database:       database,
		worktree:       worktree,
		changeRequests: crs,
		logEntries:     logs,
		width:          width,
		height:         height,
	}
	d.computeDimensions()
	d.buildItems()
	return d
}

func (d *TicketDialog) computeDimensions() {
	const (
		marginH      = 8
		marginV      = 4
		dlgOverheadH = 10
		dlgOverheadV = 9
	)
	d.width -= marginH
	d.height -= marginV
	if d.width < 40 {
		d.width = 40
	}
	if d.height < 15 {
		d.height = 15
	}
	availW := d.width - dlgOverheadH
	availH := d.height - dlgOverheadV
	if availW < 4 {
		availW = 4
	}
	if availH < 2 {
		availH = 2
	}
	d.listW = availW * 28 / 100
	if d.listW < 14 {
		d.listW = 14
	}
	d.contentW = availW - d.listW
	if d.contentW < 10 {
		d.contentW = 10
	}
	d.listH = availH
	d.contentH = availH
}

// buildItems constructs the flat item list, omitting empty sections entirely.
func (d *TicketDialog) buildItems() {
	d.items = nil
	hasCRs := len(d.changeRequests) > 0
	hasLogs := len(d.logEntries) > 0

	if hasCRs {
		d.items = append(d.items, tdItem{kind: tdItemHeader, label: "Change requests"})
		for i, cr := range d.changeRequests {
			label := cr.Date.Local().Format("01/02 15:04") + "  " + cr.Author
			d.items = append(d.items, tdItem{kind: tdItemCR, label: label, dataIdx: i})
		}
	}
	if hasCRs && hasLogs {
		d.items = append(d.items, tdItem{kind: tdItemSeparator})
	}
	if hasLogs {
		d.items = append(d.items, tdItem{kind: tdItemHeader, label: "Agent logs"})
		for i, e := range d.logEntries {
			d.items = append(d.items, tdItem{kind: tdItemLog, label: e.label, dataIdx: i})
		}
	}
	d.selectedIdx = d.firstSelectable()
}

func (d *TicketDialog) Init() tea.Cmd { return nil }

func (d *TicketDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case crStatusChangedMsg:
		if msg.idx >= 0 && msg.idx < len(d.changeRequests) {
			d.changeRequests[msg.idx].Status = msg.status
			d.buildItems() // refresh labels
		}
		return d, nil

	case crDescriptionUpdatedMsg:
		if msg.idx >= 0 && msg.idx < len(d.changeRequests) {
			d.changeRequests[msg.idx].Description = msg.description
		}
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "tab":
			if d.focus == tdListFocus {
				d.focus = tdContentFocus
			} else {
				d.focus = tdListFocus
			}

		case "up":
			if d.focus == tdListFocus {
				if prev := d.prevSelectable(d.selectedIdx); prev != d.selectedIdx {
					d.selectedIdx = prev
					d.contentOffset = 0
					d.clampListScroll()
				}
			} else {
				if d.contentOffset > 0 {
					d.contentOffset--
				}
			}

		case "down":
			if d.focus == tdListFocus {
				if next := d.nextSelectable(d.selectedIdx); next != d.selectedIdx {
					d.selectedIdx = next
					d.contentOffset = 0
					d.clampListScroll()
				}
			} else {
				d.contentOffset++
				d.clampContentScroll()
			}

		case "pgup":
			if d.focus == tdListFocus {
				for i := 0; i < d.listH; i++ {
					prev := d.prevSelectable(d.selectedIdx)
					if prev == d.selectedIdx {
						break
					}
					d.selectedIdx = prev
				}
				d.contentOffset = 0
				d.clampListScroll()
			} else {
				d.contentOffset -= d.contentH
				if d.contentOffset < 0 {
					d.contentOffset = 0
				}
			}

		case "pgdown":
			if d.focus == tdListFocus {
				for i := 0; i < d.listH; i++ {
					next := d.nextSelectable(d.selectedIdx)
					if next == d.selectedIdx {
						break
					}
					d.selectedIdx = next
				}
				d.contentOffset = 0
				d.clampListScroll()
			} else {
				d.contentOffset += d.contentH
				d.clampContentScroll()
			}

		case "x", "X":
			if item := d.currentItem(); item != nil && item.kind == tdItemCR {
				cr := d.changeRequests[item.dataIdx]
				if cr.Status != models.ChangeRequestDismissed {
					return d, d.dismissCR(item.dataIdx)
				}
			}

		case "o", "O":
			if item := d.currentItem(); item != nil && item.kind == tdItemCR {
				cr := d.changeRequests[item.dataIdx]
				if cr.Status != models.ChangeRequestOpen {
					return d, d.reopenCR(item.dataIdx)
				}
			}

		case "e", "E":
			if item := d.currentItem(); item != nil && item.kind == tdItemCR {
				return d, d.editCRDescription(item.dataIdx)
			}

		case "d", "D":
			if item := d.currentItem(); item != nil && item.kind == tdItemLog {
				e := d.logEntries[item.dataIdx]
				return d, debugPromptCmd(d.wu, e.phase, e.path)
			}
		}
	}
	return d, nil
}

// ── Navigation helpers ────────────────────────────────────────────────────────

func (d *TicketDialog) firstSelectable() int {
	for i, it := range d.items {
		if it.selectable() {
			return i
		}
	}
	return -1
}

func (d *TicketDialog) prevSelectable(from int) int {
	for i := from - 1; i >= 0; i-- {
		if d.items[i].selectable() {
			return i
		}
	}
	return from
}

func (d *TicketDialog) nextSelectable(from int) int {
	for i := from + 1; i < len(d.items); i++ {
		if d.items[i].selectable() {
			return i
		}
	}
	return from
}

func (d *TicketDialog) currentItem() *tdItem {
	if d.selectedIdx < 0 || d.selectedIdx >= len(d.items) {
		return nil
	}
	return &d.items[d.selectedIdx]
}

func (d *TicketDialog) clampListScroll() {
	if d.selectedIdx < d.listOffset {
		d.listOffset = d.selectedIdx
	}
	if d.selectedIdx >= d.listOffset+d.listH {
		d.listOffset = d.selectedIdx - d.listH + 1
	}
	maxOffset := len(d.items) - d.listH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if d.listOffset > maxOffset {
		d.listOffset = maxOffset
	}
	if d.listOffset < 0 {
		d.listOffset = 0
	}
}

func (d *TicketDialog) clampContentScroll() {
	lines := d.contentLines()
	maxOffset := len(lines) - d.contentH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if d.contentOffset > maxOffset {
		d.contentOffset = maxOffset
	}
	if d.contentOffset < 0 {
		d.contentOffset = 0
	}
}

// ── Action commands ───────────────────────────────────────────────────────────

func (d *TicketDialog) dismissCR(idx int) tea.Cmd {
	id, err := strconv.ParseInt(d.changeRequests[idx].ID, 10, 64)
	if err != nil {
		return nil
	}
	database := d.database
	return func() tea.Msg {
		if err := database.DismissChangeRequest(id); err != nil {
			return nil
		}
		return crStatusChangedMsg{idx: idx, status: models.ChangeRequestDismissed}
	}
}

func (d *TicketDialog) reopenCR(idx int) tea.Cmd {
	id, err := strconv.ParseInt(d.changeRequests[idx].ID, 10, 64)
	if err != nil {
		return nil
	}
	database := d.database
	return func() tea.Msg {
		if err := database.ReopenChangeRequest(id); err != nil {
			return nil
		}
		return crStatusChangedMsg{idx: idx, status: models.ChangeRequestOpen}
	}
}

func (d *TicketDialog) editCRDescription(idx int) tea.Cmd {
	id, err := strconv.ParseInt(d.changeRequests[idx].ID, 10, 64)
	if err != nil {
		return nil
	}
	currentDesc := d.changeRequests[idx].Description
	database := d.database
	return func() tea.Msg {
		newDesc, err := util.EditText(currentDesc)
		if err != nil {
			return nil
		}
		if err := database.UpdateChangeRequestDescription(id, newDesc); err != nil {
			return nil
		}
		return crDescriptionUpdatedMsg{idx: idx, description: newDesc}
	}
}

// ── Content lines ─────────────────────────────────────────────────────────────

func (d *TicketDialog) contentLines() []string {
	item := d.currentItem()
	if item == nil {
		return nil
	}
	switch item.kind {
	case tdItemCR:
		return d.crContentLines(item.dataIdx)
	case tdItemLog:
		return d.logContentLines(item.dataIdx)
	}
	return nil
}

func (d *TicketDialog) crContentLines(idx int) []string {
	cr := d.changeRequests[idx]
	filename, lineNumber := parseCodeLocationForDisplay(cr.CodeLocation)
	codeCtx := fetchCodeContext(d.worktree, cr.CommitHash, filename, lineNumber)

	raw := strings.Join([]string{
		detailLabelStyle.Render("File:") + " " + filename,
		detailLabelStyle.Render("Line:") + " " + strconv.Itoa(lineNumber),
		detailLabelStyle.Render("Status:") + " " + cr.Status,
		"",
		detailLabelStyle.Render("Code:"),
		codeCtx,
		"",
		detailLabelStyle.Render("Description:"),
		cr.Description,
	}, "\n")

	var result []string
	for _, line := range strings.Split(raw, "\n") {
		result = append(result, wrapLine(line, d.contentW)...)
	}
	return result
}

func (d *TicketDialog) logContentLines(idx int) []string {
	var result []string
	for _, line := range d.logEntries[idx].lines {
		result = append(result, wrapLine(line, d.contentW)...)
	}
	return result
}

// ── View ──────────────────────────────────────────────────────────────────────

func (d *TicketDialog) View() string {
	title := dialogTitleStyle.Render(fmt.Sprintf("Ticket: `%s`", d.wu.Identifier))

	if len(d.items) == 0 {
		return dialogBoxStyle.Width(d.width - 2).Render(
			lipgloss.JoinVertical(lipgloss.Left, title, "(no change requests or logfiles)"),
		)
	}

	listBorderStyle := unfocusedBorderStyle
	contentBorderStyle := unfocusedBorderStyle
	if d.focus == tdListFocus {
		listBorderStyle = focusedBorderStyle
	} else {
		contentBorderStyle = focusedBorderStyle
	}

	hint := d.renderHint()

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.JoinHorizontal(lipgloss.Top,
			d.renderListPane(listBorderStyle),
			d.renderContentPane(contentBorderStyle),
		),
		hint,
	)
	return dialogBoxStyle.Width(d.width - 2).Render(body)
}

var (
	hintActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	hintInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
)

func (d *TicketDialog) renderHint() string {
	item := d.currentItem()
	base := hintActiveStyle.Render("  Tab switch  Esc close")
	if item == nil {
		return hintActiveStyle.Render("Tab switch  Esc close")
	}
	switch item.kind {
	case tdItemCR:
		cr := d.changeRequests[item.dataIdx]
		xStyle := hintActiveStyle
		if cr.Status == models.ChangeRequestDismissed {
			xStyle = hintInactiveStyle
		}
		oStyle := hintActiveStyle
		if cr.Status == models.ChangeRequestOpen {
			oStyle = hintInactiveStyle
		}
		return xStyle.Render("X dismiss") +
			hintActiveStyle.Render("  ") +
			oStyle.Render("O reopen") +
			hintActiveStyle.Render("  E edit") +
			base
	case tdItemLog:
		return hintActiveStyle.Render("D debug prompt") + base
	}
	return hintActiveStyle.Render("Tab switch  Esc close")
}

func (d *TicketDialog) renderListPane(borderStyle lipgloss.Style) string {
	var sb strings.Builder
	end := d.listOffset + d.listH
	if end > len(d.items) {
		end = len(d.items)
	}
	for i := d.listOffset; i < end; i++ {
		item := d.items[i]
		switch item.kind {
		case tdItemHeader:
			label := item.label
			if len([]rune(label)) > d.listW {
				label = string([]rune(label)[:d.listW])
			}
			sb.WriteString(tdSectionStyle.Render(label))

		case tdItemSeparator:
			// blank row

		case tdItemCR:
			label := item.label
			if len([]rune(label)) > d.listW {
				label = string([]rune(label)[:d.listW-1]) + "…"
			}
			if i == d.selectedIdx {
				sb.WriteString(tdSelectedStyle.Width(d.listW).Render(label))
			} else {
				cr := d.changeRequests[item.dataIdx]
				switch cr.Status {
				case models.ChangeRequestDismissed:
					sb.WriteString(tdDismissedStyle.Render(label))
				case models.ChangeRequestClosed:
					sb.WriteString(tdClosedStyle.Render(label))
				default:
					sb.WriteString(label)
				}
			}

		case tdItemLog:
			label := item.label
			if i == d.selectedIdx {
				sb.WriteString(tdSelectedStyle.Width(d.listW).Render(label))
			} else {
				sb.WriteString(label)
			}
		}
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return borderStyle.Width(d.listW).Height(d.listH).Render(sb.String())
}

func (d *TicketDialog) renderContentPane(borderStyle lipgloss.Style) string {
	lines := d.contentLines()
	end := d.contentOffset + d.contentH
	if end > len(lines) {
		end = len(lines)
	}
	var sb strings.Builder
	for i := d.contentOffset; i < end; i++ {
		sb.WriteString(lines[i])
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return borderStyle.Width(d.contentW).Height(d.contentH).Render(sb.String())
}

// ── Code context ──────────────────────────────────────────────────────────────

func parseCodeLocationForDisplay(loc string) (string, int) {
	idx := strings.LastIndex(loc, ":")
	if idx < 0 {
		return loc, 0
	}
	line, err := strconv.Atoi(loc[idx+1:])
	if err != nil {
		return loc, 0
	}
	return loc[:idx], line
}

func fetchCodeContext(worktree, commitHash, filename string, lineNumber int) string {
	if commitHash == "" {
		return "(Code context unavailable)"
	}
	cmd := exec.Command("git", "show", commitHash+":"+filename)
	cmd.Dir = worktree
	out, err := cmd.Output()
	if err != nil {
		return "(Code context unavailable)"
	}
	lines := strings.Split(string(out), "\n")
	targetIdx := lineNumber - 1
	if targetIdx < 0 {
		targetIdx = 0
	}
	start := max(0, targetIdx-3)
	end := min(len(lines), targetIdx+4)
	var sb strings.Builder
	for i := start; i < end; i++ {
		marker := "  "
		if i == targetIdx {
			marker = "> "
		}
		fmt.Fprintf(&sb, "%s%4d | %s\n", marker, i+1, lines[i])
	}
	return strings.TrimRight(sb.String(), "\n")
}
