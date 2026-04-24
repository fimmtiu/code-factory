package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// openNewTicketDialogMsg asks the root model to open the new-ticket dialog
// rooted under the given parent project.
type openNewTicketDialogMsg struct {
	parent *models.WorkUnit   // the selected project
	units  []*models.WorkUnit // current work units (for dependency picklist)
}

// ticketCreatedMsg is emitted once a new ticket has been written to the DB.
type ticketCreatedMsg struct {
	identifier string
	errMsg     string
}

// ── Focus ────────────────────────────────────────────────────────────────────

type ntFocus int

const (
	ntFocusSlug ntFocus = iota
	ntFocusDesc
	ntFocusDeps
	ntFocusCancel
	ntFocusCreate
	ntFocusCount
)

// ── NewTicketDialog ──────────────────────────────────────────────────────────

// AddTicketDialog is the modal shown when the user presses N on a project row
// in the Projects view. It collects an identifier slug, description, and
// optional dependencies, then calls db.CreateTicket.
type AddTicketDialog struct {
	database *db.DB
	parent   *models.WorkUnit // the selected project

	slug       []rune
	slugCursor int
	desc       TextArea
	deps       Picklist

	focused ntFocus
	errMsg  string

	width int
}

// NewAddTicketDialog constructs the dialog. units is the list of existing
// work units (projects + tickets) used to populate the dependency picklist.
func NewAddTicketDialog(database *db.DB, parent *models.WorkUnit, units []*models.WorkUnit, width int) *AddTicketDialog {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	inputPad := theme.Current().QuickResponseInputStyle.GetHorizontalFrameSize()
	inner := width - dialogPad
	if inner < 40 {
		inner = 40
	}
	taWidth := inner - inputPad
	if taWidth < 20 {
		taWidth = 20
	}

	// Build dependency items from everything except the ticket's eventual
	// parent (can't depend on your own project) and the self identifier
	// (unknown at construction — validated on submit).
	items := make([]PicklistItem, 0, len(units))
	for _, wu := range units {
		if wu.Identifier == parent.Identifier {
			continue
		}
		items = append(items, PicklistItem{ID: wu.Identifier})
	}

	d := &AddTicketDialog{
		database: database,
		parent:   parent,
		desc:     NewTextArea(taWidth, 5),
		deps:     NewPicklist(items, inner),
		focused:  ntFocusSlug,
		width:    width,
	}
	d.applyFocusToChildren()
	return d
}

// applyFocusToChildren syncs the current focus state onto the text area and
// picklist so their cursor / highlight visuals only appear when they have
// focus.
func (d *AddTicketDialog) applyFocusToChildren() {
	d.desc.SetFocused(d.focused == ntFocusDesc)
	d.deps.SetFocused(d.focused == ntFocusDeps)
}

func (d *AddTicketDialog) Init() tea.Cmd { return nil }

func (d *AddTicketDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	kmsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}

	if kmsg.String() == "esc" {
		return d, dismissDialogCmd()
	}

	// Tab / Shift+Tab cycle focus regardless of the active field so users can
	// always advance past the picklist's own keyboard handling.
	if kmsg.String() == "tab" {
		d.setFocus((d.focused + 1) % ntFocusCount)
		return d, nil
	}
	if kmsg.String() == "shift+tab" {
		d.setFocus((d.focused + ntFocusCount - 1) % ntFocusCount)
		return d, nil
	}

	switch d.focused {
	case ntFocusSlug:
		d.updateSlug(kmsg)
	case ntFocusDesc:
		d.desc.Update(kmsg)
	case ntFocusDeps:
		d.deps.Update(kmsg)
	case ntFocusCancel:
		if kmsg.String() == "enter" {
			return d, dismissDialogCmd()
		}
	case ntFocusCreate:
		if kmsg.String() == "enter" {
			return d.submit()
		}
	}
	return d, nil
}

func (d *AddTicketDialog) setFocus(f ntFocus) {
	d.focused = f
	d.errMsg = ""
	d.applyFocusToChildren()
}

