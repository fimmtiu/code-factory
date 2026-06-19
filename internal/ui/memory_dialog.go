package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/ui/theme"
)

// ── Messages ────────────────────────────────────────────────────────────────

// memoryKinds are the selectable memory kinds, matching the db kind vocabulary.
var memoryKinds = []string{"lesson", "pattern", "gotcha", "note"}

// openMemoryDialogMsg asks the root model to open the add/edit memory dialog.
// When existing is nil the dialog adds a new repository-wide memory; otherwise
// it edits the given memory.
type openMemoryDialogMsg struct {
	existing *db.Memory
}

// memorySavedMsg is sent after a memory is created or updated.
type memorySavedMsg struct {
	err    error
	edited bool // true when an existing memory was updated, false when created
}

// ── Focus ───────────────────────────────────────────────────────────────────

type memoryDialogFocus int

const (
	mdFocusKind memoryDialogFocus = iota
	mdFocusText
	mdFocusCancel
	mdFocusOK
	mdFocusCount // sentinel for modular arithmetic
)

// ── MemoryDialog ──────────────────────────────────────────────────────────────

// MemoryDialog adds a new memory or edits an existing one. New memories are
// repository-wide in scope; editing leaves the memory's scope unchanged.
type MemoryDialog struct {
	database *db.DB
	existing *db.Memory // nil = add mode

	kindIdx  int // index into memoryKinds
	textArea TextArea
	focused  memoryDialogFocus
	errMsg   string

	width int
}

// NewMemoryDialog creates the dialog. When existing is nil it adds a new
// repository-wide memory; otherwise it edits existing with its kind and text
// pre-populated.
func NewMemoryDialog(database *db.DB, existing *db.Memory, width int) MemoryDialog {
	d := MemoryDialog{
		database: database,
		existing: existing,
		focused:  mdFocusKind,
		width:    width,
	}
	if existing != nil {
		for i, k := range memoryKinds {
			if k == existing.Kind {
				d.kindIdx = i
				break
			}
		}
	}
	d.textArea = NewTextArea(d.textAreaWidth(), 6)
	if existing != nil {
		d.textArea.SetValue(existing.Text)
	}
	return d
}

func (d MemoryDialog) Init() tea.Cmd { return nil }

func (d MemoryDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Escape always dismisses.
		if msg.String() == "esc" {
			return d, dismissDialogCmd()
		}

		// Tab cycles focus forward; Shift+Tab cycles backward.
		if msg.String() == "tab" {
			d.errMsg = ""
			d.focused = (d.focused + 1) % mdFocusCount
			d.textArea.SetFocused(d.focused == mdFocusText)
			return d, nil
		}
		if msg.String() == "shift+tab" {
			d.errMsg = ""
			d.focused = (d.focused + mdFocusCount - 1) % mdFocusCount
			d.textArea.SetFocused(d.focused == mdFocusText)
			return d, nil
		}

		switch d.focused {
		case mdFocusKind:
			switch msg.String() {
			case "up":
				if d.kindIdx > 0 {
					d.kindIdx--
				}
			case "down":
				if d.kindIdx < len(memoryKinds)-1 {
					d.kindIdx++
				}
			}
		case mdFocusText:
			d.textArea.Update(msg)
		case mdFocusCancel:
			if msg.String() == "enter" {
				return d, dismissDialogCmd()
			}
		case mdFocusOK:
			if msg.String() == "enter" {
				return d.submit()
			}
		}
	}
	return d, nil
}

func (d MemoryDialog) submit() (tea.Model, tea.Cmd) {
	text := strings.TrimSpace(d.textArea.Value())
	if text == "" {
		d.errMsg = "Memory text cannot be empty"
		return d, nil
	}
	kind := memoryKinds[d.kindIdx]
	database := d.database

	if d.existing != nil {
		id := d.existing.ID
		return d, tea.Batch(
			dismissDialogCmd(),
			func() tea.Msg {
				return memorySavedMsg{err: database.UpdateMemory(id, kind, text), edited: true}
			},
		)
	}

	return d, tea.Batch(
		dismissDialogCmd(),
		func() tea.Msg {
			_, err := database.AddMemory("", kind, text, "")
			return memorySavedMsg{err: err}
		},
	)
}

func (d MemoryDialog) View() string {
	var sb strings.Builder

	title := "Add Memory"
	if d.existing != nil {
		title = "Edit Memory"
	}
	sb.WriteString(theme.Current().DialogTitleStyle.Render(title))
	sb.WriteString("\n")

	// Scope line (informational; not editable).
	scope := "(repository-wide)"
	if d.existing != nil {
		scope = memoryScopeDisplay(d.existing.Scope)
	}
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Scope:") + " " + scope)
	sb.WriteString("\n\n")

	// Kind picklist.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Kind:"))
	sb.WriteString("\n")
	sb.WriteString(d.renderKindList())
	sb.WriteString("\n\n")

	// Text area with border. Highlight border when focused.
	sb.WriteString(theme.Current().DetailLabelStyle.Render("Text:"))
	sb.WriteString("\n")
	taStyle := theme.Current().QuickResponseInputStyle
	if d.focused == mdFocusText {
		taStyle = taStyle.BorderForeground(lipgloss.Color("63"))
	}
	sb.WriteString(taStyle.Render(d.textArea.View()))
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
	if d.focused == mdFocusCancel {
		cancelBtn = theme.Current().ButtonFocusedStyle.Render("Cancel")
	} else if d.focused == mdFocusOK {
		okBtn = theme.Current().ButtonFocusedStyle.Render("OK")
	}
	sb.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "  ", okBtn))

	return theme.Current().DialogBoxStyle.Render(sb.String())
}

// renderKindList renders the single-select kind picker, highlighting the
// current choice (brightly when the list has focus, dimly otherwise).
func (d MemoryDialog) renderKindList() string {
	items := make([]string, 0, len(memoryKinds))
	for i, k := range memoryKinds {
		if i == d.kindIdx {
			var label string
			if d.focused == mdFocusKind {
				label = theme.Current().CmdSelectedStyle.Render(fmt.Sprintf(" %-10s", k))
			} else {
				label = theme.Current().CmdSelectedUnfocusedStyle.Render(fmt.Sprintf(" %-10s", k))
			}
			items = append(items, " > "+label)
		} else {
			items = append(items, fmt.Sprintf("   %-10s ", k))
		}
	}
	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// textAreaWidth returns the inner width available for the text area content.
func (d MemoryDialog) textAreaWidth() int {
	dialogPad := theme.Current().DialogBoxStyle.GetHorizontalFrameSize()
	inputPad := theme.Current().QuickResponseInputStyle.GetHorizontalFrameSize()
	w := d.width - dialogPad - inputPad
	if w < 20 {
		w = 20
	}
	return w
}
