package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/fimmtiu/code-factory/internal/models"
	"github.com/fimmtiu/code-factory/internal/storage"
	"github.com/fimmtiu/code-factory/internal/worker"
)

// openLogfileDialogMsg requests the root model to open the logfile dialog for
// the given ticket work unit.
type openLogfileDialogMsg struct {
	wu *models.WorkUnit
}

// logfileEntry holds display metadata and content for one phase's logfile.
type logfileEntry struct {
	label string             // capitalised phase name, e.g. "Implement"
	phase models.TicketPhase // raw phase value, e.g. "implement"
	path  string             // absolute path to the logfile
	lines []string           // file content split by newline
}

// phaseOrder defines the display order and labels for ticket phases.
var phaseOrder = []struct {
	phase models.TicketPhase
	label string
}{
	{models.PhaseImplement, "Implement"},
	{models.PhaseRefactor, "Refactor"},
	{models.PhaseReview, "Review"},
	{models.PhaseRespond, "Respond"},
}

type logfilePaneFocus int

const (
	logfileListFocus    logfilePaneFocus = iota
	logfileContentFocus logfilePaneFocus = iota
)

// LogfileDialog is a two-pane modal showing logfiles for a ticket.
type LogfileDialog struct {
	wu            *models.WorkUnit
	entries       []logfileEntry
	selected      int
	listOffset    int
	contentOffset int
	focus         logfilePaneFocus

	// Outer dialog dimensions (after margin is applied in computeDimensions).
	width  int
	height int

	// Inner pane content dimensions (excluding pane borders).
	listW, listH       int
	contentW, contentH int
}

// NewLogfileDialog builds a LogfileDialog for the given ticket work unit,
// discovering logfiles from the repo root.
func NewLogfileDialog(wu *models.WorkUnit, width, height int) *LogfileDialog {
	var entries []logfileEntry
	if repoRoot, err := storage.FindRepoRoot("."); err == nil {
		ticketsDir := storage.TicketsDirPath(repoRoot)
		for _, p := range phaseOrder {
			path := worker.LatestLogfilePath(ticketsDir, wu.Identifier, p.phase)
			if path == "" {
				continue
			}
			var lines []string
			if data, err := os.ReadFile(path); err == nil {
				lines = strings.Split(string(data), "\n")
			}
			entries = append(entries, logfileEntry{
				label: p.label,
				phase: p.phase,
				path:  path,
				lines: lines,
			})
		}
	}

	d := &LogfileDialog{
		wu:      wu,
		entries: entries,
		width:   width,
		height:  height,
	}
	d.computeDimensions()
	return d
}

// computeDimensions calculates pane sizes from the terminal dimensions.
// Overhead breakdown matches ChangeRequestDialog — see that type for details.
func (d *LogfileDialog) computeDimensions() {
	const (
		marginH      = 8
		marginV      = 4
		dlgOverheadH = 10
		dlgOverheadV = 9
	)

	d.width = d.width - marginH
	d.height = d.height - marginV
	if d.width < 40 {
		d.width = 40
	}
	if d.height < 15 {
		d.height = 15
	}

	availW := d.width - dlgOverheadH
	availH := d.height - dlgOverheadV

	if availW < 4 {
		availW = 4
	}
	if availH < 2 {
		availH = 2
	}

	d.listW = availW * 25 / 100
	if d.listW < 12 {
		d.listW = 12
	}
	d.contentW = availW - d.listW
	if d.contentW < 10 {
		d.contentW = 10
	}

	d.listH = availH
	d.contentH = availH
}

func (d *LogfileDialog) Init() tea.Cmd { return nil }

func (d *LogfileDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc":
			return d, dismissDialogCmd()

		case "d", "D":
			if len(d.entries) > 0 {
				e := d.entries[d.selected]
				return d, debugPromptCmd(d.wu, e.phase, e.path)
			}

		case "tab":
			if d.focus == logfileListFocus {
				d.focus = logfileContentFocus
			} else {
				d.focus = logfileListFocus
			}

		case "up":
			if d.focus == logfileListFocus {
				if d.selected > 0 {
					d.selected--
					d.contentOffset = 0
					d.clampListScroll()
				}
			} else {
				if d.contentOffset > 0 {
					d.contentOffset--
				}
			}

		case "down":
			if d.focus == logfileListFocus {
				if d.selected < len(d.entries)-1 {
					d.selected++
					d.contentOffset = 0
					d.clampListScroll()
				}
			} else {
				d.contentOffset++
				d.clampContentScroll()
			}

		case "pgup":
			if d.focus == logfileListFocus {
				d.selected -= d.listH
				if d.selected < 0 {
					d.selected = 0
				}
				d.contentOffset = 0
				d.clampListScroll()
			} else {
				d.contentOffset -= d.contentH
				if d.contentOffset < 0 {
					d.contentOffset = 0
				}
			}

		case "pgdown":
			if d.focus == logfileListFocus {
				d.selected += d.listH
				if d.selected >= len(d.entries) {
					d.selected = max(0, len(d.entries)-1)
				}
				d.contentOffset = 0
				d.clampListScroll()
			} else {
				d.contentOffset += d.contentH
				d.clampContentScroll()
			}
		}
	}
	return d, nil
}

