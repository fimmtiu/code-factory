package ui

import (
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
// change request's details with scrollable content.
type ViewChangeRequestDialog struct {
	database      *db.DB
	cr            models.ChangeRequest
	identifier    string
	worktreePath  string
	contentLines  []string
	contentOffset int
	width         int
	contentHeight int
}

// NewViewChangeRequestDialog creates a ViewChangeRequestDialog for the given CR.
func NewViewChangeRequestDialog(database *db.DB, cr models.ChangeRequest, identifier, worktreePath string, width int) *ViewChangeRequestDialog {
	contentWidth := dialogContentWidth(width)
	raw := crDetailLines(cr, worktreePath)

	var contentLines []string
	for _, line := range strings.Split(raw, "\n") {
		contentLines = append(contentLines, wrapLine(line, contentWidth)...)
	}

	return &ViewChangeRequestDialog{
		database:      database,
		cr:            cr,
		identifier:    identifier,
		worktreePath:  worktreePath,
		contentLines:  contentLines,
		width:         width,
		contentHeight: 8,
	}
}

func (d *ViewChangeRequestDialog) Init() tea.Cmd { return nil }

func (d *ViewChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case viewCRStatusToggledMsg:
		d.cr.Status = msg.status
		d.rebuildContentLines()
		return d, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()
		case "enter":
			return d, dismissDialogCmd()

		case "up":
			if d.contentOffset > 0 {
				d.contentOffset--
			}
			return d, nil

		case "down":
			d.contentOffset++
			d.clampContentScroll()
			return d, nil

		case "pgup":
			d.contentOffset -= d.contentHeight
			if d.contentOffset < 0 {
				d.contentOffset = 0
			}
			return d, nil

		case "pgdown":
			d.contentOffset += d.contentHeight
			d.clampContentScroll()
			return d, nil

		case "x":
			return d, d.toggleStatus()
		}
	}
	return d, nil
}

func (d *ViewChangeRequestDialog) toggleStatus() tea.Cmd {
	return toggleCRStatusCmd(d.database, d.cr, func(newStatus string) tea.Msg {
		return viewCRStatusToggledMsg{status: newStatus}
	})
}

func dialogContentWidth(totalWidth int) int {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	w := totalWidth - dialogPad
	if w < 20 {
		w = 20
	}
	return w
}

func (d *ViewChangeRequestDialog) rebuildContentLines() {
	contentWidth := dialogContentWidth(d.width)
	raw := crDetailLines(d.cr, d.worktreePath)

	d.contentLines = nil
	for _, line := range strings.Split(raw, "\n") {
		d.contentLines = append(d.contentLines, wrapLine(line, contentWidth)...)
	}
}

func (d *ViewChangeRequestDialog) clampContentScroll() {
	maxOffset := len(d.contentLines) - d.contentHeight
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

func (d *ViewChangeRequestDialog) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Current().DialogTitleStyle.Render("Change Request"))
	sb.WriteString("\n")

	// Scrollable content (File, Line, Status, Code context, Description).
	end := d.contentOffset + d.contentHeight
	if end > len(d.contentLines) {
		end = len(d.contentLines)
	}
	for i := d.contentOffset; i < end; i++ {
		sb.WriteString(d.contentLines[i])
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// OK button (always focused since it's the only button).
	sb.WriteString(theme.Current().ButtonFocusedStyle.Render("OK"))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}