func (d *AddTicketDialog) updateSlug(msg tea.KeyMsg) {
	switch msg.String() {
	case "backspace":
		if d.slugCursor > 0 {
			d.slug = append(d.slug[:d.slugCursor-1], d.slug[d.slugCursor:]...)
			d.slugCursor--
		}
	case "delete":
		if d.slugCursor < len(d.slug) {
			d.slug = append(d.slug[:d.slugCursor], d.slug[d.slugCursor+1:]...)
		}
	case "left":
		if d.slugCursor > 0 {
			d.slugCursor--
		}
	case "right":
		if d.slugCursor < len(d.slug) {
			d.slugCursor++
		}
	case "home", "ctrl+a":
		d.slugCursor = 0
	case "end", "ctrl+e":
		d.slugCursor = len(d.slug)
	case "enter":
		d.setFocus(ntFocusDesc)
	default:
		if len(msg.Runes) > 0 {
			d.slug = append(d.slug[:d.slugCursor], append(append([]rune{}, msg.Runes...), d.slug[d.slugCursor:]...)...)
			d.slugCursor += len(msg.Runes)
		}
	}
}

func (d *AddTicketDialog) submit() (tea.Model, tea.Cmd) {
	slug := strings.TrimSpace(string(d.slug))
	description := strings.TrimSpace(d.desc.Value())

	if slug == "" {
		d.setFocus(ntFocusSlug)
		d.errMsg = "Name cannot be empty"
		return d, nil
	}
	if description == "" {
		d.setFocus(ntFocusDesc)
		d.errMsg = "Description cannot be empty"
		return d, nil
	}

	identifier := slug
	if d.parent != nil && d.parent.Identifier != "" {
		identifier = d.parent.Identifier + "/" + slug
	}
	if err := models.ValidateIdentifier(identifier); err != nil {
		d.setFocus(ntFocusSlug)
		d.errMsg = err.Error()
		return d, nil
	}

	deps := d.deps.PickedIDs()
	database := d.database
	return d, tea.Batch(
		dismissDialogCmd(),
		func() tea.Msg {
			if err := database.CreateTicket(identifier, description, deps, ""); err != nil {
				return ticketCreatedMsg{identifier: identifier, errMsg: err.Error()}
			}
			return ticketCreatedMsg{identifier: identifier}
		},
	)
}

func (d *AddTicketDialog) View() string {
	var sb strings.Builder

	sb.WriteString(theme.Current().DialogTitleStyle.Render("New Ticket"))
	sb.WriteString("\n")
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Parent: "))
	sb.WriteString(d.parent.Identifier)
	sb.WriteString("\n\n")

	// Slug input.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Name"))
	sb.WriteString("\n")
	sb.WriteString(d.renderSlugInput())
	sb.WriteString("\n\n")

	// Description text area.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Description"))
	sb.WriteString("\n")
	sb.WriteString(d.renderDescBox())
	sb.WriteString("\n\n")

	// Dependency picklist.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Dependencies"))
	sb.WriteString("\n")
	sb.WriteString(d.deps.View())
	sb.WriteString("\n")

	if d.errMsg != "" {
		sb.WriteString("\n")
		sb.WriteString(theme.Current().DiffErrorStyle.Render(d.errMsg))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	cancelBtn := theme.Current().ButtonNormalStyle.Render("Cancel")
	createBtn := theme.Current().ButtonNormalStyle.Render("Create")
	switch d.focused {
	case ntFocusCancel:
		cancelBtn = theme.Current().ButtonFocusedStyle.Render("Cancel")
	case ntFocusCreate:
		createBtn = theme.Current().ButtonFocusedStyle.Render("Create")
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", createBtn))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}

func (d *AddTicketDialog) renderSlugInput() string {
	style := theme.Current().QuickResponseInputStyle
	if d.focused == ntFocusSlug {
		style = style.BorderForeground(lipgloss.Color("63"))
	}
	inner := d.width - theme.Current().DialogBoxStyle.GetHorizontalFrameSize() - style.GetHorizontalFrameSize()
	if inner < 10 {
		inner = 10
	}
	text := string(d.slug)
	if d.focused == ntFocusSlug {
		text = withPicklistCursor(text, d.slugCursor)
	}
	if lipgloss.Width(text) < inner {
		text += strings.Repeat(" ", inner-lipgloss.Width(text))
	}
	return style.Width(inner).Render(text)
}

func (d *AddTicketDialog) renderDescBox() string {
	style := theme.Current().QuickResponseInputStyle
	if d.focused == ntFocusDesc {
		style = style.BorderForeground(lipgloss.Color("63"))
	}
	return style.Render(d.desc.View())
}

// ── Helpers ──────────────────────────────────────────────────────────────────

// ticketCreatedNotificationCmd returns a notification command summarising the
// outcome of a CreateTicket call.
func ticketCreatedNotificationCmd(msg ticketCreatedMsg) tea.Cmd {
	if msg.errMsg != "" {
		return ShowNotification(fmt.Sprintf("Create ticket failed: %s", msg.errMsg))
	}
	return ShowNotification("Created " + msg.identifier)
}
