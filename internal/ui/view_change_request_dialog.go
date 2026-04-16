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

// ── ViewChangeRequestDialog ─────────────────────────────────────────────────

// ViewChangeRequestDialog is the modal shown when the user presses Enter on a
// diff line that has a change request. It displays the CR details read-only.
type ViewChangeRequestDialog struct {
	database     *db.DB
	cr           models.ChangeRequest
	identifier   string
	worktreePath string
	width        int
}

// NewViewChangeRequestDialog creates a ViewChangeRequestDialog for the given CR.
func NewViewChangeRequestDialog(database *db.DB, cr models.ChangeRequest, identifier, worktreePath string, width int) ViewChangeRequestDialog {
	return ViewChangeRequestDialog{
		database:     database,
		cr:           cr,
		identifier:   identifier,
		worktreePath: worktreePath,
		width:        width,
	}
}

func (d ViewChangeRequestDialog) Init() tea.Cmd { return nil }

func (d ViewChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "enter":
			return d, dismissDialogCmd()
		}
	}
	return d, nil
}

func (d ViewChangeRequestDialog) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Current().DialogTitleStyle.Render("Change Request"))
	sb.WriteString("\n\n")

	sb.WriteString(theme.Current().DetailLabelStyle.Render("Location: "))
	sb.WriteString(d.cr.CodeLocation)
	sb.WriteString("\n")

	sb.WriteString(theme.Current().DetailLabelStyle.Render("Author: "))
	sb.WriteString(d.cr.Author)
	sb.WriteString("\n")

	sb.WriteString(theme.Current().DetailLabelStyle.Render("Status: "))
	sb.WriteString(d.cr.Status)
	sb.WriteString("\n")

	if !d.cr.Date.IsZero() {
		sb.WriteString(theme.Current().DetailLabelStyle.Render("Date: "))
		sb.WriteString(d.cr.Date.Format("2006-01-02 15:04"))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Description, wrapped to dialog width.
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	descWidth := d.width - dialogPad
	if descWidth < 20 {
		descWidth = 20
	}
	for _, line := range strings.Split(d.cr.Description, "\n") {
		wrapped := wrapLine(line, descWidth)
		for _, w := range wrapped {
			sb.WriteString(w)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(theme.Current().ButtonFocusedStyle.Render("OK"))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}
