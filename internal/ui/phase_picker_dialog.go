package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
)

// ── Messages ─────────────────────────────────────────────────────────────────

type openPhasePickerMsg struct {
	wu *models.WorkUnit
}

// phaseSetMsg is broadcast after the phase picker changes a ticket's phase,
// so views can refresh.
type phaseSetMsg struct{}

// ── PhasePickerDialog ────────────────────────────────────────────────────────

var pickerPhases = []models.TicketPhase{
	models.PhaseImplement,
	models.PhaseRefactor,
	models.PhaseReview,
	models.PhaseRespond,
}

type ppFocus int

const (
	ppFocusList ppFocus = iota
	ppFocusOK
	ppFocusCancel
)

type PhasePickerDialog struct {
	database   *db.DB
	identifier string
	selected   int // index into pickerPhases
	focus      ppFocus
}

func NewPhasePickerDialog(database *db.DB, wu *models.WorkUnit) PhasePickerDialog {
	// Pre-select the ticket's current phase if it's in the list.
	sel := 0
	for i, p := range pickerPhases {
		if p == wu.Phase {
			sel = i
			break
		}
	}
	return PhasePickerDialog{
		database:   database,
		identifier: wu.Identifier,
		selected:   sel,
		focus:      ppFocusList,
	}
}

func (d PhasePickerDialog) Init() tea.Cmd { return nil }

func (d PhasePickerDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "up":
			if d.focus == ppFocusList && d.selected > 0 {
				d.selected--
			}
		case "down":
			if d.focus == ppFocusList && d.selected < len(pickerPhases)-1 {
				d.selected++
			}

		case "tab":
			switch d.focus {
			case ppFocusList:
				d.focus = ppFocusOK
			case ppFocusOK:
				d.focus = ppFocusCancel
			case ppFocusCancel:
				d.focus = ppFocusList
			}
		case "shift+tab":
			switch d.focus {
			case ppFocusList:
				d.focus = ppFocusCancel
			case ppFocusOK:
				d.focus = ppFocusList
			case ppFocusCancel:
				d.focus = ppFocusOK
			}

		case "enter":
			switch d.focus {
			case ppFocusList, ppFocusOK:
				return d, d.setPhase()
			case ppFocusCancel:
				return d, dismissDialogCmd()
			}
		}
	}
	return d, nil
}

func (d PhasePickerDialog) setPhase() tea.Cmd {
	database := d.database
	identifier := d.identifier
	phase := pickerPhases[d.selected]
	return tea.Batch(
		func() tea.Msg {
			_ = database.SetStatus(identifier, phase, models.StatusIdle)
			return phaseSetMsg{}
		},
		dismissDialogCmd(),
	)
}

func (d PhasePickerDialog) View() string {
	title := dialogTitleStyle.Render(fmt.Sprintf("Set phase for %s", d.identifier))

	var listItems []string
	for i, p := range pickerPhases {
		label := string(p)
		if i == d.selected {
			cursor := " > "
			if d.focus == ppFocusList {
				cursor = " > "
				label = cmdSelectedStyle.Render(fmt.Sprintf(" %-12s", label))
			} else {
				label = fmt.Sprintf(" %-12s", label)
				label = lipgloss.NewStyle().Foreground(colourAccent).Render(label)
			}
			listItems = append(listItems, cursor+label)
		} else {
			listItems = append(listItems, fmt.Sprintf("   %-12s ", label))
		}
	}

	list := lipgloss.JoinVertical(lipgloss.Left, listItems...)

	okBtn := buttonNormalStyle.Render("OK")
	cancelBtn := buttonNormalStyle.Render("Cancel")
	if d.focus == ppFocusOK {
		okBtn = buttonFocusedStyle.Render("OK")
	} else if d.focus == ppFocusCancel {
		cancelBtn = buttonFocusedStyle.Render("Cancel")
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, okBtn, "  ", cancelBtn)

	body := lipgloss.JoinVertical(lipgloss.Left, title, list, "", buttons)
	return dialogBoxStyle.Render(body)
}
