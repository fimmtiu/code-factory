package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"os/exec"

	"github.com/fimmtiu/tickets/internal/db"
	"github.com/fimmtiu/tickets/internal/models"
	"github.com/fimmtiu/tickets/internal/storage"
	"github.com/fimmtiu/tickets/internal/util"
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	crDismissedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240")) // grey
	crOpenStyle      = lipgloss.NewStyle()                                   // default
	crClosedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("22"))  // dark green
	crSelectedStyle  = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("230"))
	crPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
	crFocusedPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(lipgloss.Color("12"))
)

// ── ChangeRequestDialog ────────────────────────────────────────────────────────

// ChangeRequestDialog is a modal dialog showing change requests for a ticket.
type ChangeRequestDialog struct {
	database   *db.DB
	identifier string
	worktree   string // path to the ticket's worktree

	// Sorted slice of change requests (most recent first)
	changeRequests []models.ChangeRequest

	// List pane state
	selected int
	offset   int
	listW    int
	listH    int

	// Contents pane state
	contentsW int
	contentsH int

	// Total dialog dimensions (set by the caller to fill available space)
	width  int
	height int
}

// NewChangeRequestDialog creates a new ChangeRequestDialog for the given ticket.
func NewChangeRequestDialog(database *db.DB, wu *models.WorkUnit, width, height int) *ChangeRequestDialog {
	// Sort change requests: most recent first
	crs := make([]models.ChangeRequest, len(wu.ChangeRequests))
	copy(crs, wu.ChangeRequests)
	sort.Slice(crs, func(i, j int) bool {
		return crs[i].Date.After(crs[j].Date)
	})

	worktree, _ := storage.WorktreePathForIdentifier(wu.Identifier)

	d := &ChangeRequestDialog{
		database:       database,
		identifier:     wu.Identifier,
		worktree:       worktree,
		changeRequests: crs,
		width:          width,
		height:         height,
	}
	d.computeDimensions()
	return d
}

// computeDimensions recalculates pane widths and heights from d.width/d.height
// (which hold the full terminal dimensions passed in from the root model).
//
// Overhead chain (horizontal):
//
//	dialogBoxStyle: border(1+1) + padding(2+2) = 6
//	two crPaneStyle borders side by side: 2+2 = 4
//	total to subtract from terminal width: marginH(8) + 6 + 4 = 18
//
// Overhead chain (vertical):
//
//	dialogBoxStyle: border(1+1) + padding(1+1) = 4
//	crPaneStyle top+bottom border: 2
//	fixed rows: title(1) + dialogTitleStyle MarginBottom(1) + hint(1) = 3
//	total to subtract from terminal height: marginV(4) + 4 + 2 + 3 = 13
func (d *ChangeRequestDialog) computeDimensions() {
	const (
		marginH      = 8  // terminal margin left+right
		marginV      = 4  // terminal margin top+bottom
		dlgOverheadH = 10 // dialogBoxStyle(6) + two pane borders(4)
		dlgOverheadV = 9  // dialogBoxStyle(4) + pane borders(2) + fixed rows(3)
	)

	// Outer dialog dimensions.
	d.width = d.width - marginH
	d.height = d.height - marginV
	if d.width < 40 {
		d.width = 40
	}
	if d.height < 15 {
		d.height = 15
	}

	// Pane content dimensions.
	availW := d.width - dlgOverheadH
	availH := d.height - dlgOverheadV

	if availW < 4 {
		availW = 4
	}
	if availH < 2 {
		availH = 2
	}

	d.listW = availW * 30 / 100
	if d.listW < 15 {
		d.listW = 15
	}
	d.contentsW = availW - d.listW
	if d.contentsW < 10 {
		d.contentsW = 10
	}

	d.listH = availH
	d.contentsH = availH
}

func (d *ChangeRequestDialog) Init() tea.Cmd { return nil }