func (d *LogfileDialog) clampListScroll() {
	if d.selected < d.listOffset {
		d.listOffset = d.selected
	}
	if d.selected >= d.listOffset+d.listH {
		d.listOffset = d.selected - d.listH + 1
	}
	if d.listOffset < 0 {
		d.listOffset = 0
	}
}

// wrappedContentLines word-wraps the selected entry's file lines to fit the
// content pane width, breaking at spaces, tabs, or dashes where possible and
// falling back to a hard break only when no such boundary exists.
func (d *LogfileDialog) wrappedContentLines() []string {
	if len(d.entries) == 0 || d.contentW <= 0 {
		return nil
	}
	var result []string
	for _, line := range d.entries[d.selected].lines {
		result = append(result, wrapLine(line, d.contentW)...)
	}
	return result
}

// wrapLine splits a single line into display lines of at most width runes,
// preferring to break at spaces, tabs, or after dashes. Falls back to a hard
// break only when no such boundary exists in the current segment.
func wrapLine(line string, width int) []string {
	runes := []rune(line)
	if len(runes) <= width {
		return []string{line}
	}
	var result []string
	for len(runes) > width {
		// Search backward from position width-1 for a natural break point.
		cut, skipOne := width, false // cut: where next segment starts; skipOne: consume break char
		for i := width - 1; i >= 0; i-- {
			if runes[i] == ' ' || runes[i] == '\t' {
				cut = i
				skipOne = true // consume the space
				break
			}
			if runes[i] == '-' {
				cut = i + 1
				break // keep the dash on the current line
			}
		}
		result = append(result, string(runes[:cut]))
		runes = runes[cut:]
		if skipOne && len(runes) > 0 {
			runes = runes[1:]
		}
	}
	return append(result, string(runes))
}

func (d *LogfileDialog) clampContentScroll() {
	if len(d.entries) == 0 {
		d.contentOffset = 0
		return
	}
	lines := d.wrappedContentLines()
	maxOffset := len(lines) - d.contentH
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

func (d *LogfileDialog) View() string {
	title := dialogTitleStyle.Render(fmt.Sprintf("Logfiles for `%s`", d.wu.Identifier))

	if len(d.entries) == 0 {
		body := lipgloss.JoinVertical(lipgloss.Left, title, "(no logfiles found)")
		return dialogBoxStyle.Width(d.width - 2).Render(body)
	}

	listBorderStyle := unfocusedBorderStyle
	contentBorderStyle := unfocusedBorderStyle
	if d.focus == logfileListFocus {
		listBorderStyle = focusedBorderStyle
	} else {
		contentBorderStyle = focusedBorderStyle
	}

	listPane := d.renderListPane(listBorderStyle)
	contentPane := d.renderContentPane(contentBorderStyle)

	hint := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑↓ scroll  Tab switch pane  D debug prompt  Esc close")

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		lipgloss.JoinHorizontal(lipgloss.Top, listPane, contentPane),
		hint,
	)
	return dialogBoxStyle.Width(d.width - 2).Render(body)
}

func (d *LogfileDialog) renderListPane(borderStyle lipgloss.Style) string {
	var sb strings.Builder
	end := d.listOffset + d.listH
	if end > len(d.entries) {
		end = len(d.entries)
	}
	for i := d.listOffset; i < end; i++ {
		label := d.entries[i].label
		if len([]rune(label)) > d.listW {
			label = string([]rune(label)[:d.listW])
		}
		if i == d.selected {
			sb.WriteString(crSelectedStyle.Width(d.listW).Render(label))
		} else {
			sb.WriteString(label)
		}
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return borderStyle.Width(d.listW).Height(d.listH).Render(sb.String())
}

func (d *LogfileDialog) renderContentPane(borderStyle lipgloss.Style) string {
	if len(d.entries) == 0 {
		return borderStyle.Width(d.contentW).Height(d.contentH).Render("")
	}

	lines := d.wrappedContentLines()
	end := d.contentOffset + d.contentH
	if end > len(lines) {
		end = len(lines)
	}

	var sb strings.Builder
	for i := d.contentOffset; i < end; i++ {
		sb.WriteString(lines[i])
		if i < end-1 {
			sb.WriteString("\n")
		}
	}
	return borderStyle.Width(d.contentW).Height(d.contentH).Render(sb.String())
}
