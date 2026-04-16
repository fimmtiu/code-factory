package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/git"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── Messages ────────────────────────────────────────────────────────────────

// openChangeRequestDialogMsg asks the root model to open the CR dialog.
type openChangeRequestDialogMsg struct {
	identifier   string
	fileName     string
	lineNum      int
	context      string
	worktreePath string
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

// ── ChangeRequestDialog ─────────────────────────────────────────────────────

// ChangeRequestDialog is the modal shown when the user presses R on a selected
// diff line. It collects a description and creates a change request via the database.
type ChangeRequestDialog struct {
	database     *db.DB
	identifier   string
	fileName     string
	lineNum      int
	context      string
	worktreePath string

	textArea TextArea
	focused  crDialogFocus
	errMsg   string // shown when the user submits an empty description

	width int
}

// NewChangeRequestDialog creates a ChangeRequestDialog for the given file location.
func NewChangeRequestDialog(database *db.DB, identifier, fileName string, lineNum int, context, worktreePath string, width int) ChangeRequestDialog {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	inputPad := theme.Current().QuickResponseInputStyle.GetHorizontalFrameSize()
	textWidth := width - dialogPad - inputPad
	if textWidth < 20 {
		textWidth = 20
	}
	return ChangeRequestDialog{
		database:     database,
		identifier:   identifier,
		fileName:     fileName,
		lineNum:      lineNum,
		context:      context,
		worktreePath: worktreePath,
		textArea:     NewTextArea(textWidth, 5),
		focused:      crFocusTextArea,
		width:        width,
	}
}

func (d ChangeRequestDialog) Init() tea.Cmd { return nil }

func (d ChangeRequestDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (d ChangeRequestDialog) submit() (tea.Model, tea.Cmd) {
	description := strings.TrimSpace(d.textArea.Value())
	if description == "" {
		d.errMsg = "Description cannot be empty"
		return d, nil
	}

	codeLocation := fmt.Sprintf("%s:%d", d.fileName, d.lineNum)
	database := d.database
	identifier := d.identifier
	worktreePath := d.worktreePath
	return d, tea.Batch(
		dismissDialogCmd(),
		createCRCmd(database, identifier, codeLocation, description, worktreePath),
	)
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

func (d ChangeRequestDialog) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Current().DialogTitleStyle.Render("New Change Request"))
	sb.WriteString("\n")

	// File and line info.
	sb.WriteString(theme.Current().DetailLabelStyle.Render(fmt.Sprintf("%s:%d", d.fileName, d.lineNum)))
	sb.WriteString("\n\n")

	// Code context.
	taWidth := d.textAreaWidth()
	for _, line := range strings.Split(d.context, "\n") {
		sb.WriteString(theme.Current().QuickResponseOutputStyle.Render(truncateLine(line, taWidth)))
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
func (d ChangeRequestDialog) textAreaWidth() int {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	inputPad := theme.Current().QuickResponseInputStyle.GetHorizontalFrameSize()
	w := d.width - dialogPad - inputPad
	if w < 20 {
		w = 20
	}
	return w
}
