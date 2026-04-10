package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// hintKeyStyle renders a keystroke label bold in the muted hint colour.
var hintKeyStyle = lipgloss.NewStyle().Foreground(colourMuted).Bold(true)

// hintDescStyle renders hint description text in the normal muted colour.
var hintDescStyle = lipgloss.NewStyle().Foreground(colourMuted)

// buildHint renders alternating key/description pairs with each key bolded.
// Example: buildHint("Q", "quit", "?", "help") → bold("Q")+" quit  "+bold("?")+" help"
func buildHint(pairs ...string) string {
	var sb strings.Builder
	for i := 0; i+1 < len(pairs); i += 2 {
		if i > 0 {
			sb.WriteString(hintDescStyle.Render("  "))
		}
		sb.WriteString(hintKeyStyle.Render(pairs[i]))
		sb.WriteString(hintDescStyle.Render(" " + pairs[i+1]))
	}
	return sb.String()
}

// viewPaneStyle is the blue single-line border applied to Command, Worker, and Log views.
var viewPaneStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(colourBorderBlue)

// emptyStateStyle is applied to placeholder messages such as "No actionable tickets"
// so they read as secondary/hint text rather than content.
var emptyStateStyle = lipgloss.NewStyle().
	Foreground(colourSubtleGrey).
	Italic(true)

// viewBorderOverhead is the number of rows (and columns) consumed by viewPaneStyle.
const viewBorderOverhead = 2

// ViewID is an enum for the five main views.
type ViewID int

const (
	ViewProject ViewID = iota
	ViewCommand
	ViewWorker
	ViewLog
	ViewDiff
)

// viewCount is the total number of views; used for tab cycling.
const viewCount = ViewDiff + 1

// viewModel is the interface that each view sub-model must satisfy.
type viewModel interface {
	tea.Model
	KeyBindings() []KeyBinding
	Label() string // e.g. "F1:Projects"; used by renderHeader
}

// clipLines truncates content to at most maxLines lines, preventing overflow
// when lipgloss line-wrapping produces more lines than the pane expects.
// lipgloss's Height() pads short content but does not clip tall content, so
// without this guard a wrapped line pushes the bottom border off-screen.
func clipLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[:maxLines], "\n")
}

// truncateLine truncates s to at most maxWidth visible runes, appending an
// ellipsis if truncation occurred.
func truncateLine(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth > 1 {
		return string(runes[:maxWidth-1]) + "…"
	}
	return string(runes[:maxWidth])
}

// nextView returns the next view in the cycle (project → command → worker → log → diffs → project).
func nextView(current ViewID) ViewID {
	return (current + 1) % viewCount
}

// prevView returns the previous view in the cycle.
func prevView(current ViewID) ViewID {
	return (current + viewCount - 1) % viewCount
}
