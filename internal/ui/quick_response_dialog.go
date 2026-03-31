package ui

import (
	"fmt"
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

// openQuickResponseMsg requests the root model to open the quick-response dialog.
type openQuickResponseMsg struct {
	wu *models.WorkUnit
}

// ── Styles ────────────────────────────────────────────────────────────────────

var quickResponseOutputStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("240"))

var quickResponseInputStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colourPrimary).
	Padding(0, 1)

// ── QuickResponseDialog ───────────────────────────────────────────────────────

// QuickResponseDialog is the modal shown when the user presses Enter on a
// needs-attention ticket. It shows the last 10 output lines of the ticket's
// logfile and provides a one-line input for a quick reply. Ctrl+E falls back
// to the full blocking editor.
type QuickResponseDialog struct {
	database *db.DB
	pool     *worker.Pool
	wu       *models.WorkUnit
	width    int // terminal width, used to size the input bar

	lines  []string // last 10 output lines from the logfile
	input  string   // current text in the input box
	cursor int      // cursor position within input (rune index)
}

// NewQuickResponseDialog creates a QuickResponseDialog for the given ticket.
func NewQuickResponseDialog(database *db.DB, pool *worker.Pool, wu *models.WorkUnit, width int) QuickResponseDialog {
	return QuickResponseDialog{
		database: database,
		pool:     pool,
		wu:       wu,
		width:    width,
		lines:    quickResponseLines(wu),
	}
}

// quickResponseLines returns the last 10 lines from the OUTPUT section of the
// latest logfile for wu's current phase.
func quickResponseLines(wu *models.WorkUnit) []string {
	repoRoot, err := storage.FindRepoRoot(".")
	if err != nil {
		return []string{"(could not find repo root)"}
	}
	ticketsDir := storage.TicketsDirPath(repoRoot)
	logPath := worker.LatestLogfilePath(ticketsDir, wu.Identifier, wu.Phase)
	if logPath == "" {
		return []string{"(no logfile found)"}
	}
	content := lastNLines(logPath, 10)
	if content == "" {
		return []string{"(no output)"}
	}
	return strings.Split(content, "\n")
}

func (d QuickResponseDialog) Init() tea.Cmd { return nil }

func (d QuickResponseDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "enter":
			text := strings.TrimSpace(d.input)
			if text == "" {
				return d, dismissDialogCmd()
			}
			return d, tea.Batch(dismissDialogCmd(), d.sendResponseCmd(text))

		case "ctrl+r":
			wu := d.wu
			database := d.database
			pool := d.pool
			return d, tea.Batch(
				dismissDialogCmd(),
				wrapEditorCmd(func() tea.Msg {
					return respondToAgentViaEditor(wu, database, pool)
				}),
			)

		case "backspace":
			runes := []rune(d.input)
			if d.cursor > 0 {
				runes = append(runes[:d.cursor-1], runes[d.cursor:]...)
				d.input = string(runes)
				d.cursor--
			}

		case "delete":
			runes := []rune(d.input)
			if d.cursor < len(runes) {
				runes = append(runes[:d.cursor], runes[d.cursor+1:]...)
				d.input = string(runes)
			}

		case "left":
			if d.cursor > 0 {
				d.cursor--
			}

		case "right":
			if d.cursor < len([]rune(d.input)) {
				d.cursor++
			}

		case "home", "ctrl+a":
			d.cursor = 0

		case "end", "ctrl+e":
			d.cursor = len([]rune(d.input))

		default:
			if len(msg.Runes) > 0 {
				runes := []rune(d.input)
				runes = append(runes[:d.cursor], append(msg.Runes, runes[d.cursor:]...)...)
				d.input = string(runes)
				d.cursor += len(msg.Runes)
			}
		}
	}
	return d, nil
}

// sendResponseCmd returns a Cmd that sends text to the worker and advances the
// ticket back to in-progress.
func (d QuickResponseDialog) sendResponseCmd(text string) tea.Cmd {
	wu := d.wu
	database := d.database
	pool := d.pool
	return func() tea.Msg {
		return sendResponseToWorker(wu, database, pool, text)
	}
}

// sendResponseToWorker sends text to the worker that owns wu and marks the
// ticket in-progress. It is the shared implementation used by both
// QuickResponseDialog and respondToAgentViaEditor.
func sendResponseToWorker(wu *models.WorkUnit, database *db.DB, pool *worker.Pool, text string) tea.Msg {
	workerNum, err := strconv.Atoi(wu.ClaimedBy)
	if err != nil {
		return respondToAgentDoneMsg{errMsg: fmt.Sprintf("response error: invalid worker number %q", wu.ClaimedBy)}
	}
	w := pool.GetWorker(workerNum)
	if w == nil {
		return respondToAgentDoneMsg{errMsg: fmt.Sprintf("response error: worker %d not found", workerNum)}
	}
	w.SendResponse(text)
	_ = database.SetStatus(wu.Identifier, wu.Phase, models.StatusInProgress)
	return respondToAgentDoneMsg{identifier: wu.Identifier}
}

// respondToAgentViaEditor opens the full blocking editor pre-filled with the
// agent output and returns a respondToAgentDoneMsg. Intended to be called
// inside a wrapEditorCmd goroutine.
func respondToAgentViaEditor(wu *models.WorkUnit, database *db.DB, pool *worker.Pool) tea.Msg {
	template := buildAgentResponseTemplate(wu)
	raw, err := util.EditText(template)
	if err != nil {
		return respondToAgentDoneMsg{errMsg: fmt.Sprintf("editor error: %s", err)}
	}
	response := strings.TrimSpace(extractResponseText(raw))
	if response == "" {
		return respondToAgentDoneMsg{}
	}
	return sendResponseToWorker(wu, database, pool, response)
}

func (d QuickResponseDialog) View() string {
	var sb strings.Builder

	sb.WriteString(dialogTitleStyle.Render("Agent Output — " + d.wu.Identifier))
	sb.WriteString("\n")

	for _, line := range d.lines {
		sb.WriteString(quickResponseOutputStyle.Render(line))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")

	// Build input with block cursor.
	runes := []rune(d.input)
	var inputContent string
	if d.cursor < len(runes) {
		inputContent = string(runes[:d.cursor]) + "█" + string(runes[d.cursor:])
	} else {
		inputContent = d.input + "█"
	}
	inputWidth := d.width - dialogBoxStyle.GetHorizontalFrameSize() - quickResponseInputStyle.GetHorizontalFrameSize()
	if inputWidth < 1 {
		inputWidth = 1
	}
	sb.WriteString(quickResponseInputStyle.Width(inputWidth).Render(inputContent))
	sb.WriteString("\n\n")

	sb.WriteString(helpHintStyle.Render(buildHint("Enter", "send", "Ctrl+R", "edit in editor", "Esc", "cancel")))

	return dialogBoxStyle.Render(sb.String())
}
