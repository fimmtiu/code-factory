package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/db"
	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// ── PermissionDialog ──────────────────────────────────────────────────────────

// PermissionDialog is shown when the agent issues a permission request (or any
// structured options request). It displays the options as a numbered list so
// the user can respond with a single keystroke.
type PermissionDialog struct {
	database *db.DB
	pool     *worker.Pool
	wu       *models.WorkUnit

	title    string
	options  []worker.PermissionOption // sorted for display
	selected int                       // currently highlighted option (0-based)
	width    int                       // terminal width for constraining dialog size
}

// permOptionOrder defines the canonical sort position for known option kinds.
// Unknown kinds retain their original relative order (stable sort).
var permOptionOrder = map[string]int{
	"allow_once":    1,
	"allow_always":  2,
	"reject_once":   3,
	"reject_always": 4,
}

// NewPermissionDialog creates a PermissionDialog for the given ticket and
// pending permission request. Options are sorted into the canonical order
// (allow once → always allow → reject), with unknown kinds last in original order.
func NewPermissionDialog(database *db.DB, pool *worker.Pool, wu *models.WorkUnit, perm *worker.PendingPermissionRequest, width int) PermissionDialog {
	opts := make([]worker.PermissionOption, len(perm.Options))
	copy(opts, perm.Options)
	sort.SliceStable(opts, func(i, j int) bool {
		pi, oki := permOptionOrder[opts[i].Kind]
		pj, okj := permOptionOrder[opts[j].Kind]
		if !oki {
			pi = 99
		}
		if !okj {
			pj = 99
		}
		return pi < pj
	})
	return PermissionDialog{
		database: database,
		pool:     pool,
		wu:       wu,
		title:    perm.Title,
		options:  opts,
		width:    width,
	}
}

func (d PermissionDialog) Init() tea.Cmd { return nil }

func (d PermissionDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "up", "k":
			if d.selected > 0 {
				d.selected--
			}

		case "down", "j":
			if d.selected < len(d.options)-1 {
				d.selected++
			}

		case "enter":
			return d, tea.Batch(dismissDialogCmd(), d.sendResponseCmd(d.options[d.selected].Kind))

		default:
			// Number keys 1–9: pick option directly.
			if len(msg.Runes) == 1 {
				n := int(msg.Runes[0] - '1') // 0-based index
				if n >= 0 && n < len(d.options) {
					return d, tea.Batch(dismissDialogCmd(), d.sendResponseCmd(d.options[n].Kind))
				}
			}
		}
	}
	return d, nil
}

func (d PermissionDialog) sendResponseCmd(kind string) tea.Cmd {
	wu := d.wu
	database := d.database
	pool := d.pool
	return func() tea.Msg {
		return sendResponseToWorker(wu, database, pool, kind)
	}
}

func (d PermissionDialog) View() string {
	// dialogBoxStyle has Border (2) + Padding (4) = 6 horizontal frame chars.
	// Leave 4 chars of margin (2 per side) between dialog and screen edge.
	contentWidth := d.width - 10
	if contentWidth < 20 {
		contentWidth = 20
	}

	var sb strings.Builder

	sb.WriteString(dialogTitleStyle.Render("Permission Request"))
	sb.WriteString("\n")

	if d.title != "" {
		sb.WriteString(lipgloss.NewStyle().Width(contentWidth).Render(d.title))
		sb.WriteString("\n")
	}
	sb.WriteString("\n")

	for i, opt := range d.options {
		number := strconv.Itoa(i + 1)
		label := fmt.Sprintf("%s  %s", number, opt.Name)
		if i == d.selected {
			sb.WriteString(permOptionSelectedStyle.Render(label))
		} else {
			sb.WriteString(permOptionNormalStyle.Render(label))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(helpHintStyle.Render(buildHint("1-9", "pick", "↑↓", "navigate", "Enter", "confirm", "Esc", "cancel")))

	return dialogBoxStyle.Width(contentWidth).Render(sb.String())
}
