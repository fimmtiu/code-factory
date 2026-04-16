package ui

import (
	"fmt"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── Messages ────────────────────────────────────────────────────────────────

// crLocation groups the code-location fields needed to create or edit a change request.
type crLocation struct {
	identifier   string
	fileName     string
	lineNum      int
	context      string
	worktreePath string
}

// openEditChangeRequestDialogMsg asks the root model to open the CR dialog.
type openEditChangeRequestDialogMsg struct {
	location   crLocation
	existingCR *models.ChangeRequest
}

// crCreatedMsg is the result of running cf-tickets create-cr.
type crCreatedMsg struct {
	errMsg string
}

// ── Focus ───────────────────────────────────────────────────────────────────

type crDialogFocus int

const (
	crFocusTextArea crDialogFocus = iota
	crFocusCancel
	crFocusOK
	crFocusCount // sentinel for modular arithmetic
)

// ── EditChangeRequestDialog ─────────────────────────────────────────────────

// EditChangeRequestDialog is the modal shown when the user presses R on a selected
// diff line. It collects a description and creates a change request via the database.
type EditChangeRequestDialog struct {
	database   *db.DB
	location   crLocation
	existingCR *models.ChangeRequest

	textArea TextArea
	focused  crDialogFocus
	errMsg   string // shown when the user submits an empty description

	width int
}

// NewEditChangeRequestDialog creates an EditChangeRequestDialog for the given file location.
// If existingCR is non-nil, the dialog operates in edit mode with the description pre-populated.
func NewEditChangeRequestDialog(database *db.DB, loc crLocation, existingCR *models.ChangeRequest, width int) EditChangeRequestDialog {
	d := EditChangeRequestDialog{
		database:   database,
		location:   loc,
		existingCR: existingCR,
		focused:    crFocusTextArea,
		width:      width,
	}
	d.textArea = NewTextArea(d.textAreaWidth(), 5)
	if existingCR != nil {
		d.textArea.SetValue(existingCR.Description)
	}
	return d
}

func (d EditChangeRequestDialog) Init() tea.Cmd { return nil }

func (d EditChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Escape always dismisses.
		if msg.String() == "esc" {
			return d, dismissDialogCmd()
		}

		// Tab cycles focus forward; Shift+Tab cycles backward.
		if msg.String() == "tab" {
			d.errMsg = ""
			d.focused = (d.focused + 1) % crFocusCount
			return d, nil
		}
		if msg.String() == "shift+tab" {
			d.errMsg = ""
			d.focused = (d.focused + crFocusCount - 1) % crFocusCount
			return d, nil
		}

		switch d.focused {
		case crFocusTextArea:
			d.textArea.Update(msg)
		case crFocusCancel:
			if msg.String() == "enter" {
				return d, dismissDialogCmd()
			}
		case crFocusOK:
			if msg.String() == "enter" {
				return d.submit()
			}
		}
	}
	return d, nil
}

func (d EditChangeRequestDialog) submit() (tea.Model, tea.Cmd) {
	description := strings.TrimSpace(d.textArea.Value())
	if description == "" {
		d.errMsg = "Description cannot be empty"
		return d, nil
	}

	if d.existingCR != nil {
		id, err := strconv.ParseInt(d.existingCR.ID, 10, 64)
		if err != nil {
			return d, nil
		}
		database := d.database
		return d, tea.Batch(
			dismissDialogCmd(),
			updateCRDescriptionCmd(database, id, description),
		)
	}

	codeLocation := fmt.Sprintf("%s:%d", d.location.fileName, d.location.lineNum)
	database := d.database
	identifier := d.location.identifier
	worktreePath := d.location.worktreePath
	return d, tea.Batch(
		dismissDialogCmd(),
		createCRCmd(database, identifier, codeLocation, description, worktreePath),
	)
}

// updateCRDescriptionCmd returns a command that updates an existing CR's description.
func updateCRDescriptionCmd(database *db.DB, id int64, description string) tea.Cmd {
	return func() tea.Msg {
		if err := database.UpdateChangeRequestDescription(id, description); err != nil {
			return crCreatedMsg{errMsg: fmt.Sprintf("update-cr: %s", err)}
		}
		return crCreatedMsg{}
	}
}

// createCRCmd returns a command that creates a change request via the database.
func createCRCmd(database *db.DB, identifier, codeLocation, description, worktreePath string) tea.Cmd {
	return func() tea.Msg {
		author := "user"
		if name, err := git.Output(worktreePath, "config", "user.name"); err == nil && name != "" {
			author = name
		}

		if err := database.AddChangeRequest(identifier, codeLocation, author, description); err != nil {
			return crCreatedMsg{errMsg: fmt.Sprintf("create-cr: %s", err)}
		}
		return crCreatedMsg{}
	}
}

func (d EditChangeRequestDialog) View() string {
	var sb strings.Builder

	title := "New Change Request"
	if d.existingCR != nil {
		title = "Edit Change Request"
	}
	sb.WriteString(theme.Current().DialogTitleStyle.Render(title))
	sb.WriteString("\n")

	// File and line info.
	sb.WriteString(theme.Current().DetailLabelStyle.Render(fmt.Sprintf("%s:%d", d.location.fileName, d.location.lineNum)))
	sb.WriteString("\n")

	// Status line (edit mode only).
	if d.existingCR != nil {
		sb.WriteString(theme.Current().DetailLabelStyle.Render("Status:") + " " + d.existingCR.Status)
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Code context.
	taWidth := d.textAreaWidth()
	for _, line := range strings.Split(d.location.context, "\n") {
		sb.WriteString(theme.Current().QuickResponseOutputStyle.Render(truncateLine(strings.TrimRight(line, " \t"), taWidth)))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	// Text area with border. Highlight border when focused.
	taContent := d.textArea.View()
	taStyle := theme.Current().QuickResponseInputStyle
	if d.focused == crFocusTextArea {
		taStyle = taStyle.BorderForeground(lipgloss.Color("63"))
	}
	sb.WriteString(taStyle.Render(taContent))
	sb.WriteString("\n")

	// Error message.
	if d.errMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(theme.Current().DiffErrorStyle.Render(d.errMsg))
	}
	sb.WriteString("\n")

	// Buttons.
	cancelBtn := theme.Current().ButtonNormalStyle.Render("Cancel")
	okBtn := theme.Current().ButtonNormalStyle.Render("OK")
	if d.focused == crFocusCancel {
		cancelBtn = theme.Current().ButtonFocusedStyle.Render("Cancel")
	} else if d.focused == crFocusOK {
		okBtn = theme.Current().ButtonFocusedStyle.Render("OK")
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", okBtn))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}

// textAreaWidth returns the inner width available for the text area content.
func (d EditChangeRequestDialog) textAreaWidth() int {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	inputPad := theme.Current().QuickResponseInputStyle.GetHorizontalFrameSize()
	w := d.width - dialogPad - inputPad
	if w < 20 {
		w = 20
	}
	return w
}
