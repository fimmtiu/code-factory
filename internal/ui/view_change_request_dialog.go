package ui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── Messages ────────────────────────────────────────────────────────────────

// openViewChangeRequestDialogMsg asks the root model to open the view CR dialog.
type openViewChangeRequestDialogMsg struct {
	cr           models.ChangeRequest
	identifier   string
	worktreePath string
}

// viewCRStatusToggledMsg is sent when the CR status toggle completes.
type viewCRStatusToggledMsg struct {
	status string
}

// ── ViewChangeRequestDialog ─────────────────────────────────────────────────

// ViewChangeRequestDialog is the read-only modal that displays a single
// change request's details with scrollable description.
type ViewChangeRequestDialog struct {
	database      *db.DB
	cr            models.ChangeRequest
	identifier    string
	worktreePath  string
	fileName      string
	lineNum       int
	codeContext   string
	descLines     []string
	descOffset    int
	width         int
	contentHeight int
}

// NewViewChangeRequestDialog creates a ViewChangeRequestDialog for the given CR.
func NewViewChangeRequestDialog(database *db.DB, cr models.ChangeRequest, identifier, worktreePath string, width int) *ViewChangeRequestDialog {
	fileName, lineNum := parseCodeLocationForDisplay(cr.CodeLocation)
	codeContext := fetchCodeContext(worktreePath, cr.CommitHash, fileName, lineNum)

	contentWidth := dialogContentWidth(width)

	var descLines []string
	for _, line := range strings.Split(cr.Description, "\n") {
		descLines = append(descLines, wrapLine(line, contentWidth)...)
	}

	return &ViewChangeRequestDialog{
		database:      database,
		cr:            cr,
		identifier:    identifier,
		worktreePath:  worktreePath,
		fileName:      fileName,
		lineNum:       lineNum,
		codeContext:   codeContext,
		descLines:     descLines,
		width:         width,
		contentHeight: 8,
	}
}

func (d *ViewChangeRequestDialog) Init() tea.Cmd { return nil }

func (d *ViewChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case viewCRStatusToggledMsg:
		d.cr.Status = msg.status
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()
		case "enter":
			return d, dismissDialogCmd()

		case "up":
			if d.descOffset > 0 {
				d.descOffset--
			}
			return d, nil

		case "down":
			d.descOffset++
			d.clampDescScroll()
			return d, nil

		case "pgup":
			d.descOffset -= d.contentHeight
			if d.descOffset < 0 {
				d.descOffset = 0
			}
			return d, nil

		case "pgdown":
			d.descOffset += d.contentHeight
			d.clampDescScroll()
			return d, nil

		case "x":
			return d, d.toggleStatus()
		}
	}
	return d, nil
}

func (d *ViewChangeRequestDialog) toggleStatus() tea.Cmd {
	id, err := strconv.ParseInt(d.cr.ID, 10, 64)
	if err != nil {
		return nil
	}

	var dbAction func(*db.DB, int64) error
	var newStatus string
	if d.cr.Status == models.ChangeRequestOpen {
		dbAction = func(database *db.DB, crID int64) error { return database.DismissChangeRequest(crID) }
		newStatus = models.ChangeRequestDismissed
	} else {
		dbAction = func(database *db.DB, crID int64) error { return database.ReopenChangeRequest(crID) }
		newStatus = models.ChangeRequestOpen
	}

	database := d.database
	return func() tea.Msg {
		if database != nil {
			if err := dbAction(database, id); err != nil {
				return nil
			}
		}
		return viewCRStatusToggledMsg{status: newStatus}
	}
}

func dialogContentWidth(totalWidth int) int {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	w := totalWidth - dialogPad
	if w < 20 {
		w = 20
	}
	return w
}

func (d *ViewChangeRequestDialog) clampDescScroll() {
	maxOffset := len(d.descLines) - d.contentHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if d.descOffset > maxOffset {
		d.descOffset = maxOffset
	}
	if d.descOffset < 0 {
		d.descOffset = 0
	}
}

func (d *ViewChangeRequestDialog) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Current().DialogTitleStyle.Render("Change Request"))
	sb.WriteString("\n")

	// File, line, and status info.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("File:") + " " + d.fileName)
	sb.WriteString("\n")
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Line:") + " " + strconv.Itoa(d.lineNum))
	sb.WriteString("\n")
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Status:") + " " + d.cr.Status)
	sb.WriteString("\n\n")

	// Code context.
	contentWidth := dialogContentWidth(d.width)
	for _, line := range strings.Split(d.codeContext, "\n") {
		sb.WriteString(theme.Current().QuickResponseOutputStyle.Render(truncateLine(strings.TrimRight(line, " \t"), contentWidth)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Description.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Description:"))
	sb.WriteString("\n")
	end := d.descOffset + d.contentHeight
	if end > len(d.descLines) {
		end = len(d.descLines)
	}
	for i := d.descOffset; i < end; i++ {
		sb.WriteString(d.descLines[i])
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// OK button (always focused since it's the only button).
	sb.WriteString(theme.Current().ButtonFocusedStyle.Render("OK"))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}