func (d *ChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case crStatusChangedMsg:
		d.handleStatusChanged(msg)
		return d, nil

	case crDescriptionUpdatedMsg:
		d.handleDescriptionUpdated(msg)
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "up":
			if d.selected > 0 {
				d.selected--
				d.clampScroll()
			}

		case "down":
			if d.selected < len(d.changeRequests)-1 {
				d.selected++
				d.clampScroll()
			}

		case "pgup":
			d.selected -= d.listH
			if d.selected < 0 {
				d.selected = 0
			}
			d.clampScroll()

		case "pgdown":
			d.selected += d.listH
			if d.selected >= len(d.changeRequests) {
				d.selected = len(d.changeRequests) - 1
			}
			if d.selected < 0 {
				d.selected = 0
			}
			d.clampScroll()

		case "x", "X":
			return d, d.dismissSelected()

		case "o", "O":
			return d, d.reopenSelected()

		case "e", "E":
			return d, d.editDescription()
		}
	}
	return d, nil
}

func (d *ChangeRequestDialog) clampScroll() {
	if d.listH <= 0 || len(d.changeRequests) == 0 {
		return
	}
	if d.selected < d.offset {
		d.offset = d.selected
	}
	if d.selected >= d.offset+d.listH {
		d.offset = d.selected - d.listH + 1
	}
	maxOffset := len(d.changeRequests) - d.listH
	if maxOffset < 0 {
		maxOffset = 0
	}
	if d.offset > maxOffset {
		d.offset = maxOffset
	}
	if d.offset < 0 {
		d.offset = 0
	}
}

// ── Action commands ────────────────────────────────────────────────────────────

type crStatusChangedMsg struct {
	idx    int
	status string
}

type crDescriptionUpdatedMsg struct {
	idx         int
	description string
}

func (d *ChangeRequestDialog) selectedCR() *models.ChangeRequest {
	if len(d.changeRequests) == 0 || d.selected >= len(d.changeRequests) {
		return nil
	}
	cr := &d.changeRequests[d.selected]
	return cr
}

func (d *ChangeRequestDialog) dismissSelected() tea.Cmd {
	cr := d.selectedCR()
	if cr == nil {
		return nil
	}
	id, err := strconv.ParseInt(cr.ID, 10, 64)
	if err != nil {
		return nil
	}
	idx := d.selected
	database := d.database
	return func() tea.Msg {
		if err := database.DismissChangeRequest(id); err != nil {
			return nil
		}
		return crStatusChangedMsg{idx: idx, status: models.ChangeRequestDismissed}
	}
}

func (d *ChangeRequestDialog) reopenSelected() tea.Cmd {
	cr := d.selectedCR()
	if cr == nil {
		return nil
	}
	id, err := strconv.ParseInt(cr.ID, 10, 64)
	if err != nil {
		return nil
	}
	idx := d.selected
	database := d.database
	return func() tea.Msg {
		if err := database.ReopenChangeRequest(id); err != nil {
			return nil
		}
		return crStatusChangedMsg{idx: idx, status: models.ChangeRequestOpen}
	}
}

func (d *ChangeRequestDialog) editDescription() tea.Cmd {
	cr := d.selectedCR()
	if cr == nil {
		return nil
	}
	id, err := strconv.ParseInt(cr.ID, 10, 64)
	if err != nil {
		return nil
	}
	currentDesc := cr.Description
	idx := d.selected
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

// ── Handle internal messages ───────────────────────────────────────────────────

func (d *ChangeRequestDialog) handleStatusChanged(msg crStatusChangedMsg) {
	if msg.idx >= 0 && msg.idx < len(d.changeRequests) {
		d.changeRequests[msg.idx].Status = msg.status
	}
}

func (d *ChangeRequestDialog) handleDescriptionUpdated(msg crDescriptionUpdatedMsg) {
	if msg.idx >= 0 && msg.idx < len(d.changeRequests) {
		d.changeRequests[msg.idx].Description = msg.description
	}
}

// View renders the dialog.
func (d *ChangeRequestDialog) View() string {
	title := dialogTitleStyle.Render(fmt.Sprintf("Change requests for `%s`", d.identifier))

	if len(d.changeRequests) == 0 {
		body := lipgloss.JoinVertical(lipgloss.Left,
			title,
			"(no change requests)",
		)
		return dialogBoxStyle.Render(body)
	}

	listPane := d.renderListPane()
	contentsPane := d.renderContentsPane()

	panes := lipgloss.JoinHorizontal(lipgloss.Top, listPane, contentsPane)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("X dismiss  O reopen  E edit  Esc close")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		panes,
		hint,
	)
	// dialogBoxStyle adds a 1-cell border on each side, so Width(d.width-2) produces
	// a total rendered width of d.width.
	return dialogBoxStyle.Width(d.width - 2).Render(body)
}

func (d *ChangeRequestDialog) renderListPane() string {
	var sb strings.Builder
	end := d.offset + d.listH
	if end > len(d.changeRequests) {
		end = len(d.changeRequests)
	}
	for i := d.offset; i < end; i++ {
		cr := d.changeRequests[i]
		label := cr.Date.Local().Format("01/02 15:04") + " " + cr.Author
		runes := []rune(label)
		if len(runes) > d.listW {
			if d.listW > 1 {
				label = string(runes[:d.listW-1]) + "…"
			} else {
				label = string(runes[:d.listW])
			}
		}

		var styled string
		if i == d.selected {
			styled = crSelectedStyle.Width(d.listW).Render(label)
		} else {
			switch cr.Status {
			case models.ChangeRequestDismissed:
				styled = crDismissedStyle.Render(label)
			case models.ChangeRequestClosed:
				styled = crClosedStyle.Render(label)
			default:
				styled = crOpenStyle.Render(label)
			}
		}
		sb.WriteString(styled)
		if i < end-1 {
			sb.WriteString("\n")
		}
	}

	inner := sb.String()
	return crPaneStyle.Width(d.listW).Height(d.listH).Render(inner)
}

func (d *ChangeRequestDialog) renderContentsPane() string {
	if len(d.changeRequests) == 0 {
		return crPaneStyle.Width(d.contentsW).Height(d.contentsH).Render("")
	}

	cr := d.changeRequests[d.selected]

	filename, lineNumber := parseCodeLocationForDisplay(cr.CodeLocation)

	var sb strings.Builder
	sb.WriteString(detailLabelStyle.Render("File:") + " " + filename + "\n")
	sb.WriteString(detailLabelStyle.Render("Line:") + " " + strconv.Itoa(lineNumber) + "\n")
	sb.WriteString(detailLabelStyle.Render("Status:") + " " + cr.Status + "\n")
	sb.WriteString("\n")

	codeCtx := d.fetchCodeContext(cr.CommitHash, filename, lineNumber)
	sb.WriteString(detailLabelStyle.Render("Code:") + "\n")
	sb.WriteString(codeCtx)
	sb.WriteString("\n\n")
	sb.WriteString(detailLabelStyle.Render("Description:") + "\n")
	sb.WriteString(cr.Description)

	// Truncate content to d.contentsH lines — lipgloss Height() is a minimum,
	// not a maximum, so we must limit lines explicitly to prevent overflow.
	lines := strings.Split(sb.String(), "\n")
	if len(lines) > d.contentsH {
		lines = lines[:d.contentsH]
	}
	return crPaneStyle.Width(d.contentsW).Height(d.contentsH).Render(strings.Join(lines, "\n"))
}

// parseCodeLocationForDisplay splits "file.go:42" → ("file.go", 42).
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

// fetchCodeContext reads the file at the given commit hash from the worktree
// and returns 3 lines of context around lineNumber (1-based).
func (d *ChangeRequestDialog) fetchCodeContext(commitHash, filename string, lineNumber int) string {
	if commitHash == "" {
		return "(Code context unavailable)"
	}

	cmd := exec.Command("git", "show", commitHash+":"+filename)
	cmd.Dir = d.worktree
	out, err := cmd.Output()
	if err != nil {
		return "(Code context unavailable)"
	}

	lines := strings.Split(string(out), "\n")
	targetIdx := lineNumber - 1 // lineNumber is 1-based
	if targetIdx < 0 {
		targetIdx = 0
	}

	start := targetIdx - 3
	if start < 0 {
		start = 0
	}
	end := targetIdx + 4
	if end > len(lines) {
		end = len(lines)
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		lineNum := i + 1
		marker := "  "
		if i == targetIdx {
			marker = "> "
		}
		sb.WriteString(fmt.Sprintf("%s%4d | %s\n", marker, lineNum, lines[i]))
	}
	return strings.TrimRight(sb.String(), "\n")
}
